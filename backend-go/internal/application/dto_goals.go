package application

import (
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// GoalDto is the wire shape of a savings goal.
type GoalDto struct {
	ID            uuid.UUID     `json:"id"`
	Name          string        `json:"name"`
	TargetAmount  domain.Amount `json:"targetAmount"`
	CurrentAmount domain.Amount `json:"currentAmount"`
	TargetDate    *time.Time    `json:"targetDate"`
	Color         string        `json:"color"`
}

// NewGoalDto projects a goal onto the wire.
func NewGoalDto(goal domain.Goal) GoalDto {
	return GoalDto{
		ID:            goal.Id,
		Name:          goal.Name,
		TargetAmount:  domain.NewAmount(goal.TargetAmount),
		CurrentAmount: domain.NewAmount(goal.CurrentAmount),
		TargetDate:    domain.NormalizeUTCPtr(goal.TargetDate),
		Color:         goal.Color,
	}
}

// GoalRequest is the body of POST and PUT /api/goals.
type GoalRequest struct {
	Name         string        `json:"name"`
	TargetAmount *domain.Money `json:"targetAmount"`
	// CurrentAmount is optional; an omitted balance means the goal starts empty.
	CurrentAmount *domain.Money `json:"currentAmount"`
	TargetDate    *Timestamp    `json:"targetDate"`
	Color         string        `json:"color"`
}

// Validate mirrors the attributes on FinanceTracker's GoalRequest.
func (r GoalRequest) Validate() error {
	errs := domain.NewValidationError()
	if r.TargetAmount == nil {
		requiredMembers(errs, []string{"targetAmount"})
		return errs.OrNil()
	}

	required(errs, FieldName, r.Name)
	maxLength(errs, FieldName, r.Name, nameMaxLength)

	decimalRange(errs, FieldTarget, r.TargetAmount, MoneyMinPositive)
	decimalRange(errs, FieldCurrent, r.CurrentAmount, MoneyMinZero)

	maxLength(errs, FieldColor, r.Color, colorMaxLength)

	return errs.OrNil()
}

// ContributeRequest is the body of POST /api/goals/{id}/contribute.
type ContributeRequest struct {
	Amount *domain.Money `json:"amount"`
}

// Validate mirrors the attributes on FinanceTracker's ContributeRequest.
func (r ContributeRequest) Validate() error {
	errs := domain.NewValidationError()
	if r.Amount == nil {
		requiredMembers(errs, []string{"amount"})
		return errs.OrNil()
	}

	decimalRange(errs, FieldAmount, r.Amount, MoneyMinPositive)
	return errs.OrNil()
}
