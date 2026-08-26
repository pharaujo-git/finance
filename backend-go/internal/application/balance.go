package application

import (
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// TransactionSlice is the minimum a transaction contributes to any balance
// calculation, mirroring FinanceTracker's record of the same name.
type TransactionSlice struct {
	AccountID         uuid.UUID
	TransferAccountID *uuid.UUID
	CategoryID        *uuid.UUID
	Type              domain.TransactionType
	Amount            domain.Money
	Date              time.Time
}

// DeltaFor is the signed effect of one slice on one specific account. A
// transfer credits its destination and debits its source; income credits, and
// everything else debits.
func DeltaFor(accountID uuid.UUID, slice TransactionSlice) domain.Money {
	if slice.Type == domain.TransactionTransfer &&
		slice.TransferAccountID != nil && *slice.TransferAccountID == accountID {
		return slice.Amount
	}

	if slice.AccountID != accountID {
		return domain.Zero()
	}

	if slice.Type == domain.TransactionIncome {
		return slice.Amount
	}
	return slice.Amount.Neg()
}

// NetWorthDelta is the signed effect on total net worth: transfers between the
// user's own accounts cancel out and contribute nothing.
func NetWorthDelta(slice TransactionSlice) domain.Money {
	switch slice.Type {
	case domain.TransactionIncome:
		return slice.Amount
	case domain.TransactionExpense:
		return slice.Amount.Neg()
	case domain.TransactionTransfer:
		return domain.Zero()
	default:
		return domain.Zero()
	}
}

// BalanceOf is the account's opening balance plus the signed sum of every
// slice, which is how both APIs derive a live balance.
func BalanceOf(account domain.Account, slices []TransactionSlice) domain.Money {
	total := domain.Zero()
	for _, slice := range slices {
		total = total.Add(DeltaFor(account.Id, slice))
	}
	return account.InitialBalance.Add(total)
}
