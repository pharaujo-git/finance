package application

import (
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// AccountDto is the wire shape of an account, balance included.
type AccountDto struct {
	ID         uuid.UUID          `json:"id"`
	Name       string             `json:"name"`
	Type       domain.AccountType `json:"type"`
	Balance    domain.Amount      `json:"balance"`
	Currency   string             `json:"currency"`
	IsArchived bool               `json:"isArchived"`
	CreatedAt  time.Time          `json:"createdAt"`
}

// NewAccountDto projects an account and its computed balance onto the wire.
func NewAccountDto(account domain.Account, balance domain.Money) AccountDto {
	return AccountDto{
		ID:         account.Id,
		Name:       account.Name,
		Type:       account.Type,
		Balance:    domain.NewAmount(balance),
		Currency:   account.Currency,
		IsArchived: account.IsArchived,
		CreatedAt:  account.CreatedAt.UTC(),
	}
}

// CreateAccountRequest is the body of POST /api/accounts.
type CreateAccountRequest struct {
	Name string              `json:"name"`
	Type *domain.AccountType `json:"type"`
	// InitialBalance is optional; an omitted opening balance means zero.
	InitialBalance *domain.Money `json:"initialBalance"`
	// Currency is a pointer so an omitted member keeps the "USD" the .NET DTO's
	// property initialiser supplies. Go cannot tell an omitted member from an
	// explicit null, so both take the default here; .NET rejects the null.
	Currency *string `json:"currency"`
}

// Validate mirrors the attributes on FinanceTracker's CreateAccountRequest.
func (r CreateAccountRequest) Validate() error {
	errs := domain.NewValidationError()
	if requiredMembers(errs, missingAccountMembers(r.Type)) {
		return errs.OrNil()
	}

	required(errs, FieldName, r.Name)
	maxLength(errs, FieldName, r.Name, nameMaxLength)

	currency := resolveCurrency(r.Currency)
	required(errs, FieldCurrency, currency)
	maxLength(errs, FieldCurrency, currency, currencyMaxLength)

	return errs.OrNil()
}

// UpdateAccountRequest is the body of PUT /api/accounts/{id}.
type UpdateAccountRequest struct {
	Name     string              `json:"name"`
	Type     *domain.AccountType `json:"type"`
	Currency *string             `json:"currency"`
	// IsArchived is optional; an omitted flag leaves the account active.
	IsArchived *bool `json:"isArchived"`
}

// Validate mirrors the attributes on FinanceTracker's UpdateAccountRequest.
func (r UpdateAccountRequest) Validate() error {
	errs := domain.NewValidationError()
	if requiredMembers(errs, missingAccountMembers(r.Type)) {
		return errs.OrNil()
	}

	required(errs, FieldName, r.Name)
	maxLength(errs, FieldName, r.Name, nameMaxLength)

	currency := resolveCurrency(r.Currency)
	required(errs, FieldCurrency, currency)
	maxLength(errs, FieldCurrency, currency, currencyMaxLength)

	return errs.OrNil()
}

func missingAccountMembers(accountType *domain.AccountType) []string {
	if accountType == nil {
		return []string{"type"}
	}
	return nil
}

// resolveCurrency applies the `= "USD"` initialiser the .NET DTOs carry: an
// absent member keeps the default, an explicit null does not.
func resolveCurrency(value *string) string {
	if value == nil {
		return DefaultCurrency
	}
	return *value
}
