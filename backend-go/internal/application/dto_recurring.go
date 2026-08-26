package application

import (
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// RecurringRuleDto is the wire shape of a recurring rule.
type RecurringRuleDto struct {
	ID          uuid.UUID              `json:"id"`
	AccountID   uuid.UUID              `json:"accountId"`
	CategoryID  *uuid.UUID             `json:"categoryId"`
	Type        domain.TransactionType `json:"type"`
	Amount      domain.Amount          `json:"amount"`
	Description string                 `json:"description"`
	Frequency   domain.Frequency       `json:"frequency"`
	StartDate   time.Time              `json:"startDate"`
	EndDate     *time.Time             `json:"endDate"`
	NextRunDate time.Time              `json:"nextRunDate"`
	IsActive    bool                   `json:"isActive"`
}

// NewRecurringRuleDto projects a rule onto the wire.
func NewRecurringRuleDto(rule domain.RecurringRule) RecurringRuleDto {
	return RecurringRuleDto{
		ID:          rule.Id,
		AccountID:   rule.AccountId,
		CategoryID:  rule.CategoryId,
		Type:        rule.Type,
		Amount:      domain.NewAmount(rule.Amount),
		Description: rule.Description,
		Frequency:   rule.Frequency,
		StartDate:   rule.StartDate.UTC(),
		EndDate:     domain.NormalizeUTCPtr(rule.EndDate),
		NextRunDate: rule.NextRunDate.UTC(),
		IsActive:    rule.IsActive,
	}
}

// RecurringRuleRequest is the body of POST and PUT /api/recurring.
type RecurringRuleRequest struct {
	AccountID   *uuid.UUID              `json:"accountId"`
	CategoryID  *uuid.UUID              `json:"categoryId"`
	Type        *domain.TransactionType `json:"type"`
	Amount      *domain.Money           `json:"amount"`
	Description string                  `json:"description"`
	Frequency   *domain.Frequency       `json:"frequency"`
	StartDate   *Timestamp              `json:"startDate"`
	EndDate     *Timestamp              `json:"endDate"`
	// IsActive is optional; an omitted flag leaves the rule running.
	IsActive *bool `json:"isActive"`
}

// Validate mirrors the attributes on FinanceTracker's RecurringRuleRequest.
func (r RecurringRuleRequest) Validate() error {
	errs := domain.NewValidationError()

	var missing []string
	if r.AccountID == nil {
		missing = append(missing, "accountId")
	}
	if r.Type == nil {
		missing = append(missing, "type")
	}
	if r.Amount == nil {
		missing = append(missing, "amount")
	}
	if r.Frequency == nil {
		missing = append(missing, "frequency")
	}
	if r.StartDate == nil {
		missing = append(missing, "startDate")
	}
	if requiredMembers(errs, missing) {
		return errs.OrNil()
	}

	decimalRange(errs, FieldAmount, r.Amount, MoneyMinPositive)

	required(errs, FieldDescription, r.Description)
	maxLength(errs, FieldDescription, r.Description, descriptionMaxLength)

	return errs.OrNil()
}
