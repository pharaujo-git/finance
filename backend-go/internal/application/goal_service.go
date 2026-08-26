package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// contributionMessage is the 400 a non-positive contribution raises.
const contributionMessage = "Contribution amount must be greater than zero."

// GoalService is savings goals and contributions, the Go twin of
// FinanceTracker.Application.Services.GoalService.
type GoalService struct {
	goals GoalRepository

	newID func() uuid.UUID
}

// NewGoalService wires the service to its port.
func NewGoalService(goals GoalRepository) *GoalService {
	return &GoalService{goals: goals, newID: uuid.New}
}

// List returns the caller's goals by name.
func (s *GoalService) List(ctx context.Context, userID uuid.UUID) ([]GoalDto, error) {
	goals, err := s.goals.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing goals: %w", err)
	}

	dtos := make([]GoalDto, 0, len(goals))
	for _, goal := range goals {
		dtos = append(dtos, NewGoalDto(goal))
	}
	return dtos, nil
}

// Create opens a savings goal.
func (s *GoalService) Create(
	ctx context.Context,
	userID uuid.UUID,
	request GoalRequest,
) (GoalDto, error) {
	if err := request.Validate(); err != nil {
		return GoalDto{}, err
	}

	goal := &domain.Goal{Id: s.newID(), UserId: userID}
	applyGoal(goal, request)

	if err := s.goals.Add(ctx, goal); err != nil {
		return GoalDto{}, fmt.Errorf("application: inserting goal: %w", err)
	}
	return NewGoalDto(*goal), nil
}

// Update rewrites a goal, including its current balance.
func (s *GoalService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	request GoalRequest,
) (GoalDto, error) {
	if err := request.Validate(); err != nil {
		return GoalDto{}, err
	}

	goal, err := s.load(ctx, userID, id)
	if err != nil {
		return GoalDto{}, err
	}

	applyGoal(goal, request)
	if err := s.goals.Update(ctx, goal); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return GoalDto{}, domain.NotFound(goalEntityName)
		}
		return GoalDto{}, fmt.Errorf("application: updating goal: %w", err)
	}
	return NewGoalDto(*goal), nil
}

// Delete removes a goal the caller owns.
func (s *GoalService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.load(ctx, userID, id); err != nil {
		return err
	}

	if err := s.goals.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return domain.NotFound(goalEntityName)
		}
		return fmt.Errorf("application: deleting goal: %w", err)
	}
	return nil
}

// Contribute adds to a goal's balance. The DTO's range rule rejects anything
// below a cent first; the check here is the service's own guard, kept so the
// rule survives a caller that skips validation.
func (s *GoalService) Contribute(
	ctx context.Context,
	userID, id uuid.UUID,
	request ContributeRequest,
) (GoalDto, error) {
	if err := request.Validate(); err != nil {
		return GoalDto{}, err
	}
	if !request.Amount.IsPositive() {
		return GoalDto{}, domain.BadRequest(contributionMessage)
	}

	goal, err := s.load(ctx, userID, id)
	if err != nil {
		return GoalDto{}, err
	}

	goal.CurrentAmount = domain.RoundMoney(goal.CurrentAmount.Add(*request.Amount))
	if err := s.goals.Update(ctx, goal); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return GoalDto{}, domain.NotFound(goalEntityName)
		}
		return GoalDto{}, fmt.Errorf("application: updating goal: %w", err)
	}
	return NewGoalDto(*goal), nil
}

func (s *GoalService) load(ctx context.Context, userID, id uuid.UUID) (*domain.Goal, error) {
	goal, err := s.goals.Get(ctx, id, userID)
	switch {
	case errors.Is(err, ErrRowNotFound):
		return nil, domain.NotFound(goalEntityName)
	case err != nil:
		return nil, fmt.Errorf("application: reading goal: %w", err)
	}
	return goal, nil
}

func applyGoal(goal *domain.Goal, request GoalRequest) {
	current := domain.Zero()
	if request.CurrentAmount != nil {
		current = *request.CurrentAmount
	}

	goal.Name = strings.TrimSpace(request.Name)
	goal.TargetAmount = domain.RoundMoney(*request.TargetAmount)
	goal.CurrentAmount = domain.RoundMoney(current)
	goal.TargetDate = TimePtr(request.TargetDate)
	goal.Color = strings.TrimSpace(request.Color)
}
