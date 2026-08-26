package apptest

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Store is an in-memory stand-in for every repository port except the user one,
// which predates it. Each method hands out copies, so a caller that mutates
// what it read cannot change the "database" without going back through the
// port — which is what makes assertions about persistence meaningful.
//
// Ordering, filtering and the ownership scoping reproduce what the SQL
// repositories ask Postgres for, so a service test and an integration test
// exercise the same rules.
type Store struct {
	mu sync.Mutex

	accounts     map[uuid.UUID]domain.Account
	categories   map[uuid.UUID]domain.Category
	transactions map[uuid.UUID]domain.Transaction
	rules        map[uuid.UUID]domain.RecurringRule
	budgets      map[uuid.UUID]domain.Budget
	goals        map[uuid.UUID]domain.Goal

	// FailWith, when set, is returned by every method. It stands in for a
	// database that is down.
	FailWith error
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		accounts:     make(map[uuid.UUID]domain.Account),
		categories:   make(map[uuid.UUID]domain.Category),
		transactions: make(map[uuid.UUID]domain.Transaction),
		rules:        make(map[uuid.UUID]domain.RecurringRule),
		budgets:      make(map[uuid.UUID]domain.Budget),
		goals:        make(map[uuid.UUID]domain.Goal),
	}
}

// The six ports are exposed through views rather than by *Store itself: their
// method names collide (every repository has a Get and an Add), so one type
// cannot satisfy them all.
var (
	_ application.AccountRepository     = accountView{}
	_ application.CategoryRepository    = categoryView{}
	_ application.TransactionRepository = transactionView{}
	_ application.RecurringRepository   = recurringView{}
	_ application.BudgetRepository      = budgetView{}
	_ application.GoalRepository        = goalView{}
)

// Accounts is the store's application.AccountRepository view.
func (s *Store) Accounts() application.AccountRepository { return accountView{s} }

// Categories is the store's application.CategoryRepository view.
func (s *Store) Categories() application.CategoryRepository { return categoryView{s} }

// Transactions is the store's application.TransactionRepository view.
func (s *Store) Transactions() application.TransactionRepository { return transactionView{s} }

// Rules is the store's application.RecurringRepository view.
func (s *Store) Rules() application.RecurringRepository { return recurringView{s} }

// Budgets is the store's application.BudgetRepository view.
func (s *Store) Budgets() application.BudgetRepository { return budgetView{s} }

// Goals is the store's application.GoalRepository view.
func (s *Store) Goals() application.GoalRepository { return goalView{s} }

// columnScale snaps a value to numeric(18,2), which is what every money column
// in db/migrations does on write. Reproducing it here keeps a service test's
// figures identical to what the same code returns from Postgres: an amount
// posted as 2000 comes back as 2000.00.
func columnScale(value domain.Money) domain.Money { return value.Round(domain.MoneyScale) }

// Seeding and inspection, which bypass the ports.

// SeedCategory inserts a category directly.
func (s *Store) SeedCategory(category domain.Category) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.categories[category.Id] = category
}

// SeedRule inserts a recurring rule directly.
func (s *Store) SeedRule(rule domain.RecurringRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[rule.Id] = rule
}

// SeedBudget inserts a budget directly.
func (s *Store) SeedBudget(budget domain.Budget) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.budgets[budget.Id] = budget
}

// StoredTransaction returns a stored transaction as the test sees it after the
// service ran.
func (s *Store) StoredTransaction(id uuid.UUID) (domain.Transaction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	transaction, ok := s.transactions[id]
	return transaction, ok
}

// StoredRule returns a stored recurring rule.
func (s *Store) StoredRule(id uuid.UUID) (domain.RecurringRule, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.rules[id]
	return rule, ok
}

// StoredAccount returns a stored account.
func (s *Store) StoredAccount(id uuid.UUID) (domain.Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[id]
	return account, ok
}

// StoredGoal returns a stored goal.
func (s *Store) StoredGoal(id uuid.UUID) (domain.Goal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	goal, ok := s.goals[id]
	return goal, ok
}

// CountTransactions reports how many transactions are stored.
func (s *Store) CountTransactions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.transactions)
}

// CountBudgets reports how many budgets are stored.
func (s *Store) CountBudgets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.budgets)
}

// CountCategories reports how many categories are stored.
func (s *Store) CountCategories() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.categories)
}

// AccountRepository

// ListAccounts returns the user's accounts, active first then by name.
func (s *Store) ListAccounts(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	return s.accountsOf(userID)
}

func (s *Store) accountsOf(userID uuid.UUID) ([]domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	accounts := make([]domain.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if account.UserId == userID {
			accounts = append(accounts, account)
		}
	}

	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].IsArchived != accounts[j].IsArchived {
			return !accounts[i].IsArchived
		}
		return accounts[i].Name < accounts[j].Name
	})
	return accounts, nil
}

// GetAccount reads one account owned by the user.
func (s *Store) GetAccount(ctx context.Context, id, userID uuid.UUID) (*domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	account, ok := s.accounts[id]
	if !ok || account.UserId != userID {
		return nil, application.ErrRowNotFound
	}
	return &account, nil
}

// ExistsAccount is the ownership probe.
func (s *Store) ExistsAccount(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return false, s.FailWith
	}

	account, ok := s.accounts[id]
	return ok && account.UserId == userID, nil
}

// AddAccount inserts an account.
func (s *Store) AddAccount(ctx context.Context, account *domain.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	stored := *account
	stored.InitialBalance = columnScale(stored.InitialBalance)
	s.accounts[account.Id] = stored
	return nil
}

// UpdateAccount writes an account the user owns.
func (s *Store) UpdateAccount(ctx context.Context, account *domain.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	stored, ok := s.accounts[account.Id]
	if !ok || stored.UserId != account.UserId {
		return application.ErrRowNotFound
	}

	written := *account
	written.InitialBalance = columnScale(written.InitialBalance)
	s.accounts[account.Id] = written
	return nil
}

// CategoryRepository

// ListVisible returns the defaults plus the user's own, by type then name.
func (s *Store) ListVisible(ctx context.Context, userID uuid.UUID) ([]domain.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	categories := make([]domain.Category, 0, len(s.categories))
	for _, category := range s.categories {
		if visibleTo(category, userID) {
			categories = append(categories, category)
		}
	}

	sort.SliceStable(categories, func(i, j int) bool {
		if categories[i].Type != categories[j].Type {
			return categories[i].Type < categories[j].Type
		}
		return categories[i].Name < categories[j].Name
	})
	return categories, nil
}

// FindVisible reads one visible category.
func (s *Store) FindVisible(ctx context.Context, id, userID uuid.UUID) (*domain.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	category, ok := s.categories[id]
	if !ok || !visibleTo(category, userID) {
		return nil, application.ErrRowNotFound
	}
	return &category, nil
}

// AddCategory inserts a category.
func (s *Store) AddCategory(ctx context.Context, category *domain.Category) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	s.categories[category.Id] = *category
	return nil
}

// UpdateCategory writes a category the user owns.
func (s *Store) UpdateCategory(ctx context.Context, category *domain.Category) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	stored, ok := s.categories[category.Id]
	if !ok || stored.UserId == nil || category.UserId == nil || *stored.UserId != *category.UserId {
		return application.ErrRowNotFound
	}
	s.categories[category.Id] = *category
	return nil
}

// DeleteCategory removes a category, detaching transactions and dropping the
// user's budgets for it.
func (s *Store) DeleteCategory(ctx context.Context, id, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	category, ok := s.categories[id]
	if !ok || category.UserId == nil || *category.UserId != userID {
		return application.ErrRowNotFound
	}

	for key, transaction := range s.transactions {
		if transaction.UserId == userID && transaction.CategoryId != nil && *transaction.CategoryId == id {
			transaction.CategoryId = nil
			s.transactions[key] = transaction
		}
	}
	for key, budget := range s.budgets {
		if budget.UserId == userID && budget.CategoryId == id {
			delete(s.budgets, key)
		}
	}

	delete(s.categories, id)
	return nil
}

func visibleTo(category domain.Category, userID uuid.UUID) bool {
	return category.IsDefault || (category.UserId != nil && *category.UserId == userID)
}

// TransactionRepository

// SearchTransactions filters, orders and pages as the SQL repository does.
func (s *Store) SearchTransactions(
	ctx context.Context,
	userID uuid.UUID,
	filter application.TransactionFilter,
) ([]domain.Transaction, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, 0, s.FailWith
	}

	matches := make([]domain.Transaction, 0, len(s.transactions))
	for _, transaction := range s.transactions {
		if transaction.UserId == userID && matchesFilter(transaction, filter) {
			matches = append(matches, transaction)
		}
	}
	sortNewestFirst(matches)

	total := len(matches)
	if filter.Offset >= total {
		return []domain.Transaction{}, total, nil
	}

	end := min(filter.Offset+filter.Limit, total)
	return matches[filter.Offset:end], total, nil
}

// GetTransaction reads one transaction owned by the user.
func (s *Store) GetTransaction(ctx context.Context, id, userID uuid.UUID) (*domain.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	transaction, ok := s.transactions[id]
	if !ok || transaction.UserId != userID {
		return nil, application.ErrRowNotFound
	}
	return &transaction, nil
}

// ListRange returns an inclusive window, newest first.
func (s *Store) ListRange(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]domain.Transaction, error) {
	transactions, err := s.window(userID, from, to)
	if err != nil {
		return nil, err
	}
	sortNewestFirst(transactions)
	return transactions, nil
}

// Slices projects an inclusive window, oldest first.
func (s *Store) Slices(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]application.TransactionSlice, error) {
	transactions, err := s.window(userID, from, to)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(transactions, func(i, j int) bool {
		if !transactions[i].Date.Equal(transactions[j].Date) {
			return transactions[i].Date.Before(transactions[j].Date)
		}
		return bytes.Compare(transactions[i].Id[:], transactions[j].Id[:]) < 0
	})

	slices := make([]application.TransactionSlice, 0, len(transactions))
	for _, transaction := range transactions {
		slices = append(slices, application.TransactionSlice{
			AccountID:         transaction.AccountId,
			TransferAccountID: transaction.TransferAccountId,
			CategoryID:        transaction.CategoryId,
			Type:              transaction.Type,
			Amount:            transaction.Amount,
			Date:              transaction.Date,
		})
	}
	return slices, nil
}

// AddTransaction inserts one transaction.
func (s *Store) AddTransaction(ctx context.Context, transaction *domain.Transaction) error {
	return s.AddMany(ctx, []domain.Transaction{*transaction})
}

// AddMany inserts a batch.
func (s *Store) AddMany(ctx context.Context, transactions []domain.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	for _, transaction := range transactions {
		transaction.Amount = columnScale(transaction.Amount)
		s.transactions[transaction.Id] = transaction
	}
	return nil
}

// UpdateTransaction writes a transaction the user owns.
func (s *Store) UpdateTransaction(ctx context.Context, transaction *domain.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	stored, ok := s.transactions[transaction.Id]
	if !ok || stored.UserId != transaction.UserId {
		return application.ErrRowNotFound
	}

	written := *transaction
	written.Amount = columnScale(written.Amount)
	s.transactions[transaction.Id] = written
	return nil
}

// DeleteTransaction removes a transaction the user owns.
func (s *Store) DeleteTransaction(ctx context.Context, id, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	transaction, ok := s.transactions[id]
	if !ok || transaction.UserId != userID {
		return application.ErrRowNotFound
	}
	delete(s.transactions, id)
	return nil
}

func (s *Store) window(userID uuid.UUID, from, to *time.Time) ([]domain.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	transactions := make([]domain.Transaction, 0, len(s.transactions))
	for _, transaction := range s.transactions {
		if transaction.UserId != userID {
			continue
		}
		if from != nil && transaction.Date.Before(*from) {
			continue
		}
		if to != nil && transaction.Date.After(*to) {
			continue
		}
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}

func matchesFilter(transaction domain.Transaction, filter application.TransactionFilter) bool {
	if filter.AccountID != nil {
		onEitherSide := transaction.AccountId == *filter.AccountID ||
			(transaction.TransferAccountId != nil && *transaction.TransferAccountId == *filter.AccountID)
		if !onEitherSide {
			return false
		}
	}
	if filter.CategoryID != nil &&
		(transaction.CategoryId == nil || *transaction.CategoryId != *filter.CategoryID) {
		return false
	}
	if filter.Type != nil && transaction.Type != *filter.Type {
		return false
	}
	if filter.From != nil && transaction.Date.Before(*filter.From) {
		return false
	}
	if filter.To != nil && transaction.Date.After(*filter.To) {
		return false
	}
	if filter.Search != "" {
		notes := ""
		if transaction.Notes != nil {
			notes = *transaction.Notes
		}
		found := strings.Contains(strings.ToLower(transaction.Description), filter.Search) ||
			strings.Contains(strings.ToLower(notes), filter.Search) ||
			strings.Contains(strings.ToLower(transaction.TagsRaw), filter.Search)
		if !found {
			return false
		}
	}
	return true
}

func sortNewestFirst(transactions []domain.Transaction) {
	sort.SliceStable(transactions, func(i, j int) bool {
		if !transactions[i].Date.Equal(transactions[j].Date) {
			return transactions[i].Date.After(transactions[j].Date)
		}
		return bytes.Compare(transactions[i].Id[:], transactions[j].Id[:]) > 0
	})
}

// RecurringRepository

// ListRules returns the user's rules, soonest run first.
func (s *Store) ListRules(ctx context.Context, userID uuid.UUID) ([]domain.RecurringRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	rules := make([]domain.RecurringRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.UserId == userID {
			rules = append(rules, rule)
		}
	}
	sortRules(rules)
	return rules, nil
}

// GetRule reads one rule owned by the user.
func (s *Store) GetRule(ctx context.Context, id, userID uuid.UUID) (*domain.RecurringRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	rule, ok := s.rules[id]
	if !ok || rule.UserId != userID {
		return nil, application.ErrRowNotFound
	}
	return &rule, nil
}

// ListDue returns every active rule of every user due at or before the cutoff.
func (s *Store) ListDue(ctx context.Context, cutoff time.Time) ([]domain.RecurringRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	rules := make([]domain.RecurringRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.IsActive && !rule.NextRunDate.After(cutoff) {
			rules = append(rules, rule)
		}
	}
	sortRules(rules)
	return rules, nil
}

// AddRule inserts a rule.
func (s *Store) AddRule(ctx context.Context, rule *domain.RecurringRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	written := *rule
	written.Amount = columnScale(written.Amount)
	s.rules[rule.Id] = written
	return nil
}

// UpdateRule writes a rule the user owns.
func (s *Store) UpdateRule(ctx context.Context, rule *domain.RecurringRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	stored, ok := s.rules[rule.Id]
	if !ok || stored.UserId != rule.UserId {
		return application.ErrRowNotFound
	}

	written := *rule
	written.Amount = columnScale(written.Amount)
	s.rules[rule.Id] = written
	return nil
}

// DeleteRule removes a rule the user owns.
func (s *Store) DeleteRule(ctx context.Context, id, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	rule, ok := s.rules[id]
	if !ok || rule.UserId != userID {
		return application.ErrRowNotFound
	}
	delete(s.rules, id)
	return nil
}

func sortRules(rules []domain.RecurringRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		if !rules[i].NextRunDate.Equal(rules[j].NextRunDate) {
			return rules[i].NextRunDate.Before(rules[j].NextRunDate)
		}
		return bytes.Compare(rules[i].Id[:], rules[j].Id[:]) < 0
	})
}

// BudgetRepository

// ListForMonth returns the user's budgets for one month key, unordered: the
// service is what puts them in order.
func (s *Store) ListForMonth(
	ctx context.Context,
	userID uuid.UUID,
	month string,
) ([]domain.Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	budgets := make([]domain.Budget, 0, len(s.budgets))
	for _, budget := range s.budgets {
		if budget.UserId == userID && budget.Month == month {
			budgets = append(budgets, budget)
		}
	}
	return budgets, nil
}

// GetBudget reads one budget owned by the user.
func (s *Store) GetBudget(ctx context.Context, id, userID uuid.UUID) (*domain.Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	budget, ok := s.budgets[id]
	if !ok || budget.UserId != userID {
		return nil, application.ErrRowNotFound
	}
	return &budget, nil
}

// ExistsBudget reports whether that category is already budgeted that month.
func (s *Store) ExistsBudget(
	ctx context.Context,
	userID, categoryID uuid.UUID,
	month string,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return false, s.FailWith
	}

	for _, budget := range s.budgets {
		if budget.UserId == userID && budget.CategoryId == categoryID && budget.Month == month {
			return true, nil
		}
	}
	return false, nil
}

// AddBudget inserts a budget.
func (s *Store) AddBudget(ctx context.Context, budget *domain.Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	written := *budget
	written.Limit = columnScale(written.Limit)
	s.budgets[budget.Id] = written
	return nil
}

// UpdateBudget writes a budget the user owns.
func (s *Store) UpdateBudget(ctx context.Context, budget *domain.Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	stored, ok := s.budgets[budget.Id]
	if !ok || stored.UserId != budget.UserId {
		return application.ErrRowNotFound
	}

	written := *budget
	written.Limit = columnScale(written.Limit)
	s.budgets[budget.Id] = written
	return nil
}

// DeleteBudget removes a budget the user owns.
func (s *Store) DeleteBudget(ctx context.Context, id, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	budget, ok := s.budgets[id]
	if !ok || budget.UserId != userID {
		return application.ErrRowNotFound
	}
	delete(s.budgets, id)
	return nil
}

// GoalRepository

// ListGoals returns the user's goals by name.
func (s *Store) ListGoals(ctx context.Context, userID uuid.UUID) ([]domain.Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	goals := make([]domain.Goal, 0, len(s.goals))
	for _, goal := range s.goals {
		if goal.UserId == userID {
			goals = append(goals, goal)
		}
	}

	sort.SliceStable(goals, func(i, j int) bool { return goals[i].Name < goals[j].Name })
	return goals, nil
}

// GetGoal reads one goal owned by the user.
func (s *Store) GetGoal(ctx context.Context, id, userID uuid.UUID) (*domain.Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return nil, s.FailWith
	}

	goal, ok := s.goals[id]
	if !ok || goal.UserId != userID {
		return nil, application.ErrRowNotFound
	}
	return &goal, nil
}

// AddGoal inserts a goal.
func (s *Store) AddGoal(ctx context.Context, goal *domain.Goal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	written := *goal
	written.TargetAmount = columnScale(written.TargetAmount)
	written.CurrentAmount = columnScale(written.CurrentAmount)
	s.goals[goal.Id] = written
	return nil
}

// UpdateGoal writes a goal the user owns.
func (s *Store) UpdateGoal(ctx context.Context, goal *domain.Goal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	stored, ok := s.goals[goal.Id]
	if !ok || stored.UserId != goal.UserId {
		return application.ErrRowNotFound
	}

	written := *goal
	written.TargetAmount = columnScale(written.TargetAmount)
	written.CurrentAmount = columnScale(written.CurrentAmount)
	s.goals[goal.Id] = written
	return nil
}

// DeleteGoal removes a goal the user owns.
func (s *Store) DeleteGoal(ctx context.Context, id, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWith != nil {
		return s.FailWith
	}

	goal, ok := s.goals[id]
	if !ok || goal.UserId != userID {
		return application.ErrRowNotFound
	}
	delete(s.goals, id)
	return nil
}

// Views

// accountView adapts the store to application.AccountRepository.
type accountView struct{ store *Store }

func (v accountView) List(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	return v.store.ListAccounts(ctx, userID)
}

func (v accountView) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Account, error) {
	return v.store.GetAccount(ctx, id, userID)
}

func (v accountView) Exists(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	return v.store.ExistsAccount(ctx, id, userID)
}

func (v accountView) Add(ctx context.Context, account *domain.Account) error {
	return v.store.AddAccount(ctx, account)
}

func (v accountView) Update(ctx context.Context, account *domain.Account) error {
	return v.store.UpdateAccount(ctx, account)
}

// categoryView adapts the store to application.CategoryRepository.
type categoryView struct{ store *Store }

func (v categoryView) ListVisible(ctx context.Context, userID uuid.UUID) ([]domain.Category, error) {
	return v.store.ListVisible(ctx, userID)
}

func (v categoryView) FindVisible(ctx context.Context, id, userID uuid.UUID) (*domain.Category, error) {
	return v.store.FindVisible(ctx, id, userID)
}

func (v categoryView) Add(ctx context.Context, category *domain.Category) error {
	return v.store.AddCategory(ctx, category)
}

func (v categoryView) Update(ctx context.Context, category *domain.Category) error {
	return v.store.UpdateCategory(ctx, category)
}

func (v categoryView) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return v.store.DeleteCategory(ctx, id, userID)
}

// transactionView adapts the store to application.TransactionRepository.
type transactionView struct{ store *Store }

func (v transactionView) Search(
	ctx context.Context,
	userID uuid.UUID,
	filter application.TransactionFilter,
) ([]domain.Transaction, int, error) {
	return v.store.SearchTransactions(ctx, userID, filter)
}

func (v transactionView) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Transaction, error) {
	return v.store.GetTransaction(ctx, id, userID)
}

func (v transactionView) ListRange(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]domain.Transaction, error) {
	return v.store.ListRange(ctx, userID, from, to)
}

func (v transactionView) Slices(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]application.TransactionSlice, error) {
	return v.store.Slices(ctx, userID, from, to)
}

func (v transactionView) Add(ctx context.Context, transaction *domain.Transaction) error {
	return v.store.AddTransaction(ctx, transaction)
}

func (v transactionView) AddMany(ctx context.Context, transactions []domain.Transaction) error {
	return v.store.AddMany(ctx, transactions)
}

func (v transactionView) Update(ctx context.Context, transaction *domain.Transaction) error {
	return v.store.UpdateTransaction(ctx, transaction)
}

func (v transactionView) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return v.store.DeleteTransaction(ctx, id, userID)
}

// recurringView adapts the store to application.RecurringRepository.
type recurringView struct{ store *Store }

func (v recurringView) List(ctx context.Context, userID uuid.UUID) ([]domain.RecurringRule, error) {
	return v.store.ListRules(ctx, userID)
}

func (v recurringView) Get(ctx context.Context, id, userID uuid.UUID) (*domain.RecurringRule, error) {
	return v.store.GetRule(ctx, id, userID)
}

func (v recurringView) ListDue(ctx context.Context, cutoff time.Time) ([]domain.RecurringRule, error) {
	return v.store.ListDue(ctx, cutoff)
}

func (v recurringView) Add(ctx context.Context, rule *domain.RecurringRule) error {
	return v.store.AddRule(ctx, rule)
}

func (v recurringView) Update(ctx context.Context, rule *domain.RecurringRule) error {
	return v.store.UpdateRule(ctx, rule)
}

func (v recurringView) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return v.store.DeleteRule(ctx, id, userID)
}

// budgetView adapts the store to application.BudgetRepository.
type budgetView struct{ store *Store }

func (v budgetView) ListForMonth(
	ctx context.Context,
	userID uuid.UUID,
	month string,
) ([]domain.Budget, error) {
	return v.store.ListForMonth(ctx, userID, month)
}

func (v budgetView) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Budget, error) {
	return v.store.GetBudget(ctx, id, userID)
}

func (v budgetView) Exists(
	ctx context.Context,
	userID, categoryID uuid.UUID,
	month string,
) (bool, error) {
	return v.store.ExistsBudget(ctx, userID, categoryID, month)
}

func (v budgetView) Add(ctx context.Context, budget *domain.Budget) error {
	return v.store.AddBudget(ctx, budget)
}

func (v budgetView) Update(ctx context.Context, budget *domain.Budget) error {
	return v.store.UpdateBudget(ctx, budget)
}

func (v budgetView) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return v.store.DeleteBudget(ctx, id, userID)
}

// goalView adapts the store to application.GoalRepository.
type goalView struct{ store *Store }

func (v goalView) List(ctx context.Context, userID uuid.UUID) ([]domain.Goal, error) {
	return v.store.ListGoals(ctx, userID)
}

func (v goalView) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Goal, error) {
	return v.store.GetGoal(ctx, id, userID)
}

func (v goalView) Add(ctx context.Context, goal *domain.Goal) error {
	return v.store.AddGoal(ctx, goal)
}

func (v goalView) Update(ctx context.Context, goal *domain.Goal) error {
	return v.store.UpdateGoal(ctx, goal)
}

func (v goalView) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return v.store.DeleteGoal(ctx, id, userID)
}
