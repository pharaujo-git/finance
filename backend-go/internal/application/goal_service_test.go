package application_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestCreateGoalNormalisesItsFields(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	goal, err := h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
		Name:         "  New bike  ",
		TargetAmount: moneyPtr("1200.005"),
		TargetDate:   timestamp(date(2027, 3, 1)),
		Color:        " #ff8800 ",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if goal.Name != "New bike" || goal.Color != "#ff8800" {
		t.Errorf("goal = %+v, want trimmed text", goal)
	}
	requireMoney(t, goal.TargetAmount, "1200.01")
	// An omitted balance starts the goal empty.
	requireMoney(t, goal.CurrentAmount, "0")
	if goal.TargetDate == nil || !goal.TargetDate.Equal(date(2027, 3, 1)) {
		t.Errorf("targetDate = %v, want 2027-03-01", goal.TargetDate)
	}
}

func TestGoalValidation(t *testing.T) {
	t.Parallel()

	h := newServices(t)

	_, err := h.goals.Create(h.ctx(), h.userID, application.GoalRequest{Name: "No target"})
	requireValidationError(t, err, "$",
		"The JSON payload was missing required properties, including the following: targetAmount")

	_, err = h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
		Name:         "Free",
		TargetAmount: moneyPtr("0.00"),
	})
	requireValidationError(t, err, "TargetAmount",
		"The field TargetAmount must be between 0.01 and 999999999999.99.")

	_, err = h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
		TargetAmount: moneyPtr("10.00"),
	})
	requireValidationError(t, err, "Name", "The Name field is required.")
}

func TestContributeAddsToTheBalance(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	goal, err := h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
		Name:          "Trip",
		TargetAmount:  moneyPtr("1000.00"),
		CurrentAmount: moneyPtr("100.00"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := h.goals.Contribute(h.ctx(), h.userID, goal.ID, application.ContributeRequest{
		Amount: moneyPtr("25.50"),
	})
	if err != nil {
		t.Fatalf("Contribute: %v", err)
	}
	requireMoney(t, updated.CurrentAmount, "125.50")

	stored, _ := h.store.StoredGoal(goal.ID)
	if !stored.CurrentAmount.Equal(money("125.50")) {
		t.Errorf("stored balance = %s, want 125.50", stored.CurrentAmount)
	}
}

func TestContributeRejectsNothing(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	goal, err := h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
		Name:         "Trip",
		TargetAmount: moneyPtr("1000.00"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = h.goals.Contribute(h.ctx(), h.userID, goal.ID, application.ContributeRequest{
		Amount: moneyPtr("0.00"),
	})
	requireValidationError(t, err, "Amount",
		"The field Amount must be between 0.01 and 999999999999.99.")

	_, err = h.goals.Contribute(h.ctx(), h.userID, goal.ID, application.ContributeRequest{})
	requireValidationError(t, err, "$",
		"The JSON payload was missing required properties, including the following: amount")
}

func TestGoalsOfAnotherUserAreNotFound(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	goal, err := h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
		Name:         "Trip",
		TargetAmount: moneyPtr("1000.00"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = h.goals.Contribute(h.ctx(), h.otherID, goal.ID, application.ContributeRequest{
		Amount: moneyPtr("1.00"),
	})
	requireAppError(t, err, domain.KindNotFound, "Goal was not found.")

	err = h.goals.Delete(h.ctx(), h.otherID, goal.ID)
	requireAppError(t, err, domain.KindNotFound, "Goal was not found.")

	err = h.goals.Delete(h.ctx(), h.userID, uuid.New())
	requireAppError(t, err, domain.KindNotFound, "Goal was not found.")
}

func TestGoalListIsOrderedByName(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	for _, name := range []string{"Zebra fund", "Anchor fund"} {
		if _, err := h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
			Name:         name,
			TargetAmount: moneyPtr("10.00"),
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	goals, err := h.goals.List(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(goals) != 2 || goals[0].Name != "Anchor fund" {
		t.Errorf("goals = %+v, want Anchor fund first", goals)
	}
}

func TestUpdateGoalRewritesEverything(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	goal, err := h.goals.Create(h.ctx(), h.userID, application.GoalRequest{
		Name:          "Trip",
		TargetAmount:  moneyPtr("1000.00"),
		CurrentAmount: moneyPtr("100.00"),
		TargetDate:    timestamp(date(2027, 1, 1)),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := h.goals.Update(h.ctx(), h.userID, goal.ID, application.GoalRequest{
		Name:         "Bigger trip",
		TargetAmount: moneyPtr("2000.00"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	// An omitted balance resets it, and an omitted date clears it: Apply writes
	// every field.
	requireMoney(t, updated.CurrentAmount, "0")
	if updated.TargetDate != nil {
		t.Errorf("targetDate = %v, want nil", updated.TargetDate)
	}
}
