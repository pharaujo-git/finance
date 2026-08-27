# frozen_string_literal: true

module Repositories
  # All SQL lives here, one class per aggregate. Queries and persistence only:
  # no business rules, and nothing that knows about HTTP.
  UNIQUE_VIOLATION = "23505"

  class EmailTaken < StandardError; end

  User = Struct.new(:id, :email, :name, :password_hash, :currency, :created_at, keyword_init: true)
  Account = Struct.new(:id, :user_id, :name, :type, :initial_balance, :currency,
                       :is_archived, :created_at, keyword_init: true)
  Category = Struct.new(:id, :user_id, :name, :type, :icon, :color, :is_default, keyword_init: true)
  Transaction = Struct.new(:id, :user_id, :account_id, :category_id, :type, :amount, :date,
                           :description, :notes, :tags, :transfer_account_id, keyword_init: true)
  Slice = Struct.new(:account_id, :transfer_account_id, :category_id, :type, :amount, :date,
                     keyword_init: true)
  Budget = Struct.new(:id, :user_id, :category_id, :month, :limit, keyword_init: true)
  Goal = Struct.new(:id, :user_id, :name, :target_amount, :current_amount, :target_date, :color,
                    keyword_init: true)
  RecurringRule = Struct.new(:id, :user_id, :account_id, :category_id, :type, :amount,
                             :description, :frequency, :start_date, :end_date, :next_run_date,
                             :is_active, keyword_init: true)

  UNCATEGORIZED_NAME = "Uncategorized"
  UNCATEGORIZED_COLOR = "#94a3b8"

  # Rows per INSERT when writing imports and materialised recurrences.
  CHUNK = 500

  class Base
    def initialize(db)
      @db = db
    end

    private

    attr_reader :db

    def unique_violation?(error)
      error.is_a?(PG::UniqueViolation) ||
        (error.respond_to?(:result) && error.result&.error_field(PG::PG_DIAG_SQLSTATE) == UNIQUE_VIOLATION)
    end
  end

  class UserRepository < Base
    COLUMNS = '"Id", "Email", "Name", "PasswordHash", "Currency", "CreatedAt"'

    def find_by_email(email)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Users" WHERE "Email" = $1), [ email ]).first
      row && to_user(row)
    end

    def find_by_id(user_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Users" WHERE "Id" = $1), [ user_id ]).first
      row && to_user(row)
    end

    def add(user)
      db.exec(
        %(INSERT INTO "Users" ("Id", "Email", "Name", "PasswordHash", "Currency", "CreatedAt")
          VALUES ($1, $2, $3, $4, $5, $6)),
        [ user.id, user.email, user.name, user.password_hash, user.currency,
          user.created_at.to_param ]
      )
    rescue PG::Error => e
      raise EmailTaken, user.email if unique_violation?(e)

      raise
    end

    def update_password_hash(user_id, password_hash)
      db.exec(%(UPDATE "Users" SET "PasswordHash" = $2 WHERE "Id" = $1), [ user_id, password_hash ])
    end

    def update_profile(user_id, name, currency)
      db.exec(%(UPDATE "Users" SET "Name" = $2, "Currency" = $3 WHERE "Id" = $1),
              [ user_id, name, currency ]).cmd_tuples.positive?
    end

    private

    def to_user(row)
      User.new(id: row["Id"], email: row["Email"], name: row["Name"],
               password_hash: row["PasswordHash"], currency: row["Currency"],
               created_at: Rows.instant(row["CreatedAt"]))
    end
  end

  class AccountRepository < Base
    COLUMNS = '"Id", "UserId", "Name", "Type", "InitialBalance", "Currency", "IsArchived", "CreatedAt"'

    # Active accounts first, then by name -- the order the UI expects.
    def list_all(user_id)
      db.exec(%(SELECT #{COLUMNS} FROM "Accounts" WHERE "UserId" = $1
                ORDER BY "IsArchived", "Name"), [ user_id ]).map { |row| to_account(row) }
    end

    def get(user_id, account_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2),
                    [ account_id, user_id ]).first
      row && to_account(row)
    end

    def exists?(user_id, account_id)
      db.exec(%(SELECT 1 FROM "Accounts" WHERE "Id" = $1 AND "UserId" = $2),
              [ account_id, user_id ]).ntuples.positive?
    end

    def add(account)
      db.exec(
        %(INSERT INTO "Accounts" ("Id", "UserId", "Name", "Type", "InitialBalance", "Currency",
          "IsArchived", "CreatedAt") VALUES ($1, $2, $3, $4, $5, $6, $7, $8)),
        [ account.id, account.user_id, account.name, account.type.ordinal,
          account.initial_balance.to_s, account.currency, account.is_archived,
          account.created_at.to_param ]
      )
    end

    def update(account)
      db.exec(
        %(UPDATE "Accounts" SET "Name" = $3, "Type" = $4, "InitialBalance" = $5, "Currency" = $6,
          "IsArchived" = $7 WHERE "Id" = $1 AND "UserId" = $2),
        [ account.id, account.user_id, account.name, account.type.ordinal,
          account.initial_balance.to_s, account.currency, account.is_archived ]
      )
    end

    def archive(user_id, account_id)
      db.exec(%(UPDATE "Accounts" SET "IsArchived" = true WHERE "Id" = $1 AND "UserId" = $2),
              [ account_id, user_id ]).cmd_tuples.positive?
    end

    private

    def to_account(row)
      Account.new(id: row["Id"], user_id: row["UserId"], name: row["Name"],
                  type: Rows.enum("AccountType", row["Type"]),
                  initial_balance: Rows.money(row["InitialBalance"]),
                  currency: row["Currency"], is_archived: Rows.flag(row["IsArchived"]),
                  created_at: Rows.instant(row["CreatedAt"]))
    end
  end

  class CategoryRepository < Base
    COLUMNS = '"Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault"'

    # The shared defaults plus the caller's own.
    def list_visible(user_id)
      db.exec(%(SELECT #{COLUMNS} FROM "Categories"
                WHERE "IsDefault" = true OR "UserId" = $1 ORDER BY "Type", "Name"),
              [ user_id ]).map { |row| to_category(row) }
    end

    def get(user_id, category_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Categories"
                      WHERE "Id" = $1 AND ("IsDefault" = true OR "UserId" = $2)),
                    [ category_id, user_id ]).first
      row && to_category(row)
    end

    def get_owned(user_id, category_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Categories" WHERE "Id" = $1 AND "UserId" = $2),
                    [ category_id, user_id ]).first
      row && to_category(row)
    end

    def add(category)
      db.exec(
        %(INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
          VALUES ($1, $2, $3, $4, $5, $6, $7)),
        [ category.id, category.user_id, category.name, category.type.ordinal,
          category.icon, category.color, category.is_default ]
      )
    end

    def update(category)
      db.exec(
        %(UPDATE "Categories" SET "Name" = $3, "Type" = $4, "Icon" = $5, "Color" = $6
          WHERE "Id" = $1 AND "UserId" = $2),
        [ category.id, category.user_id, category.name, category.type.ordinal,
          category.icon, category.color ]
      )
    end

    # Detaches the category from its transactions, then drops its budgets. All
    # three writes share one transaction: a half-applied delete would leave
    # transactions pointing at a row that no longer exists.
    def remove(user_id, category_id)
      db.transaction do |connection|
        connection.exec_params(
          %(UPDATE "Transactions" SET "CategoryId" = NULL WHERE "UserId" = $1 AND "CategoryId" = $2),
          [ user_id, category_id ]
        )
        connection.exec_params(%(DELETE FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2),
                               [ user_id, category_id ])
        connection.exec_params(%(DELETE FROM "Categories" WHERE "Id" = $1 AND "UserId" = $2),
                               [ category_id, user_id ])
      end
    end

    private

    def to_category(row)
      Category.new(id: row["Id"], user_id: row["UserId"], name: row["Name"],
                   type: Rows.enum("CategoryType", row["Type"]), icon: row["Icon"],
                   color: row["Color"], is_default: Rows.flag(row["IsDefault"]))
    end
  end
end

module Repositories
  class TransactionRepository < Base
    COLUMNS = '"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Date", ' \
              '"Description", "Notes", "TagsRaw", "TransferAccountId"'

    Filter = Struct.new(:account_id, :category_id, :type, :date_from, :date_to, :search,
                        :limit, :offset, keyword_init: true)

    # One page of matches plus the total number of them.
    def search(user_id, filter)
      where, args = predicate(user_id, filter)

      total = db.exec(%(SELECT COUNT(*) AS total FROM "Transactions"#{where}), args)
                .first["total"].to_i

      paged = args + [ filter.limit || 20, filter.offset || 0 ]
      rows = db.exec(
        %(SELECT #{COLUMNS} FROM "Transactions"#{where}
          ORDER BY "Date" DESC, "Id" DESC LIMIT $#{paged.length - 1} OFFSET $#{paged.length}),
        paged
      )
      [ rows.map { |row| to_transaction(row) }, total ]
    end

    def get(user_id, transaction_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2),
                    [ transaction_id, user_id ]).first
      row && to_transaction(row)
    end

    # Newest first -- the order the CSV export writes.
    def list_range(user_id, date_from, date_to)
      where, args = predicate(user_id, Filter.new(date_from: date_from, date_to: date_to))
      db.exec(%(SELECT #{COLUMNS} FROM "Transactions"#{where} ORDER BY "Date" DESC, "Id" DESC),
              args).map { |row| to_transaction(row) }
    end

    # Oldest first, so a running total accumulates in one pass.
    def slices(user_id, date_from, date_to)
      where, args = predicate(user_id, Filter.new(date_from: date_from, date_to: date_to))
      db.exec(%(SELECT "AccountId", "TransferAccountId", "CategoryId", "Type", "Amount", "Date"
                FROM "Transactions"#{where} ORDER BY "Date", "Id"), args).map do |row|
        Slice.new(account_id: row["AccountId"], transfer_account_id: row["TransferAccountId"],
                  category_id: row["CategoryId"],
                  type: Rows.enum("TransactionType", row["Type"]),
                  amount: Rows.money(row["Amount"]), date: Rows.instant(row["Date"]))
      end
    end

    def add(transaction) = add_many([ transaction ])

    # Chunked so one oversized import cannot build an unbounded statement.
    def add_many(transactions)
      transactions.each_slice(CHUNK) do |chunk|
        values = []
        rows = chunk.each_with_index.map do |item, index|
          base = index * 11
          values.push(item.id, item.user_id, item.account_id, item.category_id,
                      item.type.ordinal, item.amount.to_s, item.date.to_param,
                      item.description, item.notes, Domain::Tags.join(item.tags),
                      item.transfer_account_id)
          "(#{(1..11).map { |n| "$#{base + n}" }.join(', ')})"
        end

        db.exec(
          %(INSERT INTO "Transactions" ("Id", "UserId", "AccountId", "CategoryId", "Type",
            "Amount", "Date", "Description", "Notes", "TagsRaw", "TransferAccountId")
            VALUES #{rows.join(', ')}), values
        )
      end
    end

    def update(transaction)
      db.exec(
        %(UPDATE "Transactions" SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5,
          "Amount" = $6, "Date" = $7, "Description" = $8, "Notes" = $9, "TagsRaw" = $10,
          "TransferAccountId" = $11 WHERE "Id" = $1 AND "UserId" = $2),
        [ transaction.id, transaction.user_id, transaction.account_id, transaction.category_id,
          transaction.type.ordinal, transaction.amount.to_s, transaction.date.to_param,
          transaction.description, transaction.notes, Domain::Tags.join(transaction.tags),
          transaction.transfer_account_id ]
      )
    end

    def remove(user_id, transaction_id)
      db.exec(%(DELETE FROM "Transactions" WHERE "Id" = $1 AND "UserId" = $2),
              [ transaction_id, user_id ]).cmd_tuples.positive?
    end

    private

    # Builds the shared WHERE clause; both bounds are inclusive.
    def predicate(user_id, filter)
      clauses = [ %("UserId" = $1) ]
      args = [ user_id ]

      add = lambda do |clause, value|
        args << value
        clauses << clause.call(args.length)
      end

      if filter.account_id
        add.call(->(n) { %(("AccountId" = $#{n} OR "TransferAccountId" = $#{n})) }, filter.account_id)
      end
      add.call(->(n) { %("CategoryId" = $#{n}) }, filter.category_id) if filter.category_id
      add.call(->(n) { %("Type" = $#{n}) }, filter.type.ordinal) if filter.type
      add.call(->(n) { %("Date" >= $#{n}) }, filter.date_from.to_param) if filter.date_from
      add.call(->(n) { %("Date" <= $#{n}) }, filter.date_to.to_param) if filter.date_to
      if filter.search && !filter.search.empty?
        # Lowered on both sides: the term arrives already lowercased.
        add.call(->(n) { %(LOWER("Description") LIKE $#{n}) }, "%#{filter.search}%")
      end

      [ " WHERE #{clauses.join(' AND ')}", args ]
    end

    def to_transaction(row)
      Transaction.new(
        id: row["Id"], user_id: row["UserId"], account_id: row["AccountId"],
        category_id: row["CategoryId"], type: Rows.enum("TransactionType", row["Type"]),
        amount: Rows.money(row["Amount"]), date: Rows.instant(row["Date"]),
        description: row["Description"], notes: row["Notes"], tags: Rows.tags(row["TagsRaw"]),
        transfer_account_id: row["TransferAccountId"]
      )
    end
  end

  class BudgetRepository < Base
    COLUMNS = '"Id", "UserId", "CategoryId", "Month", "Limit"'

    # Unordered on purpose -- the service sorts by category id.
    def list_for_month(user_id, month)
      db.exec(%(SELECT #{COLUMNS} FROM "Budgets" WHERE "UserId" = $1 AND "Month" = $2),
              [ user_id, month ]).map { |row| to_budget(row) }
    end

    def get(user_id, budget_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2),
                    [ budget_id, user_id ]).first
      row && to_budget(row)
    end

    def exists?(user_id, category_id, month)
      db.exec(%(SELECT 1 FROM "Budgets" WHERE "UserId" = $1 AND "CategoryId" = $2 AND "Month" = $3),
              [ user_id, category_id, month ]).ntuples.positive?
    end

    def add(budget)
      db.exec(%(INSERT INTO "Budgets" ("Id", "UserId", "CategoryId", "Month", "Limit")
                VALUES ($1, $2, $3, $4, $5)),
              [ budget.id, budget.user_id, budget.category_id, budget.month, budget.limit.to_s ])
    end

    def update_limit(user_id, budget_id, limit)
      db.exec(%(UPDATE "Budgets" SET "Limit" = $3 WHERE "Id" = $1 AND "UserId" = $2),
              [ budget_id, user_id, limit.to_s ]).cmd_tuples.positive?
    end

    def remove(user_id, budget_id)
      db.exec(%(DELETE FROM "Budgets" WHERE "Id" = $1 AND "UserId" = $2),
              [ budget_id, user_id ]).cmd_tuples.positive?
    end

    private

    def to_budget(row)
      Budget.new(id: row["Id"], user_id: row["UserId"], category_id: row["CategoryId"],
                 month: row["Month"], limit: Rows.money(row["Limit"]))
    end
  end

  class GoalRepository < Base
    COLUMNS = '"Id", "UserId", "Name", "TargetAmount", "CurrentAmount", "TargetDate", "Color"'

    def list_all(user_id)
      db.exec(%(SELECT #{COLUMNS} FROM "Goals" WHERE "UserId" = $1 ORDER BY "Name"),
              [ user_id ]).map { |row| to_goal(row) }
    end

    def get(user_id, goal_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2),
                    [ goal_id, user_id ]).first
      row && to_goal(row)
    end

    def add(goal)
      db.exec(
        %(INSERT INTO "Goals" ("Id", "UserId", "Name", "TargetAmount", "CurrentAmount",
          "TargetDate", "Color") VALUES ($1, $2, $3, $4, $5, $6, $7)),
        [ goal.id, goal.user_id, goal.name, goal.target_amount.to_s, goal.current_amount.to_s,
          goal.target_date&.to_param, goal.color ]
      )
    end

    def update(goal)
      db.exec(
        %(UPDATE "Goals" SET "Name" = $3, "TargetAmount" = $4, "CurrentAmount" = $5,
          "TargetDate" = $6, "Color" = $7 WHERE "Id" = $1 AND "UserId" = $2),
        [ goal.id, goal.user_id, goal.name, goal.target_amount.to_s, goal.current_amount.to_s,
          goal.target_date&.to_param, goal.color ]
      )
    end

    def remove(user_id, goal_id)
      db.exec(%(DELETE FROM "Goals" WHERE "Id" = $1 AND "UserId" = $2),
              [ goal_id, user_id ]).cmd_tuples.positive?
    end

    private

    def to_goal(row)
      Goal.new(id: row["Id"], user_id: row["UserId"], name: row["Name"],
               target_amount: Rows.money(row["TargetAmount"]),
               current_amount: Rows.money(row["CurrentAmount"]),
               target_date: Rows.instant_or_nil(row["TargetDate"]), color: row["Color"])
    end
  end

  class RecurringRepository < Base
    COLUMNS = '"Id", "UserId", "AccountId", "CategoryId", "Type", "Amount", "Description", ' \
              '"Frequency", "StartDate", "EndDate", "NextRunDate", "IsActive"'

    def list_all(user_id)
      db.exec(%(SELECT #{COLUMNS} FROM "RecurringRules" WHERE "UserId" = $1
                ORDER BY "NextRunDate"), [ user_id ]).map { |row| to_rule(row) }
    end

    def get(user_id, rule_id)
      row = db.exec(%(SELECT #{COLUMNS} FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2),
                    [ rule_id, user_id ]).first
      row && to_rule(row)
    end

    # Every active rule due at or before the cutoff, across all users.
    def list_due(cutoff)
      db.exec(%(SELECT #{COLUMNS} FROM "RecurringRules"
                WHERE "IsActive" = true AND "NextRunDate" <= $1 ORDER BY "NextRunDate", "Id"),
              [ cutoff.to_param ]).map { |row| to_rule(row) }
    end

    def add(rule)
      db.exec(
        %(INSERT INTO "RecurringRules" ("Id", "UserId", "AccountId", "CategoryId", "Type",
          "Amount", "Description", "Frequency", "StartDate", "EndDate", "NextRunDate",
          "IsActive") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)),
        rule_params(rule)
      )
    end

    def update(rule)
      db.exec(
        %(UPDATE "RecurringRules" SET "AccountId" = $3, "CategoryId" = $4, "Type" = $5,
          "Amount" = $6, "Description" = $7, "Frequency" = $8, "StartDate" = $9,
          "EndDate" = $10, "NextRunDate" = $11, "IsActive" = $12
          WHERE "Id" = $1 AND "UserId" = $2), rule_params(rule)
      )
    end

    def remove(user_id, rule_id)
      db.exec(%(DELETE FROM "RecurringRules" WHERE "Id" = $1 AND "UserId" = $2),
              [ rule_id, user_id ]).cmd_tuples.positive?
    end

    private

    def rule_params(rule)
      [ rule.id, rule.user_id, rule.account_id, rule.category_id, rule.type.ordinal,
        rule.amount.to_s, rule.description, rule.frequency.ordinal, rule.start_date.to_param,
        rule.end_date&.to_param, rule.next_run_date.to_param, rule.is_active ]
    end

    def to_rule(row)
      RecurringRule.new(
        id: row["Id"], user_id: row["UserId"], account_id: row["AccountId"],
        category_id: row["CategoryId"], type: Rows.enum("TransactionType", row["Type"]),
        amount: Rows.money(row["Amount"]), description: row["Description"],
        frequency: Rows.enum("Frequency", row["Frequency"]),
        start_date: Rows.instant(row["StartDate"]),
        end_date: Rows.instant_or_nil(row["EndDate"]),
        next_run_date: Rows.instant(row["NextRunDate"]), is_active: Rows.flag(row["IsActive"])
      )
    end
  end
end
