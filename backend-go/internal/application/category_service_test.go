package application_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestCategoryListMixesDefaultsAndOwnRows(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	h.seedDefaultCategory("Groceries")
	h.seedCategory("Coffee", domain.CategoryExpense)
	h.seedCategory("Salary", domain.CategoryIncome)

	// Another user's private category is invisible here.
	other := uuid.New()
	h.store.SeedCategory(domain.Category{Id: uuid.New(), UserId: &other, Name: "Secret"})

	categories, err := h.categories.List(h.ctx(), h.userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	names := make([]string, 0, len(categories))
	for _, category := range categories {
		names = append(names, category.Name)
	}
	// Ordered by type (income first) then by name.
	want := []string{"Salary", "Coffee", "Groceries"}
	if len(names) != len(want) {
		t.Fatalf("categories = %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Errorf("categories = %v, want %v", names, want)
			break
		}
	}
}

func TestDefaultCategoriesCannotBeModified(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	shared := h.seedDefaultCategory("Groceries")

	_, err := h.categories.Update(h.ctx(), h.userID, shared.Id, application.CategoryRequest{
		Name: "Renamed",
		Type: categoryType(domain.CategoryExpense),
	})
	requireAppError(t, err, domain.KindValidation, "Default categories cannot be modified.")

	err = h.categories.Delete(h.ctx(), h.userID, shared.Id)
	requireAppError(t, err, domain.KindValidation, "Default categories cannot be modified.")

	if h.store.CountCategories() != 1 {
		t.Errorf("categories = %d, want the default untouched", h.store.CountCategories())
	}
}

func TestCategoryTypeIsEditable(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	updated, err := h.categories.Update(h.ctx(), h.userID, category.ID, application.CategoryRequest{
		Name:  " Side gig ",
		Type:  categoryType(domain.CategoryIncome),
		Icon:  " briefcase ",
		Color: " #abcdef ",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Type != domain.CategoryIncome || updated.Name != "Side gig" ||
		updated.Icon != "briefcase" || updated.Color != "#abcdef" {
		t.Errorf("category = %+v, want a retyped and trimmed row", updated)
	}
}

func TestDeleteCategoryDetachesTransactionsAndDropsBudgets(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	account := h.seedAccount("Checking", "0.00")
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	transaction := h.addTransaction(account.ID, domain.TransactionExpense, "4.00", date(2026, 8, 1),
		withCategory(category.ID))

	if _, err := h.budgets.Create(h.ctx(), h.userID, application.CreateBudgetRequest{
		CategoryID: identifier(category.ID),
		Month:      "2026-08",
		Limit:      moneyPtr("100.00"),
	}); err != nil {
		t.Fatalf("creating a budget: %v", err)
	}

	if err := h.categories.Delete(h.ctx(), h.userID, category.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	stored, ok := h.store.StoredTransaction(transaction.ID)
	if !ok || stored.CategoryId != nil {
		t.Errorf("transaction = %+v, want its category detached", stored)
	}
	if h.store.CountBudgets() != 0 {
		t.Errorf("budgets = %d, want the category's budgets dropped", h.store.CountBudgets())
	}
	if h.store.CountCategories() != 0 {
		t.Errorf("categories = %d, want the row gone", h.store.CountCategories())
	}
}

func TestCategoriesOfAnotherUserAreNotFound(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	category := h.seedCategory("Coffee", domain.CategoryExpense)

	_, err := h.categories.Update(h.ctx(), h.otherID, category.ID, application.CategoryRequest{
		Name: "Theirs now",
		Type: categoryType(domain.CategoryExpense),
	})
	requireAppError(t, err, domain.KindNotFound, "Category was not found.")

	err = h.categories.Delete(h.ctx(), h.otherID, category.ID)
	requireAppError(t, err, domain.KindNotFound, "Category was not found.")

	err = h.categories.Delete(h.ctx(), h.userID, uuid.New())
	requireAppError(t, err, domain.KindNotFound, "Category was not found.")
}

func TestEnsureUsableAcceptsNothingAndDefaults(t *testing.T) {
	t.Parallel()

	h := newServices(t)
	shared := h.seedDefaultCategory("Groceries")

	if err := h.categories.EnsureUsable(h.ctx(), h.userID, nil); err != nil {
		t.Errorf("EnsureUsable(nil) = %v, want nil: a row may be uncategorized", err)
	}
	if err := h.categories.EnsureUsable(h.ctx(), h.userID, identifier(shared.Id)); err != nil {
		t.Errorf("EnsureUsable(default) = %v, want nil", err)
	}

	err := h.categories.EnsureUsable(h.ctx(), h.userID, identifier(uuid.New()))
	requireAppError(t, err, domain.KindNotFound, "Category was not found.")
}
