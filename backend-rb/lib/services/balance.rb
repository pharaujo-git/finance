# frozen_string_literal: true

module Services
  # How a transaction moves an account balance and the net worth.
  #
  # Amounts are always stored positive; the type carries the sign. A transfer
  # debits its source and credits its destination, which is why the destination
  # check comes first -- the destination is not the row's "AccountId".
  module Balance
    module_function

    # What one transaction does to one account's balance.
    def delta_for(account_id, item)
      # The receiving side of a transfer, which the row records as the
      # *transfer* account rather than the owning one.
      if item.type.is?(Domain::Enums::TRANSACTION_TRANSFER) &&
         !item.transfer_account_id.nil? && item.transfer_account_id == account_id
        return item.amount
      end

      return Domain::MONEY_ZERO unless item.account_id == account_id
      return item.amount if item.type.is?(Domain::Enums::TRANSACTION_INCOME)

      # An expense and the paying side of a transfer both debit.
      item.amount.negate
    end

    # What one transaction does to the total. A transfer moves money, so zero.
    def net_worth_delta(item)
      return item.amount if item.type.is?(Domain::Enums::TRANSACTION_INCOME)
      return item.amount.negate if item.type.is?(Domain::Enums::TRANSACTION_EXPENSE)

      Domain::MONEY_ZERO
    end

    # The opening balance plus every movement. Deliberately not rounded.
    def balance_of(account, slices)
      total = slices.reduce(Domain::MONEY_ZERO) { |sum, item| sum.add(delta_for(account.id, item)) }
      account.initial_balance.add(total)
    end
  end
end
