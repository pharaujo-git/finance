# frozen_string_literal: true

module Domain
  # The four enums, twins of FinanceTracker.Domain.Enums.
  #
  # Two wire formats have to agree at once: the database stores the ordinal in
  # an integer column, and JSON carries the camelCase member name. An ordinal
  # outside the declared set is preserved rather than rejected -- the .NET
  # converter accepts any number on read and writes an undefined value straight
  # back out as a number, and rows already holding one must round-trip.
  module Enums
    # Wire names in ordinal order. "creditCard" is the one that is not a plain
    # lowercase of the C# name.
    WIRE_NAMES = {
      "AccountType" => %w[checking savings creditCard cash investment].freeze,
      "CategoryType" => %w[income expense].freeze,
      "TransactionType" => %w[income expense transfer].freeze,
      "Frequency" => %w[daily weekly monthly yearly].freeze
    }.freeze

    ACCOUNT_CHECKING = 0
    ACCOUNT_SAVINGS = 1
    ACCOUNT_CREDIT_CARD = 2

    CATEGORY_INCOME = 0
    CATEGORY_EXPENSE = 1

    TRANSACTION_INCOME = 0
    TRANSACTION_EXPENSE = 1
    TRANSACTION_TRANSFER = 2

    FREQUENCY_DAILY = 0
    FREQUENCY_WEEKLY = 1
    FREQUENCY_MONTHLY = 2
    FREQUENCY_YEARLY = 3

    # An ordinal tagged with the enum it belongs to, so it can render itself.
    class Value
      attr_reader :kind, :ordinal

      def initialize(kind, ordinal)
        @kind = kind
        @ordinal = ordinal
        freeze
      end

      def defined? = ordinal >= 0 && ordinal < WIRE_NAMES.fetch(kind).length

      # The camelCase name, or the bare ordinal for an undefined value.
      def wire_name = WIRE_NAMES.fetch(kind)[ordinal] || ordinal.to_s

      def is?(other) = ordinal == other
      def to_s = wire_name
      def ==(other) = other.is_a?(Value) && kind == other.kind && ordinal == other.ordinal
      alias eql? ==
      def hash = [ kind, ordinal ].hash
    end

    module_function

    def of(kind, ordinal) = Value.new(kind, ordinal)

    # Reads a member name in any casing, or an ordinal as digits. Any integer is
    # accepted, defined or not; anything else is nil.
    def parse(kind, value)
      case value
      when nil, true, false then nil
      when ::Integer then Value.new(kind, value)
      when ::String
        text = value.strip
        return nil if text.empty?

        index = WIRE_NAMES.fetch(kind).index { |name| name.casecmp?(text) }
        return Value.new(kind, index) if index

        /\A-?\d+\z/.match?(text) ? Value.new(kind, text.to_i) : nil
      else nil # anything else on the wire is simply not an enum member
      end
    end
  end
end
