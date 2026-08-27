# frozen_string_literal: true

require "securerandom"

module Services
  # Business logic, testable without HTTP. Services take domain arguments and
  # return plain hashes shaped for the wire; controllers only bind and render.
  E = Domain::Errors
  V = Domain::Validation

  # Strings the frontend surfaces verbatim, byte-identical to the others'.
  DEFAULT_CURRENCY = "USD"
  DUPLICATE_EMAIL_MESSAGE = "An account with that email already exists."
  INVALID_CREDENTIALS_MESSAGE = "Invalid email or password."
  DEFAULT_CATEGORY_MESSAGE = "Default categories cannot be modified."
  TRANSFER_TARGET_MESSAGE = "A transfer requires a destination account."
  TRANSFER_SAME_ACCOUNT_MESSAGE = "A transfer must use two different accounts."
  DUPLICATE_BUDGET_MESSAGE = "A budget already exists for that category and month."
  CONTRIBUTION_MESSAGE = "Contribution amount must be greater than zero."

  ACCOUNT_ENTITY = "Account"
  CATEGORY_ENTITY = "Category"
  TRANSACTION_ENTITY = "Transaction"
  BUDGET_ENTITY = "Budget"
  GOAL_ENTITY = "Goal"
  USER_ENTITY = "User"

  module_function

  # Trim, then lowercase -- so "  Owner@Example.COM " is one account.
  def normalize_email(email) = email.to_s.strip.downcase
  def normalize_currency(currency) = currency.to_s.strip.upcase

  class AuthService
    def initialize(users, tokens)
      @users = users
      @tokens = tokens
    end

    # Creates an account and signs the caller straight in.
    def register(email, password, name)
      normalized = Services.normalize_email(email)
      raise E.conflict(DUPLICATE_EMAIL_MESSAGE) if @users.find_by_email(normalized)

      user = Repositories::User.new(
        id: SecureRandom.uuid, email: normalized, name: name.to_s.strip,
        password_hash: Core::Security.hash_password(password),
        currency: DEFAULT_CURRENCY, created_at: Domain::Instant.now
      )

      begin
        @users.add(user)
      rescue Repositories::EmailTaken
        # The lookup above is racy on its own; the unique index decides, and the
        # loser reports the same conflict as the early check.
        raise E.conflict(DUPLICATE_EMAIL_MESSAGE)
      end

      auth_response(user)
    end

    # Verifies credentials and issues a token. An unknown address and a wrong
    # password fail identically, so the response cannot enumerate accounts.
    def login(email, password)
      user = @users.find_by_email(Services.normalize_email(email))
      raise E.unauthorized(INVALID_CREDENTIALS_MESSAGE) if user.nil?

      case Core::Security.verify_password(user.password_hash, password)
      when :failed
        raise E.unauthorized(INVALID_CREDENTIALS_MESSAGE)
      when :success_rehash_needed
        # The stored blob predates the current parameters; upgrade it now that
        # the plaintext is in hand.
        upgraded = Core::Security.hash_password(password)
        @users.update_password_hash(user.id, upgraded)
        user.password_hash = upgraded
      else
        # :success -- the stored blob already uses the current parameters.
        nil
      end

      auth_response(user)
    end

    def profile(user_id) = user_dto(load(user_id))

    def update_profile(user_id, name, currency)
      user = load(user_id)
      user.name = name.to_s.strip
      user.currency = Services.normalize_currency(currency)

      raise E.not_found(USER_ENTITY) unless @users.update_profile(user.id, user.name, user.currency)

      user_dto(user)
    end

    private

    # A token whose subject no longer exists gets a 404, not a 401.
    def load(user_id)
      @users.find_by_id(user_id) || raise(E.not_found(USER_ENTITY))
    end

    def user_dto(user)
      { "id" => user.id, "email" => user.email, "name" => user.name, "currency" => user.currency }
    end

    def auth_response(user)
      { "token" => @tokens.issue(user.id, user.email), "user" => user_dto(user) }
    end
  end

  class AccountService
    def initialize(accounts, transactions)
      @accounts = accounts
      @transactions = transactions
    end

    def list_all(user_id)
      accounts = @accounts.list_all(user_id)
      slices = @transactions.slices(user_id, nil, nil)
      accounts.map { |account| dto(account, Balance.balance_of(account, slices)) }
    end

    def get(user_id, account_id)
      account = load(user_id, account_id)
      slices = @transactions.slices(user_id, nil, nil)
      dto(account, Balance.balance_of(account, slices))
    end

    def create(user_id, input)
      account = Repositories::Account.new(
        id: SecureRandom.uuid, user_id: user_id, name: input[:name].to_s.strip,
        type: input[:type],
        # Stored verbatim, not rounded: the other backends echo back whatever
        # scale the caller sent, and the column rounds on write.
        initial_balance: input[:initial_balance] || Domain::MONEY_ZERO,
        currency: Services.normalize_currency(input[:currency]),
        is_archived: false, created_at: Domain::Instant.now
      )
      @accounts.add(account)
      # A brand-new account has no transactions, so the opening balance is it.
      dto(account, account.initial_balance)
    end

    def update(user_id, account_id, input)
      account = load(user_id, account_id)
      account.name = input[:name].to_s.strip
      account.type = input[:type]
      account.currency = Services.normalize_currency(input[:currency])
      # An omitted flag un-archives, matching `request.IsArchived ?? false`.
      account.is_archived = input[:is_archived] == true

      @accounts.update(account)
      slices = @transactions.slices(user_id, nil, nil)
      dto(account, Balance.balance_of(account, slices))
    end

    # The DELETE handler's work: flag the row so history stays intact.
    def archive(user_id, account_id)
      raise E.not_found(ACCOUNT_ENTITY) unless @accounts.archive(user_id, account_id)
    end

    private

    def load(user_id, account_id)
      @accounts.get(user_id, account_id) || raise(E.not_found(ACCOUNT_ENTITY))
    end

    def dto(account, balance)
      { "id" => account.id, "name" => account.name, "type" => account.type,
        "balance" => balance, "currency" => account.currency,
        "isArchived" => account.is_archived, "createdAt" => account.created_at }
    end
  end

  CategoryInfo = Struct.new(:name, :color, :type, keyword_init: true)

  class CategoryService
    def initialize(categories)
      @categories = categories
    end

    def list_all(user_id) = @categories.list_visible(user_id).map { |item| dto(item) }

    # Id -> label. A blank stored colour falls back to the grey too.
    def lookup(user_id)
      @categories.list_visible(user_id).to_h do |item|
        colour = item.color.to_s.empty? ? Repositories::UNCATEGORIZED_COLOR : item.color
        [ item.id, CategoryInfo.new(name: item.name, color: colour, type: item.type) ]
      end
    end

    # A nil category is fine; one the caller cannot see is a 404.
    def ensure_usable(user_id, category_id)
      return if category_id.nil?
      raise E.not_found(CATEGORY_ENTITY) if @categories.get(user_id, category_id).nil?
    end

    def create(user_id, input)
      category = Repositories::Category.new(
        id: SecureRandom.uuid, user_id: user_id, name: input[:name].to_s.strip,
        type: input[:type], icon: input[:icon].to_s.strip, color: input[:color].to_s.strip,
        is_default: false
      )
      @categories.add(category)
      dto(category)
    end

    def update(user_id, category_id, input)
      category = load_owned(user_id, category_id)
      category.name = input[:name].to_s.strip
      category.type = input[:type]
      category.icon = input[:icon].to_s.strip
      category.color = input[:color].to_s.strip

      @categories.update(category)
      dto(category)
    end

    def remove(user_id, category_id)
      load_owned(user_id, category_id)
      @categories.remove(user_id, category_id)
    end

    private

    # A shared default is visible to everyone but editable by no one.
    def load_owned(user_id, category_id)
      visible = @categories.get(user_id, category_id)
      raise E.not_found(CATEGORY_ENTITY) if visible.nil?
      raise E.validation(DEFAULT_CATEGORY_MESSAGE) if visible.is_default

      @categories.get_owned(user_id, category_id) || raise(E.not_found(CATEGORY_ENTITY))
    end

    def dto(category)
      { "id" => category.id, "name" => category.name, "type" => category.type,
        "icon" => category.icon, "color" => category.color, "isDefault" => category.is_default }
    end
  end

  module_function

  # Labels a slice, falling back to the shared "Uncategorized" grey.
  def describe(lookup, category_id)
    found = category_id.nil? ? nil : lookup[category_id]
    return found if found

    CategoryInfo.new(name: Repositories::UNCATEGORIZED_NAME,
                     color: Repositories::UNCATEGORIZED_COLOR,
                     type: Domain::Enums.of("CategoryType", Domain::Enums::CATEGORY_EXPENSE))
  end
end

module Services
  class TransactionService
    DEFAULT_PAGE = 1
    DEFAULT_PAGE_SIZE = 20
    MAX_PAGE_SIZE = 200
    MAX_INT32 = 2_147_483_647

    def initialize(transactions, accounts, categories)
      @transactions = transactions
      @accounts = accounts
      @categories = categories
    end

    def search(user_id, query)
      validate_query(query)

      items, total = @transactions.search(
        user_id,
        Repositories::TransactionRepository::Filter.new(
          account_id: query[:account_id], category_id: query[:category_id], type: query[:type],
          date_from: query[:date_from], date_to: query[:date_to],
          search: query[:search].to_s.strip.downcase,
          limit: query[:page_size], offset: (query[:page] - 1) * query[:page_size]
        )
      )

      { "items" => items.map { |item| Services.transaction_dto(item) }, "total" => total,
        "page" => query[:page], "pageSize" => query[:page_size] }
    end

    def get(user_id, transaction_id) = Services.transaction_dto(load(user_id, transaction_id))

    def create(user_id, input)
      item = build(user_id, SecureRandom.uuid, input)
      @transactions.add(item)
      Services.transaction_dto(item)
    end

    def update(user_id, transaction_id, input)
      load(user_id, transaction_id)
      item = build(user_id, transaction_id, input)
      @transactions.update(item)
      Services.transaction_dto(item)
    end

    def remove(user_id, transaction_id)
      raise E.not_found(TRANSACTION_ENTITY) unless @transactions.remove(user_id, transaction_id)
    end

    private

    def validate_query(query)
      errs = E::FieldErrors.new
      V.int_range(errs, V::FIELD[:page], query[:page], 1, MAX_INT32)
      V.int_range(errs, V::FIELD[:page_size], query[:page_size], 1, MAX_PAGE_SIZE)
      V.max_length(errs, V::FIELD[:search], query[:search], V::LIMITS[:search])
      errs.raise_if_any
    end

    # Validates the references, then shapes the row. The order of the checks
    # decides which error a doubly-wrong request gets, so it is fixed: account,
    # then category, then the transfer rules.
    def build(user_id, id, input)
      ensure_account(user_id, input[:account_id])
      @categories.ensure_usable(user_id, input[:category_id])

      transfer_account_id = nil
      if input[:type].is?(Domain::Enums::TRANSACTION_TRANSFER)
        raise E.validation(TRANSFER_TARGET_MESSAGE) if input[:transfer_account_id].nil?
        if input[:transfer_account_id] == input[:account_id]
          raise E.validation(TRANSFER_SAME_ACCOUNT_MESSAGE)
        end

        ensure_account(user_id, input[:transfer_account_id])
        transfer_account_id = input[:transfer_account_id]
      end

      Repositories::Transaction.new(
        id: id, user_id: user_id, account_id: input[:account_id],
        category_id: input[:category_id], type: input[:type],
        amount: input[:amount].round_money, date: input[:date],
        description: input[:description].to_s.strip,
        notes: Domain::Tags.trimmed_or_nil(input[:notes]),
        # Normalised here, not just on the way to the column: the response has
        # to show the tags as stored, trimmed and without the blanks.
        tags: Domain::Tags.split(Domain::Tags.join(input[:tags])),
        transfer_account_id: transfer_account_id
      )
    end

    def ensure_account(user_id, account_id)
      raise E.not_found(ACCOUNT_ENTITY) unless @accounts.exists?(user_id, account_id)
    end

    def load(user_id, transaction_id)
      @transactions.get(user_id, transaction_id) || raise(E.not_found(TRANSACTION_ENTITY))
    end
  end

  module_function

  def transaction_dto(item)
    { "id" => item.id, "accountId" => item.account_id, "categoryId" => item.category_id,
      "type" => item.type, "amount" => item.amount, "date" => item.date,
      "description" => item.description, "notes" => item.notes, "tags" => item.tags,
      "transferAccountId" => item.transfer_account_id }
  end

  # A sort key matching .NET's Guid.CompareTo, which is not byte order: the
  # first three groups compare as signed integers, so a uuid starting 0x80 sorts
  # before one starting 0x7f. Reproduced because it decides budget ordering.
  def compare_guid(left, right)
    a = [ left.delete("-") ].pack("H*").bytes
    b = [ right.delete("-") ].pack("H*").bytes

    [ [ 0, 4 ], [ 4, 6 ], [ 6, 8 ] ].each do |from, to|
      x = signed(a[from...to])
      y = signed(b[from...to])
      return x <=> y unless x == y
    end
    (8...16).each do |index|
      return a[index] <=> b[index] unless a[index] == b[index]
    end
    0
  end

  def signed(bytes)
    value = bytes.reduce(0) { |acc, byte| (acc << 8) | byte }
    bits = bytes.length * 8
    value >= (1 << (bits - 1)) ? value - (1 << bits) : value
  end

  class BudgetService
    def initialize(budgets, transactions, categories)
      @budgets = budgets
      @transactions = transactions
      @categories = categories
    end

    def list_all(user_id, month)
      key = month || Domain::Dates.month_key(Time.now.utc)
      # An explicitly supplied month that is not a real one is a 400; an
      # *absent* key means "this month", but "?month=" does not.
      if !month.nil? && Domain::Dates.try_parse_month(month).nil?
        raise E.validation(Domain::Dates::MONTH_FORMAT_MESSAGE)
      end

      start = Domain::Dates.try_parse_month(key)
      raise E.validation(Domain::Dates::MONTH_FORMAT_MESSAGE) if start.nil?

      budgets = @budgets.list_for_month(user_id, key)
      # Nothing to measure, so skip the spend query entirely.
      return [] if budgets.empty?

      spent = spent_by_category(user_id, start)
      budgets
        .sort { |a, b| Services.compare_guid(a.category_id, b.category_id) }
        .map { |budget| dto(budget, spent.fetch(budget.category_id, Domain::MONEY_ZERO)) }
    end

    def create(user_id, category_id, month, limit)
      start = Domain::Dates.try_parse_month(month)
      raise E.validation(Domain::Dates::MONTH_FORMAT_MESSAGE) if start.nil?

      @categories.ensure_usable(user_id, category_id)
      raise E.conflict(DUPLICATE_BUDGET_MESSAGE) if @budgets.exists?(user_id, category_id, month)

      budget = Repositories::Budget.new(
        id: SecureRandom.uuid, user_id: user_id, category_id: category_id,
        month: month, limit: limit.round_money
      )
      @budgets.add(budget)

      spent = spent_by_category(user_id, start)
      dto(budget, spent.fetch(category_id, Domain::MONEY_ZERO))
    end

    # Only the limit moves; the category and month are immutable.
    def update(user_id, budget_id, limit)
      budget = load(user_id, budget_id)
      budget.limit = limit.round_money
      @budgets.update_limit(user_id, budget_id, budget.limit)

      start = Domain::Dates.try_parse_month(budget.month)
      spent = start.nil? ? {} : spent_by_category(user_id, start)
      dto(budget, spent.fetch(budget.category_id, Domain::MONEY_ZERO))
    end

    def remove(user_id, budget_id)
      raise E.not_found(BUDGET_ENTITY) unless @budgets.remove(user_id, budget_id)
    end

    private

    def load(user_id, budget_id)
      @budgets.get(user_id, budget_id) || raise(E.not_found(BUDGET_ENTITY))
    end

    # Expenses in [start, start + 1 month). Income and transfers never count.
    def spent_by_category(user_id, start)
      finish = Domain::Dates.add_months(start, 1)
      slices = @transactions.slices(user_id, Domain::Instant.from_time(start), nil)

      slices.each_with_object({}) do |item, totals|
        next unless item.type.is?(Domain::Enums::TRANSACTION_EXPENSE)
        # Uncategorized spending is not measured against any budget.
        next if item.category_id.nil?
        next if item.date.time >= finish

        totals[item.category_id] = totals.fetch(item.category_id, Domain::MONEY_ZERO).add(item.amount)
      end
    end

    def dto(budget, spent)
      { "id" => budget.id, "categoryId" => budget.category_id, "month" => budget.month,
        "limit" => budget.limit, "spent" => spent,
        # Allowed to go negative: an overspent budget should show how far over.
        "remaining" => budget.limit.subtract(spent) }
    end
  end

  class GoalService
    def initialize(goals)
      @goals = goals
    end

    def list_all(user_id) = @goals.list_all(user_id).map { |goal| dto(goal) }

    def create(user_id, input)
      goal = Repositories::Goal.new(
        id: SecureRandom.uuid, user_id: user_id, name: input[:name].to_s.strip,
        target_amount: input[:target_amount].round_money,
        current_amount: (input[:current_amount] || Domain::MONEY_ZERO).round_money,
        target_date: input[:target_date], color: input[:color].to_s.strip
      )
      @goals.add(goal)
      dto(goal)
    end

    def update(user_id, goal_id, input)
      goal = load(user_id, goal_id)
      goal.name = input[:name].to_s.strip
      goal.target_amount = input[:target_amount].round_money
      # An omitted currentAmount resets the goal rather than leaving it be,
      # matching the other backends.
      goal.current_amount = (input[:current_amount] || Domain::MONEY_ZERO).round_money
      goal.target_date = input[:target_date]
      goal.color = input[:color].to_s.strip

      @goals.update(goal)
      dto(goal)
    end

    def remove(user_id, goal_id)
      raise E.not_found(GOAL_ENTITY) unless @goals.remove(user_id, goal_id)
    end

    def contribute(user_id, goal_id, amount)
      raise E.validation(CONTRIBUTION_MESSAGE) unless amount > Domain::MONEY_ZERO

      goal = load(user_id, goal_id)
      # Not clamped at the target: a goal is allowed to be exceeded.
      goal.current_amount = goal.current_amount.add(amount).round_money
      @goals.update(goal)
      dto(goal)
    end

    private

    def load(user_id, goal_id)
      @goals.get(user_id, goal_id) || raise(E.not_found(GOAL_ENTITY))
    end

    def dto(goal)
      { "id" => goal.id, "name" => goal.name, "targetAmount" => goal.target_amount,
        "currentAmount" => goal.current_amount, "targetDate" => goal.target_date,
        "color" => goal.color }
    end
  end
end
