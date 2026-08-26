package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Advance clamps rather than overflowing, which is what DateTime.AddMonths
// does and what Go's own AddDate does not.
func TestAdvanceEdgeCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		from      time.Time
		frequency domain.Frequency
		want      time.Time
	}{
		{"daily", date(2026, 8, 31), domain.FrequencyDaily, date(2026, 9, 1)},
		{"weekly", date(2026, 8, 26), domain.FrequencyWeekly, date(2026, 9, 2)},
		{"monthly", date(2026, 8, 15), domain.FrequencyMonthly, date(2026, 9, 15)},
		{"monthly clamps onto a shorter month", date(2026, 1, 31), domain.FrequencyMonthly, date(2026, 2, 28)},
		{"monthly stays clamped", date(2026, 2, 28), domain.FrequencyMonthly, date(2026, 3, 28)},
		{"monthly rolls the year", date(2026, 12, 31), domain.FrequencyMonthly, date(2027, 1, 31)},
		{"yearly", date(2026, 3, 1), domain.FrequencyYearly, date(2027, 3, 1)},
		{"yearly clamps a leap day", date(2028, 2, 29), domain.FrequencyYearly, date(2029, 2, 28)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := application.Advance(testCase.from, testCase.frequency)
			if !got.Equal(testCase.want) {
				t.Errorf("Advance(%s) = %s, want %s",
					testCase.from.Format(time.DateOnly), got.Format(time.DateOnly),
					testCase.want.Format(time.DateOnly))
			}
		})
	}
}

func TestCreateRecurringRuleStartsAtItsStartDate(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	rule, err := h.recurring.Create(h.ctx(), h.userID, application.RecurringRuleRequest{
		AccountID:   identifier(account.ID),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("12.00"),
		Description: "  Streaming  ",
		Frequency:   frequency(domain.FrequencyMonthly),
		StartDate:   timestamp(date(2026, 9, 1)),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !rule.NextRunDate.Equal(date(2026, 9, 1)) {
		t.Errorf("nextRunDate = %s, want the start date", rule.NextRunDate)
	}
	if rule.Description != "Streaming" || !rule.IsActive {
		t.Errorf("rule = %+v, want a trimmed, active rule", rule)
	}
}

func TestRecurringRuleValidation(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	base := func() application.RecurringRuleRequest {
		return application.RecurringRuleRequest{
			AccountID:   identifier(account.ID),
			Type:        transactionType(domain.TransactionExpense),
			Amount:      moneyPtr("12.00"),
			Description: "Rent",
			Frequency:   frequency(domain.FrequencyMonthly),
			StartDate:   timestamp(date(2026, 9, 1)),
		}
	}

	request := base()
	request.Type = transactionType(domain.TransactionTransfer)
	_, err := h.recurring.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindValidation, "Recurring transfers are not supported.")

	request = base()
	request.AccountID = identifier(uuid.New())
	_, err = h.recurring.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindNotFound, "Account was not found.")

	request = base()
	request.EndDate = timestamp(date(2026, 8, 1))
	_, err = h.recurring.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindValidation, "End date must not be before the start date.")

	request = base()
	request.CategoryID = identifier(uuid.New())
	_, err = h.recurring.Create(h.ctx(), h.userID, request)
	requireAppError(t, err, domain.KindNotFound, "Category was not found.")

	_, err = h.recurring.Create(h.ctx(), h.userID, application.RecurringRuleRequest{})
	requireValidationError(t, err, "$",
		"The JSON payload was missing required properties, including the following: "+
			"accountId, type, amount, frequency, startDate")
}

func TestUpdateRecurringRulePullsTheNextRunForward(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	rule, err := h.recurring.Create(h.ctx(), h.userID, application.RecurringRuleRequest{
		AccountID:   identifier(account.ID),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("12.00"),
		Description: "Rent",
		Frequency:   frequency(domain.FrequencyMonthly),
		StartDate:   timestamp(date(2026, 1, 1)),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := h.recurring.Update(h.ctx(), h.userID, rule.ID, application.RecurringRuleRequest{
		AccountID:   identifier(account.ID),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("15.00"),
		Description: "Rent",
		Frequency:   frequency(domain.FrequencyMonthly),
		StartDate:   timestamp(date(2026, 10, 1)),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.NextRunDate.Equal(date(2026, 10, 1)) {
		t.Errorf("nextRunDate = %s, want it moved up to the new start date", updated.NextRunDate)
	}
}

func TestMaterializeCreatesEveryDueOccurrence(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	category := h.seedCategory("Rent", domain.CategoryExpense)

	rule, err := h.recurring.Create(h.ctx(), h.userID, application.RecurringRuleRequest{
		AccountID:   identifier(account.ID),
		CategoryID:  identifier(category.ID),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("100.00"),
		Description: "Rent",
		Frequency:   frequency(domain.FrequencyMonthly),
		StartDate:   timestamp(date(2026, 6, 1)),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	created, err := h.recurring.MaterializeDue(h.ctx(), fixedNow)
	if err != nil {
		t.Fatalf("MaterializeDue: %v", err)
	}
	// June, July and August are due on 26 August 2026; September is not.
	if created != 3 {
		t.Fatalf("created = %d, want 3", created)
	}

	stored, _ := h.store.StoredRule(rule.ID)
	if !stored.NextRunDate.Equal(date(2026, 9, 1)) || !stored.IsActive {
		t.Errorf("rule = %+v, want an active rule due on 2026-09-01", stored)
	}

	page, err := h.transactions.Search(h.ctx(), h.userID, application.TransactionQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("transactions = %d, want 3", page.Total)
	}
	for _, item := range page.Items {
		if len(item.Tags) != 1 || item.Tags[0] != "recurring" {
			t.Errorf("tags = %v, want [recurring]", item.Tags)
		}
		if item.CategoryID == nil || *item.CategoryID != category.ID {
			t.Errorf("categoryId = %v, want the rule's category", item.CategoryID)
		}
		if item.TransferAccountID != nil {
			t.Error("a materialized transaction carried a destination account")
		}
	}

	// A second pass with nothing newly due creates nothing.
	again, err := h.recurring.MaterializeDue(h.ctx(), fixedNow)
	if err != nil {
		t.Fatalf("MaterializeDue: %v", err)
	}
	if again != 0 {
		t.Errorf("second pass created %d, want 0", again)
	}
}

func TestMaterializeStopsAtTheEndDate(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	end := date(2026, 7, 15)

	rule := domain.RecurringRule{
		Id:          uuid.New(),
		UserId:      h.userID,
		AccountId:   account.ID,
		Type:        domain.TransactionExpense,
		Amount:      money("10.00"),
		Description: "Ends soon",
		Frequency:   domain.FrequencyMonthly,
		StartDate:   date(2026, 6, 1),
		EndDate:     &end,
		NextRunDate: date(2026, 6, 1),
		IsActive:    true,
	}
	h.store.SeedRule(rule)

	created, err := h.recurring.MaterializeDue(h.ctx(), fixedNow)
	if err != nil {
		t.Fatalf("MaterializeDue: %v", err)
	}
	// June runs; July's occurrence on the 1st runs; the August one is past the
	// end date, which deactivates the rule.
	if created != 2 {
		t.Fatalf("created = %d, want 2", created)
	}

	stored, _ := h.store.StoredRule(rule.Id)
	if stored.IsActive {
		t.Error("isActive = true, want the rule retired at its end date")
	}
}

func TestMaterializeIgnoresInactiveRules(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	h.store.SeedRule(domain.RecurringRule{
		Id:          uuid.New(),
		UserId:      h.userID,
		AccountId:   account.ID,
		Type:        domain.TransactionExpense,
		Amount:      money("10.00"),
		Description: "Paused",
		Frequency:   domain.FrequencyDaily,
		StartDate:   date(2026, 1, 1),
		NextRunDate: date(2026, 1, 1),
		IsActive:    false,
	})

	created, err := h.recurring.MaterializeDue(h.ctx(), fixedNow)
	if err != nil {
		t.Fatalf("MaterializeDue: %v", err)
	}
	if created != 0 {
		t.Errorf("created = %d, want 0 for an inactive rule", created)
	}
}

// A rule left alone for years must not loop forever: one pass materializes at
// most MaxOccurrencesPerPass occurrences and leaves the rest for the next one.
func TestMaterializeCapsOnePass(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	start := date(2000, 1, 1)

	rule := domain.RecurringRule{
		Id:          uuid.New(),
		UserId:      h.userID,
		AccountId:   account.ID,
		Type:        domain.TransactionExpense,
		Amount:      money("1.00"),
		Description: "Ancient",
		Frequency:   domain.FrequencyDaily,
		StartDate:   start,
		NextRunDate: start,
		IsActive:    true,
	}
	h.store.SeedRule(rule)

	created, err := h.recurring.MaterializeDue(h.ctx(), fixedNow)
	if err != nil {
		t.Fatalf("MaterializeDue: %v", err)
	}
	if created != application.MaxOccurrencesPerPass {
		t.Fatalf("created = %d, want %d", created, application.MaxOccurrencesPerPass)
	}

	stored, _ := h.store.StoredRule(rule.Id)
	want := start.AddDate(0, 0, application.MaxOccurrencesPerPass)
	if !stored.NextRunDate.Equal(want) {
		t.Errorf("nextRunDate = %s, want %s", stored.NextRunDate, want)
	}
	if !stored.IsActive {
		t.Error("isActive = false, want the rule still running after a capped pass")
	}
}

func TestRecurringRulesOfAnotherUserAreNotFound(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")

	rule, err := h.recurring.Create(h.ctx(), h.userID, application.RecurringRuleRequest{
		AccountID:   identifier(account.ID),
		Type:        transactionType(domain.TransactionExpense),
		Amount:      moneyPtr("12.00"),
		Description: "Rent",
		Frequency:   frequency(domain.FrequencyMonthly),
		StartDate:   timestamp(date(2026, 9, 1)),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = h.recurring.Delete(h.ctx(), h.otherID, rule.ID)
	requireAppError(t, err, domain.KindNotFound, "Recurring rule was not found.")

	rules, err := h.recurring.List(h.ctx(), h.otherID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("rules = %d, want none for another user", len(rules))
	}
}
