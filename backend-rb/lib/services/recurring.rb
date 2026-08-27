# frozen_string_literal: true

require "securerandom"

module Services
  # Recurring rules and the pass that turns them into transactions.
  class RecurringService
    RECURRING_ENTITY = "Recurring rule"
    TRANSFER_MESSAGE = "Recurring transfers are not supported."
    END_DATE_MESSAGE = "End date must not be before the start date."

    # A cap so one long-dormant rule cannot generate an unbounded batch in a
    # single pass; the rule stays active and the next pass carries on.
    MAX_OCCURRENCES_PER_PASS = 500
    RECURRING_TAG = "recurring"

    def initialize(rules, transactions, accounts, categories)
      @rules = rules
      @transactions = transactions
      @accounts = accounts
      @categories = categories
    end

    class << self
      # The next occurrence after `moment`. Clock time is preserved throughout.
      def advance(moment, frequency)
        return moment + (24 * 60 * 60) if frequency.is?(Domain::Enums::FREQUENCY_DAILY)
        return moment + (7 * 24 * 60 * 60) if frequency.is?(Domain::Enums::FREQUENCY_WEEKLY)
        return Domain::Dates.add_years(moment, 1) if frequency.is?(Domain::Enums::FREQUENCY_YEARLY)

        # Monthly, and anything undefined, advance by a clamped month.
        Domain::Dates.add_months(moment, 1)
      end

      # Emits the occurrences due at or before the cutoff, mutating the rule.
      def materialize(rule, cutoff)
        created = []

        MAX_OCCURRENCES_PER_PASS.times do
          break if !rule.is_active || rule.next_run_date > cutoff

          if !rule.end_date.nil? && rule.next_run_date > rule.end_date
            # An occurrence exactly *on* the end date is still created; only a
            # run past it retires the rule.
            rule.is_active = false
            break
          end

          created << Repositories::Transaction.new(
            id: SecureRandom.uuid, user_id: rule.user_id, account_id: rule.account_id,
            category_id: rule.category_id, type: rule.type, amount: rule.amount,
            date: rule.next_run_date, description: rule.description, notes: nil,
            tags: [ RECURRING_TAG ], transfer_account_id: nil
          )

          rule.next_run_date =
            Domain::Instant.from_time(advance(rule.next_run_date.time, rule.frequency))
        end

        created
      end
    end

    def list_all(user_id) = @rules.list_all(user_id).map { |rule| self.class.dto(rule) }

    def create(user_id, input)
      check(user_id, input)

      rule = Repositories::RecurringRule.new(
        id: SecureRandom.uuid, user_id: user_id, **shape(input),
        # The first occurrence is the start date itself.
        next_run_date: input[:start_date]
      )
      @rules.add(rule)
      self.class.dto(rule)
    end

    def update(user_id, rule_id, input)
      check(user_id, input)
      rule = load(user_id, rule_id)
      shape(input).each { |key, value| rule[key] = value }

      # Pull the next run forward if the start moved later; never push it back.
      rule.next_run_date = rule.start_date if rule.next_run_date < rule.start_date

      @rules.update(rule)
      self.class.dto(rule)
    end

    def remove(user_id, rule_id)
      raise E.not_found(RECURRING_ENTITY) unless @rules.remove(user_id, rule_id)
    end

    # Runs one pass. The caller owns the transaction and the lock.
    def materialize_due(now)
      due = @rules.list_due(now)

      created = due.flat_map { |rule| self.class.materialize(rule, now) }
      @transactions.add_many(created) unless created.empty?
      due.each { |rule| @rules.update(rule) }

      created.length
    end

    def self.dto(rule)
      { "id" => rule.id, "accountId" => rule.account_id, "categoryId" => rule.category_id,
        "type" => rule.type, "amount" => rule.amount, "description" => rule.description,
        "frequency" => rule.frequency, "startDate" => rule.start_date,
        "endDate" => rule.end_date, "nextRunDate" => rule.next_run_date,
        "isActive" => rule.is_active }
    end

    private

    # Order fixed: it decides which error a doubly-wrong request gets.
    def check(user_id, input)
      raise E.validation(TRANSFER_MESSAGE) if input[:type].is?(Domain::Enums::TRANSACTION_TRANSFER)
      raise E.not_found(ACCOUNT_ENTITY) unless @accounts.exists?(user_id, input[:account_id])
      if !input[:end_date].nil? && input[:end_date] < input[:start_date]
        raise E.validation(END_DATE_MESSAGE)
      end

      @categories.ensure_usable(user_id, input[:category_id])
    end

    def shape(input)
      { account_id: input[:account_id], category_id: input[:category_id], type: input[:type],
        amount: input[:amount].round_money, description: input[:description].to_s.strip,
        frequency: input[:frequency], start_date: input[:start_date], end_date: input[:end_date],
        # An omitted flag leaves the rule running.
        is_active: input[:is_active].nil? ? true : input[:is_active] }
    end

    def load(user_id, rule_id)
      @rules.get(user_id, rule_id) || raise(E.not_found(RECURRING_ENTITY))
    end
  end
end
