package application

import (
	"regexp"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// monthPattern is the [RegularExpression(@"^\d{4}-\d{2}$")] on
// CreateBudgetRequest.Month. It is looser than MonthKey.TryParse, which the
// service applies afterwards, so "2026-13" fails the second check, not this one.
var monthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)

// BudgetDto is the wire shape of a budget, with the month's spend computed.
type BudgetDto struct {
	ID         uuid.UUID     `json:"id"`
	CategoryID uuid.UUID     `json:"categoryId"`
	Month      string        `json:"month"`
	Limit      domain.Amount `json:"limit"`
	Spent      domain.Amount `json:"spent"`
	Remaining  domain.Amount `json:"remaining"`
}

// NewBudgetDto projects a budget and its spend onto the wire.
func NewBudgetDto(budget domain.Budget, spent domain.Money) BudgetDto {
	return BudgetDto{
		ID:         budget.Id,
		CategoryID: budget.CategoryId,
		Month:      budget.Month,
		Limit:      domain.NewAmount(budget.Limit),
		Spent:      domain.NewAmount(spent),
		Remaining:  domain.NewAmount(budget.Limit.Sub(spent)),
	}
}

// CreateBudgetRequest is the body of POST /api/budgets.
type CreateBudgetRequest struct {
	CategoryID *uuid.UUID    `json:"categoryId"`
	Month      string        `json:"month"`
	Limit      *domain.Money `json:"limit"`
}

// Validate mirrors the attributes on FinanceTracker's CreateBudgetRequest.
func (r CreateBudgetRequest) Validate() error {
	errs := domain.NewValidationError()

	var missing []string
	if r.CategoryID == nil {
		missing = append(missing, "categoryId")
	}
	if r.Limit == nil {
		missing = append(missing, "limit")
	}
	if requiredMembers(errs, missing) {
		return errs.OrNil()
	}

	required(errs, FieldMonth, r.Month)
	if r.Month != "" && !monthPattern.MatchString(r.Month) {
		errs.Add(FieldMonth, monthFormatMessage)
	}

	decimalRange(errs, FieldLimit, r.Limit, MoneyMinZero)

	return errs.OrNil()
}

// UpdateBudgetRequest is the body of PUT /api/budgets/{id}.
type UpdateBudgetRequest struct {
	Limit *domain.Money `json:"limit"`
}

// Validate mirrors the attributes on FinanceTracker's UpdateBudgetRequest.
func (r UpdateBudgetRequest) Validate() error {
	errs := domain.NewValidationError()
	if r.Limit == nil {
		requiredMembers(errs, []string{"limit"})
		return errs.OrNil()
	}

	decimalRange(errs, FieldLimit, r.Limit, MoneyMinZero)
	return errs.OrNil()
}
