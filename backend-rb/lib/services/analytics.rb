# frozen_string_literal: true

module Services
  # Dashboard and report aggregations.
  #
  # Every window here is half-open -- [start, start + 1 month) -- so a
  # transaction timestamped midnight on the 1st lands in exactly one bucket.
  class AnalyticsService
    DEFAULT_NET_WORTH_MONTHS = 12
    DEFAULT_CASHFLOW_MONTHS = 6

    # The savings rate is a ratio, so it gets more places than money does.
    SAVINGS_RATE_SCALE = 4

    Group = Struct.new(:category_id, :total, :has_income, keyword_init: true)

    def initialize(transactions, accounts, categories)
      @transactions = transactions
      @accounts = accounts
      @categories = categories
    end

    def summary(user_id, now)
      slices = @transactions.slices(user_id, nil, nil)
      opening = opening_balance(user_id)

      month_start = Domain::Dates.start_of_month(now)
      month_end = Domain::Dates.add_months(month_start, 1)

      income = Domain::MONEY_ZERO
      expenses = Domain::MONEY_ZERO
      slices.each do |item|
        at = item.date.time
        next if at < month_start || at >= month_end

        if item.type.is?(Domain::Enums::TRANSACTION_INCOME)
          income = income.add(item.amount)
        elsif item.type.is?(Domain::Enums::TRANSACTION_EXPENSE)
          expenses = expenses.add(item.amount)
        end
      end

      # Net worth spans all time, not just this month.
      net_worth = slices.reduce(opening) { |sum, item| sum.add(Balance.net_worth_delta(item)) }

      rate =
        if income > Domain::MONEY_ZERO
          income.subtract(expenses).divide(income, SAVINGS_RATE_SCALE).trim
        else
          Domain::MONEY_ZERO
        end

      { "netWorth" => net_worth, "totalIncome" => income, "totalExpenses" => expenses,
        "savingsRate" => rate }
    end

    # A cumulative series: each point is the total as of that month's end.
    def net_worth(user_id, now, months)
      window = Domain::Dates.trailing_months(now, Domain::Dates.clamp_months(months))
      slices = @transactions.slices(user_id, nil, nil)
      opening = opening_balance(user_id)

      window.map do |start|
        finish = Domain::Dates.add_months(start, 1)
        value = slices.reduce(opening) do |sum, item|
          item.date.time < finish ? sum.add(Balance.net_worth_delta(item)) : sum
        end
        { "month" => Domain::Dates.month_key(start), "value" => value }
      end
    end

    def cashflow(user_id, now, months)
      window = Domain::Dates.trailing_months(now, Domain::Dates.clamp_months(months))
      slices = @transactions.slices(user_id, Domain::Instant.from_time(window.first), nil)

      window.map do |start|
        month, income, expenses = bucket(slices, start)
        { "month" => month, "income" => income, "expenses" => expenses }
      end
    end

    def spending(user_id, now, month)
      start = month.nil? ? Domain::Dates.start_of_month(now) : Domain::Dates.try_parse_month(month)
      raise E.validation(Domain::Dates::MONTH_FORMAT_MESSAGE) if start.nil?

      finish = Domain::Dates.add_months(start, 1)
      slices = @transactions.slices(user_id, Domain::Instant.from_time(start), nil)
      lookup = @categories.lookup(user_id)

      groups = group_by_category(slices) do |item|
        item.type.is?(Domain::Enums::TRANSACTION_EXPENSE) && item.date.time < finish
      end

      by_amount_descending(
        groups.map do |group|
          info = Services.describe(lookup, group.category_id)
          { "categoryId" => group.category_id, "categoryName" => info.name,
            "color" => info.color, "amount" => group.total }
        end
      )
    end

    def monthly_report(user_id, year)
      if year < Domain::Dates::MIN_REPORT_YEAR || year > Domain::Dates::MAX_REPORT_YEAR
        raise E.validation(Domain::Dates::YEAR_RANGE_MESSAGE)
      end

      start = Domain::Dates.first_day_utc(year, 1)
      # The SQL bound is inclusive, so stop just short of next January.
      finish = Domain::Dates.add_years(start, 1) - 0.000001

      slices = @transactions.slices(user_id, Domain::Instant.from_time(start),
                                    Domain::Instant.from_time(finish))

      (0...12).map do |offset|
        month, income, expenses = bucket(slices, Domain::Dates.add_months(start, offset))
        { "month" => month, "income" => income, "expenses" => expenses,
          "net" => income.subtract(expenses) }
      end
    end

    def category_report(user_id, date_from, date_to)
      slices = @transactions.slices(user_id, date_from, date_to)
      lookup = @categories.lookup(user_id)

      groups = group_by_category(slices) do |item|
        !item.type.is?(Domain::Enums::TRANSACTION_TRANSFER)
      end

      by_amount_descending(
        groups.map do |group|
          info = Services.describe(lookup, group.category_id)
          # An uncategorized bucket holding any income reads as income.
          type =
            if group.category_id.nil? && group.has_income
              Domain::Enums.of("CategoryType", Domain::Enums::CATEGORY_INCOME)
            else
              info.type
            end
          { "categoryId" => group.category_id, "categoryName" => info.name, "type" => type,
            "color" => info.color, "amount" => group.total }
        end
      )
    end

    private

    # Income and expense totals for the month beginning at `start`.
    def bucket(slices, start)
      finish = Domain::Dates.add_months(start, 1)
      income = Domain::MONEY_ZERO
      expenses = Domain::MONEY_ZERO

      slices.each do |item|
        at = item.date.time
        next if at < start || at >= finish

        if item.type.is?(Domain::Enums::TRANSACTION_INCOME)
          income = income.add(item.amount)
        elsif item.type.is?(Domain::Enums::TRANSACTION_EXPENSE)
          expenses = expenses.add(item.amount)
        end
      end

      [ Domain::Dates.month_key(start), income, expenses ]
    end

    # Groups by category, preserving first-seen key order like LINQ's GroupBy.
    def group_by_category(slices)
      groups = {}
      order = []

      slices.each do |item|
        next unless yield(item)

        key = item.category_id
        unless groups.key?(key)
          groups[key] = Group.new(category_id: key, total: Domain::MONEY_ZERO, has_income: false)
          order << key
        end
        group = groups[key]
        group.total = group.total.add(item.amount)
        group.has_income ||= item.type.is?(Domain::Enums::TRANSACTION_INCOME)
      end

      order.map { |key| groups[key] }
    end

    # Stable descending sort on the amount, so ties keep their original order.
    def by_amount_descending(rows)
      rows.each_with_index
          .sort { |(a, ai), (b, bi)| (b["amount"] <=> a["amount"]).nonzero? || (ai <=> bi) }
          .map(&:first)
    end

    def opening_balance(user_id)
      @accounts.list_all(user_id)
               .reduce(Domain::MONEY_ZERO) { |sum, account| sum.add(account.initial_balance) }
    end
  end
end
