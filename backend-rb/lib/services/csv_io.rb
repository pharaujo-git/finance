# frozen_string_literal: true

require "securerandom"

module Services
  # CSV export and import.
  #
  # The reader is hand-written rather than taken from the stdlib: it has to
  # match the other backends byte for byte, including treating a bad row as
  # *skipped* rather than fatal, and ignoring bare carriage returns outside
  # quotes.
  class CsvService
    HEADER = %w[Date Type Amount Account Category Description Notes Tags].freeze
    # The column separator for tags, distinct from the storage unit separator.
    TAG_DELIMITER = ";"

    EMPTY_MESSAGE = "The uploaded file contains no rows."
    MISSING_FILE_MESSAGE = "A non-empty CSV file is required."
    OVERSIZE_MESSAGE = "The uploaded file is larger than 5 MB."
    MAX_UPLOAD_BYTES = 5 * 1024 * 1024

    CURRENCY_SYMBOLS = [ "¤", "$", "£", "€" ].freeze

    def initialize(transactions, accounts, categories)
      @transactions = transactions
      @accounts = accounts
      @categories = categories
    end

    class << self
      def export_file_name(now) = "transactions-#{now.utc.strftime('%Y-%m-%d')}.csv"

      # RFC 4180: quote only when needed, and double any internal quote.
      def escape_field(value)
        text = value.to_s
        return text unless text.match?(/[",\n\r]/)

        %("#{text.gsub('"', '""')}")
      end

      def row(fields)
        # A bare \n, never \r\n -- that is what the other backends write.
        "#{fields.map { |field| escape_field(field) }.join(',')}\n"
      end

      # A minimal RFC 4180 reader. Outside quotes a carriage return is dropped
      # entirely, so CRLF files parse the same as LF ones, and a trailing
      # newline produces no empty final record.
      def parse(text)
        rows = []
        fields = []
        buffer = +""
        quoted = false
        touched = false

        commit = lambda do
          next if !touched && buffer.empty? && fields.empty?

          fields << buffer.dup
          rows << fields.dup
          fields.clear
          buffer.clear
          touched = false
        end

        # Inside quotes: consume one character and report how many were used,
        # since an escaped quote ("") is two.
        inside_quotes = lambda do |char, next_char|
          if char != '"'
            # Quoted fields may span line breaks.
            buffer << char
          elsif next_char == '"'
            buffer << '"'
            next 2
          else
            quoted = false
          end
          1
        end

        outside_quotes = lambda do |char|
          case char
          when '"'
            quoted = true
            touched = true
          when ","
            fields << buffer.dup
            buffer.clear
            touched = true
          when "\r" then nil # ignored outside quotes
          when "\n" then commit.call
          else
            buffer << char
            touched = true
          end
        end

        index = 0
        while index < text.length
          char = text[index]
          if quoted
            index += inside_quotes.call(char, text[index + 1])
          else
            outside_quotes.call(char)
            index += 1
          end
        end

        commit.call
        rows
      end

      # Most specific first; a value with no zone is read as UTC.
      def parse_date(value)
        text = value.to_s.strip
        return nil if text.empty?

        iso = Domain::Instant.parse_wire(text.sub(" ", "T"))
        return iso if iso

        match = %r{\A(\d{1,2})/(\d{1,2})/(\d{4})(?:[ T](\d{1,2}):(\d{2}):(\d{2})(?:\s*(AM|PM))?)?\z}i
                .match(text)
        return nil if match.nil?

        month = match[1].to_i
        day = match[2].to_i
        year = match[3].to_i
        # Time.utc rolls an out-of-range field over rather than failing, so
        # 26/08/2026 would silently become February 2028; the other backends
        # reject it because no layout of theirs matches a 26th month.
        return nil if month < 1 || month > 12
        return nil if day < 1 || day > ::Date.new(year, month, -1).day

        hour = match[4].to_i
        meridiem = match[7]&.upcase
        hour += 12 if meridiem == "PM" && hour < 12
        hour = 0 if meridiem == "AM" && hour == 12
        return nil if hour > 23 || match[5].to_i > 59 || match[6].to_i > 59

        Domain::Instant.from_time(Time.utc(year, month, day, hour, match[5].to_i, match[6].to_i))
      end

      # Reads the shapes .NET's NumberStyles.Currency accepts.
      def parse_currency(value)
        text = value.to_s.strip
        negative = false

        if text.start_with?("(") && text.end_with?(")")
          negative = true
          text = text[1..-2].to_s.strip
        end

        text = text[1..].to_s while CURRENCY_SYMBOLS.include?(text[0])
        text = text.delete(",").strip

        if text.end_with?("-")
          negative = true
          text = text[0..-2].to_s
        end

        amount = Domain::Money.parse(text.strip)
        return nil if amount.nil?

        negative ? amount.negate : amount
      end
    end

    def export(user_id, date_from, date_to)
      items = @transactions.list_range(user_id, date_from, date_to)
      account_names = @accounts.list_all(user_id).to_h { |account| [ account.id, account.name ] }
      category_names = @categories.list_visible(user_id).to_h { |item| [ item.id, item.name ] }

      out = +self.class.row(HEADER)
      items.each do |item|
        out << self.class.row([
          item.date.time.utc.strftime("%Y-%m-%d"),
          item.type.wire_name,
          # Always exactly two places here, unlike the JSON shape.
          item.amount.to_fixed2,
          account_names.fetch(item.account_id, ""),
          item.category_id ? category_names.fetch(item.category_id, "") : "",
          item.description,
          item.notes.to_s,
          item.tags.join(TAG_DELIMITER)
        ])
      end
      out
    end

    def import(user_id, content)
      # Strip a leading UTF-8 BOM, which spreadsheets like to add.
      text = content.to_s.dup.force_encoding("UTF-8").sub(/\A﻿/, "")
      rows = self.class.parse(text)
      raise E.validation(EMPTY_MESSAGE) if rows.empty?

      # Accounts: the last name wins. Categories: the first does. That asymmetry
      # is inherited from the .NET lookups and kept deliberately.
      account_ids = {}
      @accounts.list_all(user_id).each { |account| account_ids[account.name.upcase] = account.id }
      category_ids = {}
      @categories.list_visible(user_id).each do |category|
        category_ids[category.name.upcase] ||= category.id
      end

      rows = rows[1..] || [] if header_row?(rows.first)

      imported = []
      skipped = 0
      rows.each do |row|
        built = build_row(user_id, row, account_ids, category_ids)
        built.nil? ? skipped += 1 : imported << built
      end

      @transactions.add_many(imported) unless imported.empty?
      { "imported" => imported.length, "skipped" => skipped }
    end

    private

    def header_row?(row) = !row.nil? && !row.empty? && row[0].to_s.strip.casecmp?("date")

    def field(row, index) = row[index].to_s.strip

    # Returns nil for any unusable row; the caller counts it as skipped.
    def build_row(user_id, row, account_ids, category_ids)
      return nil if row.length < 6

      date = self.class.parse_date(field(row, 0))
      return nil if date.nil?

      type = Domain::Enums.parse("TransactionType", field(row, 1))
      # Transfers carry no destination in this CSV shape, so they never import.
      return nil if type.nil? || type.is?(Domain::Enums::TRANSACTION_TRANSFER)

      amount = self.class.parse_currency(field(row, 2))
      return nil if amount.nil? || amount <= Domain::MONEY_ZERO

      account_id = account_ids[field(row, 3).upcase]
      return nil if account_id.nil?

      # A missing category is fine; the transaction is simply uncategorized.
      raw_tags = field(row, 7)
      Repositories::Transaction.new(
        id: SecureRandom.uuid, user_id: user_id, account_id: account_id,
        category_id: category_ids[field(row, 4).upcase], type: type,
        amount: amount.round_money, date: date, description: field(row, 5),
        notes: Domain::Tags.trimmed_or_nil(field(row, 6)),
        tags: raw_tags.empty? ? [] : raw_tags.split(TAG_DELIMITER),
        transfer_account_id: nil
      )
    end
  end
end
