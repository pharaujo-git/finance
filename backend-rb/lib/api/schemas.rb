# frozen_string_literal: true

module Api
  # Request bodies and the rules that guard them.
  #
  # A schema library is deliberately not used. The .NET pipeline fails
  # *deserialisation* before model validation runs, so a payload missing a
  # `required` member reports one error under "$" and nothing else -- a
  # short-circuit no declarative validator expresses. Everything is hand-rolled
  # to keep the five backends byte-identical on error responses.
  module Schemas
    E = Domain::Errors
    V = Domain::Validation
    M = Domain::Money

    UUID_PATTERN = /\A[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\z/i

    # A decoded JSON object, tracking which members the caller actually sent.
    class Body
      def initialize(raw)
        unless raw.is_a?(Hash)
          errs = E::FieldErrors.new
          errs.add(V::JSON_BODY_FIELD, "the JSON value could not be converted to an object")
          raise errs
        end
        @raw = raw
      end

      def text(key) = @raw[key].is_a?(String) ? @raw[key] : ""
      def optional_text(key) = @raw[key].is_a?(String) ? @raw[key] : nil

      # A member counts as supplied only when it is there and not null.
      def present?(key) = !@raw[key].nil?

      def money(key) = M.from_json(@raw[key])

      def uuid(key)
        value = @raw[key]
        value.is_a?(String) && UUID_PATTERN.match?(value) ? value : nil
      end

      def moment(key) = @raw[key].is_a?(String) ? Domain::Instant.parse_wire(@raw[key]) : nil
      def flag(key) = [ true, false ].include?(@raw[key]) ? @raw[key] : nil
      def tags(key) = @raw[key].is_a?(Array) ? @raw[key].select { |t| t.is_a?(String) } : []

      # Reads an enum member. A string naming no member is a *deserialisation*
      # failure reported under "$", like the other JSON-reader errors; a number
      # is not, because any ordinal is accepted.
      def enum(key, kind)
        raw = @raw[key]
        parsed = Domain::Enums.parse(kind, raw)
        return parsed unless parsed.nil? && !raw.nil?

        errs = E::FieldErrors.new
        errs.add(
          V::JSON_BODY_FIELD,
          raw.is_a?(String) ? %(the JSON value "#{raw}" could not be converted to #{kind})
                            : "the JSON value could not be converted to #{kind}"
        )
        raise errs
      end

      # The `required` members the caller left out, in declaration order.
      def missing(*keys) = keys.reject { |key| present?(key) }
    end

    module_function

    # Runs the required-member check, raising on the first failure.
    def guard(body, *members)
      errs = E::FieldErrors.new
      raise errs if V.required_members(errs, body.missing(*members))
    end

    def register(raw)
      body = Body.new(raw)
      request = { email: body.text("email"), password: body.text("password"),
                  name: body.text("name") }

      errs = E::FieldErrors.new
      V.required(errs, V::FIELD[:email], request[:email])
      V.email_address(errs, V::FIELD[:email], request[:email])
      V.max_length(errs, V::FIELD[:email], request[:email], V::LIMITS[:email])
      V.required(errs, V::FIELD[:password], request[:password])
      V.min_length(errs, V::FIELD[:password], request[:password], V::LIMITS[:password_min])
      V.max_length(errs, V::FIELD[:password], request[:password], V::LIMITS[:password_max])
      V.required(errs, V::FIELD[:name], request[:name])
      V.max_length(errs, V::FIELD[:name], request[:name], V::LIMITS[:name])
      errs.raise_if_any

      request
    end

    def login(raw)
      body = Body.new(raw)
      request = { email: body.text("email"), password: body.text("password") }

      # No email-shape rule here: sign-in must fail as bad credentials, not as a
      # validation error, so a wrong address cannot be told from a wrong one.
      errs = E::FieldErrors.new
      V.required(errs, V::FIELD[:email], request[:email])
      V.max_length(errs, V::FIELD[:email], request[:email], V::LIMITS[:email])
      V.required(errs, V::FIELD[:password], request[:password])
      V.max_length(errs, V::FIELD[:password], request[:password], V::LIMITS[:password_max])
      errs.raise_if_any

      request
    end

    def update_profile(raw)
      body = Body.new(raw)
      request = { name: body.text("name"), currency: body.text("currency") }

      errs = E::FieldErrors.new
      V.required(errs, V::FIELD[:name], request[:name])
      V.max_length(errs, V::FIELD[:name], request[:name], V::LIMITS[:name])
      V.required(errs, V::FIELD[:currency], request[:currency])
      V.max_length(errs, V::FIELD[:currency], request[:currency], V::LIMITS[:currency])
      errs.raise_if_any

      request
    end

    def account(raw)
      body = Body.new(raw)
      guard(body, "type")

      # An omitted currency and an explicit null both mean USD. The Go API
      # cannot tell them apart, so neither does this one.
      currency = body.optional_text("currency")
      request = { name: body.text("name"), type: body.enum("type", "AccountType"),
                  initial_balance: body.money("initialBalance"),
                  currency: currency.nil? ? "USD" : currency,
                  is_archived: body.flag("isArchived") }

      errs = E::FieldErrors.new
      V.required(errs, V::FIELD[:name], request[:name])
      V.max_length(errs, V::FIELD[:name], request[:name], V::LIMITS[:name])
      V.required(errs, V::FIELD[:currency], request[:currency])
      V.max_length(errs, V::FIELD[:currency], request[:currency], V::LIMITS[:currency])
      errs.raise_if_any

      request
    end

    def category(raw)
      body = Body.new(raw)
      guard(body, "type")

      request = { name: body.text("name"), type: body.enum("type", "CategoryType"),
                  icon: body.text("icon"), color: body.text("color") }

      errs = E::FieldErrors.new
      V.required(errs, V::FIELD[:name], request[:name])
      V.max_length(errs, V::FIELD[:name], request[:name], V::LIMITS[:name])
      V.max_length(errs, V::FIELD[:icon], request[:icon], V::LIMITS[:icon])
      V.max_length(errs, V::FIELD[:color], request[:color], V::LIMITS[:color])
      errs.raise_if_any

      request
    end

    def transaction(raw)
      body = Body.new(raw)
      guard(body, "accountId", "type", "amount", "date")

      request = { account_id: body.uuid("accountId"), category_id: body.uuid("categoryId"),
                  type: body.enum("type", "TransactionType"),
                  amount: body.money("amount") || M.zero, date: body.moment("date"),
                  description: body.text("description"), notes: body.optional_text("notes"),
                  tags: body.tags("tags"),
                  transfer_account_id: body.uuid("transferAccountId") }

      errs = E::FieldErrors.new
      V.money_range(errs, V::FIELD[:amount], request[:amount], M::MIN_POSITIVE)
      V.required(errs, V::FIELD[:description], request[:description])
      V.max_length(errs, V::FIELD[:description], request[:description], V::LIMITS[:description])
      unless request[:notes].nil?
        V.max_length(errs, V::FIELD[:notes], request[:notes], V::LIMITS[:notes])
      end
      errs.raise_if_any

      request
    end

    def create_budget(raw)
      body = Body.new(raw)
      guard(body, "categoryId", "limit")

      request = { category_id: body.uuid("categoryId"), month: body.text("month"),
                  limit: body.money("limit") || M.zero }

      errs = E::FieldErrors.new
      V.required(errs, V::FIELD[:month], request[:month])
      if !request[:month].empty? && !V::MONTH_PATTERN.match?(request[:month])
        errs.add(V::FIELD[:month], Domain::Dates::MONTH_FORMAT_MESSAGE)
      end
      V.money_range(errs, V::FIELD[:limit], request[:limit], M::MIN_ZERO)
      errs.raise_if_any

      request
    end

    def update_budget(raw)
      body = Body.new(raw)
      guard(body, "limit")

      request = { limit: body.money("limit") || M.zero }
      errs = E::FieldErrors.new
      V.money_range(errs, V::FIELD[:limit], request[:limit], M::MIN_ZERO)
      errs.raise_if_any

      request
    end

    def goal(raw)
      body = Body.new(raw)
      guard(body, "targetAmount")

      request = { name: body.text("name"), target_amount: body.money("targetAmount") || M.zero,
                  current_amount: body.money("currentAmount"),
                  target_date: body.moment("targetDate"), color: body.text("color") }

      errs = E::FieldErrors.new
      V.required(errs, V::FIELD[:name], request[:name])
      V.max_length(errs, V::FIELD[:name], request[:name], V::LIMITS[:name])
      V.money_range(errs, V::FIELD[:target_amount], request[:target_amount], M::MIN_POSITIVE)
      V.money_range(errs, V::FIELD[:current_amount], request[:current_amount], M::MIN_ZERO)
      V.max_length(errs, V::FIELD[:color], request[:color], V::LIMITS[:color])
      errs.raise_if_any

      request
    end

    def contribute(raw)
      body = Body.new(raw)
      guard(body, "amount")

      request = { amount: body.money("amount") || M.zero }
      errs = E::FieldErrors.new
      V.money_range(errs, V::FIELD[:amount], request[:amount], M::MIN_POSITIVE)
      errs.raise_if_any

      request
    end

    def recurring(raw)
      body = Body.new(raw)
      guard(body, "accountId", "type", "amount", "frequency", "startDate")

      request = { account_id: body.uuid("accountId"), category_id: body.uuid("categoryId"),
                  type: body.enum("type", "TransactionType"),
                  amount: body.money("amount") || M.zero, description: body.text("description"),
                  frequency: body.enum("frequency", "Frequency"),
                  start_date: body.moment("startDate"), end_date: body.moment("endDate"),
                  is_active: body.flag("isActive") }

      errs = E::FieldErrors.new
      V.money_range(errs, V::FIELD[:amount], request[:amount], M::MIN_POSITIVE)
      V.required(errs, V::FIELD[:description], request[:description])
      V.max_length(errs, V::FIELD[:description], request[:description], V::LIMITS[:description])
      errs.raise_if_any

      request
    end
  end

  # Query-string binding that mirrors MVC's model binder. Conversion failures
  # are collected rather than raised one at a time, so a request with three bad
  # values reports all three. An *absent* key is not the same as an empty one:
  # "?month=" is a failure, not "this month".
  class QueryReader
    def initialize(params)
      @params = params
      @errs = Domain::Errors::FieldErrors.new
    end

    # nil when the caller omitted the key entirely.
    def text(key) = @params.key?(key) ? @params[key].to_s : nil

    def number(key, field)
      raw = text(key)
      return nil if raw.nil?

      # Deliberately strict: MVC's binder rejects "12abc" rather than reading 12.
      unless /\A[+-]?\d+\z/.match?(raw.strip)
        @errs.add(field, Domain::Validation.invalid_value_message(raw, field))
        return nil
      end
      raw.to_i
    end

    def number_or(key, field, fallback)
      parsed = number(key, field)
      parsed.nil? ? fallback : parsed
    end

    def identifier(key, field)
      raw = text(key)
      return nil if raw.nil?
      return raw if Schemas::UUID_PATTERN.match?(raw)

      @errs.add(field, Domain::Validation.invalid_value_message(raw, field))
      nil
    end

    def moment(key, field)
      raw = text(key)
      return nil if raw.nil?

      parsed = Domain::Instant.parse_wire(raw)
      @errs.add(field, Domain::Validation.invalid_value_message(raw, field)) if parsed.nil?
      parsed
    end

    def enum(key, field, kind)
      raw = text(key)
      return nil if raw.nil?

      parsed = Domain::Enums.parse(kind, raw)
      @errs.add(field, Domain::Validation.invalid_value_message(raw, field)) if parsed.nil?
      parsed
    end

    # Raises the accumulated conversion failures, if any.
    def done = @errs.raise_if_any
  end
end
