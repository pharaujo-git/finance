package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// yearRangeMessage is the 400 an out-of-range report year raises.
const yearRangeMessage = "Year must be between 1900 and 9999."

// The bounds the analytics endpoints clamp their inputs to.
const (
	minReportYear  = 1900
	maxReportYear  = 9999
	minWindowMonth = 1
	maxWindowMonth = 120
	// savingsRateScale is the number of decimals the summary rounds the rate to.
	savingsRateScale int32 = 4
)

// AnalyticsService is the aggregation behind the dashboard and report
// endpoints. Every figure is derived from the same slice projection, so the
// numbers cannot drift apart.
type AnalyticsService struct {
	transactions *TransactionService
	accounts     AccountRepository
	categories   *CategoryService

	now func() time.Time
}

// NewAnalyticsService wires the service to its ports.
func NewAnalyticsService(
	transactions *TransactionService,
	accounts AccountRepository,
	categories *CategoryService,
) *AnalyticsService {
	return &AnalyticsService{
		transactions: transactions,
		accounts:     accounts,
		categories:   categories,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// WithClock returns a copy that resolves "now" from the given clock.
func (s *AnalyticsService) WithClock(now func() time.Time) *AnalyticsService {
	clone := *s
	clone.now = now
	return &clone
}

// Summary is GET /api/dashboard/summary: net worth over all time, income and
// expenses inside the current month, and the resulting savings rate.
func (s *AnalyticsService) Summary(ctx context.Context, userID uuid.UUID) (DashboardSummaryDto, error) {
	slices, err := s.transactions.Slices(ctx, userID, nil, nil)
	if err != nil {
		return DashboardSummaryDto{}, err
	}
	opening, err := s.openingBalance(ctx, userID)
	if err != nil {
		return DashboardSummaryDto{}, err
	}

	monthStart := StartOfMonth(s.now())
	monthEnd := domain.AddMonths(monthStart, 1)

	thisMonth := make([]TransactionSlice, 0, len(slices))
	for _, slice := range slices {
		if !slice.Date.Before(monthStart) && slice.Date.Before(monthEnd) {
			thisMonth = append(thisMonth, slice)
		}
	}

	income := totalOf(thisMonth, domain.TransactionIncome)
	expenses := totalOf(thisMonth, domain.TransactionExpense)

	netWorth := opening
	for _, slice := range slices {
		netWorth = netWorth.Add(NetWorthDelta(slice))
	}

	savingsRate := domain.Zero()
	if income.IsPositive() {
		savingsRate = domain.TrimMoney(
			income.Sub(expenses).Div(income).Round(savingsRateScale))
	}

	return DashboardSummaryDto{
		NetWorth:      domain.NewAmount(netWorth),
		TotalIncome:   domain.NewAmount(income),
		TotalExpenses: domain.NewAmount(expenses),
		SavingsRate:   domain.NewAmount(savingsRate),
	}, nil
}

// NetWorth is GET /api/dashboard/networth: the running net worth at the end of
// each of the trailing months.
func (s *AnalyticsService) NetWorth(
	ctx context.Context,
	userID uuid.UUID,
	months int,
) ([]NetWorthPointDto, error) {
	window := TrailingMonths(s.now(), clampMonths(months))

	slices, err := s.transactions.Slices(ctx, userID, nil, nil)
	if err != nil {
		return nil, err
	}
	opening, err := s.openingBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	points := make([]NetWorthPointDto, 0, len(window))
	for _, start := range window {
		end := domain.AddMonths(start, 1)
		value := opening
		for _, slice := range slices {
			if slice.Date.Before(end) {
				value = value.Add(NetWorthDelta(slice))
			}
		}
		points = append(points, NetWorthPointDto{Month: MonthFrom(start), Value: domain.NewAmount(value)})
	}
	return points, nil
}

// Cashflow is GET /api/dashboard/cashflow: income and expenses per month.
func (s *AnalyticsService) Cashflow(
	ctx context.Context,
	userID uuid.UUID,
	months int,
) ([]CashflowPointDto, error) {
	window := TrailingMonths(s.now(), clampMonths(months))

	slices, err := s.transactions.Slices(ctx, userID, &window[0], nil)
	if err != nil {
		return nil, err
	}

	points := make([]CashflowPointDto, 0, len(window))
	for _, start := range window {
		month, income, expenses := bucket(slices, start)
		points = append(points, CashflowPointDto{
			Month:    month,
			Income:   domain.NewAmount(income),
			Expenses: domain.NewAmount(expenses),
		})
	}
	return points, nil
}

// Spending is GET /api/dashboard/spending: one month's expenses per category,
// largest first.
func (s *AnalyticsService) Spending(
	ctx context.Context,
	userID uuid.UUID,
	month *string,
) ([]SpendingSliceDto, error) {
	start := StartOfMonth(s.now())
	if month != nil {
		parsed, err := ParseMonth(*month)
		if err != nil {
			return nil, err
		}
		start = parsed
	}
	end := domain.AddMonths(start, 1)

	slices, err := s.transactions.Slices(ctx, userID, &start, nil)
	if err != nil {
		return nil, err
	}
	lookup, err := s.categories.lookup(ctx, userID)
	if err != nil {
		return nil, err
	}

	groups := groupByCategory(slices, func(slice TransactionSlice) bool {
		return slice.Type == domain.TransactionExpense && slice.Date.Before(end)
	})

	result := make([]SpendingSliceDto, 0, len(groups))
	for _, group := range groups {
		info := describe(lookup, group.CategoryID)
		result = append(result, SpendingSliceDto{
			CategoryID:   group.CategoryID,
			CategoryName: info.Name,
			Color:        info.Color,
			Amount:       domain.NewAmount(group.Total),
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Amount.Money().GreaterThan(result[j].Amount.Money())
	})
	return result, nil
}

// MonthlyReport is GET /api/reports/monthly: the twelve months of one year.
func (s *AnalyticsService) MonthlyReport(
	ctx context.Context,
	userID uuid.UUID,
	year int,
) ([]MonthlyReportDto, error) {
	if year < minReportYear || year > maxReportYear {
		return nil, domain.BadRequest(yearRangeMessage)
	}

	start := FirstDayUTC(year, time.January)
	// The .NET window ends one tick (100ns) before the next year begins, and
	// both bounds are inclusive.
	end := domain.AddYears(start, 1).Add(-100 * time.Nanosecond)

	slices, err := s.transactions.Slices(ctx, userID, &start, &end)
	if err != nil {
		return nil, err
	}

	months := make([]MonthlyReportDto, 0, 12)
	for offset := range 12 {
		month, income, expenses := bucket(slices, domain.AddMonths(start, offset))
		months = append(months, MonthlyReportDto{
			Month:    month,
			Income:   domain.NewAmount(income),
			Expenses: domain.NewAmount(expenses),
			Net:      domain.NewAmount(income.Sub(expenses)),
		})
	}
	return months, nil
}

// CategoryReport is GET /api/reports/categories: totals per category over an
// arbitrary window, transfers excluded.
func (s *AnalyticsService) CategoryReport(
	ctx context.Context,
	userID uuid.UUID,
	from, to *time.Time,
) ([]CategoryReportDto, error) {
	slices, err := s.transactions.Slices(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	lookup, err := s.categories.lookup(ctx, userID)
	if err != nil {
		return nil, err
	}

	groups := groupByCategory(slices, func(slice TransactionSlice) bool {
		return slice.Type != domain.TransactionTransfer
	})

	result := make([]CategoryReportDto, 0, len(groups))
	for _, group := range groups {
		info := describe(lookup, group.CategoryID)
		// An uncategorized group that contains any income is reported as income;
		// otherwise the category's own type wins.
		categoryType := info.Type
		if group.CategoryID == nil && group.HasIncome {
			categoryType = domain.CategoryIncome
		}

		result = append(result, CategoryReportDto{
			CategoryID:   group.CategoryID,
			CategoryName: info.Name,
			Type:         categoryType,
			Color:        info.Color,
			Amount:       domain.NewAmount(group.Total),
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Amount.Money().GreaterThan(result[j].Amount.Money())
	})
	return result, nil
}

// openingBalance is the sum of every account's opening balance, which is where
// net worth starts before any transaction moves money.
func (s *AnalyticsService) openingBalance(ctx context.Context, userID uuid.UUID) (domain.Money, error) {
	accounts, err := s.accounts.List(ctx, userID)
	if err != nil {
		return domain.Zero(), fmt.Errorf("application: listing accounts: %w", err)
	}

	total := domain.Zero()
	for _, account := range accounts {
		total = total.Add(account.InitialBalance)
	}
	return total, nil
}

// categoryGroup is one bucket of the per-category aggregations.
type categoryGroup struct {
	CategoryID *uuid.UUID
	Total      domain.Money
	HasIncome  bool
}

// groupByCategory groups the kept slices by category, preserving the order the
// keys were first seen, which is what LINQ's GroupBy does.
func groupByCategory(slices []TransactionSlice, keep func(TransactionSlice) bool) []categoryGroup {
	groups := make([]categoryGroup, 0, 8)
	indexes := make(map[uuid.UUID]int, 8)
	uncategorized := -1

	for _, slice := range slices {
		if !keep(slice) {
			continue
		}

		index := uncategorized
		if slice.CategoryID != nil {
			existing, ok := indexes[*slice.CategoryID]
			if !ok {
				existing = -1
			}
			index = existing
		}

		if index < 0 {
			groups = append(groups, categoryGroup{CategoryID: slice.CategoryID, Total: domain.Zero()})
			index = len(groups) - 1
			if slice.CategoryID == nil {
				uncategorized = index
			} else {
				indexes[*slice.CategoryID] = index
			}
		}

		groups[index].Total = groups[index].Total.Add(slice.Amount)
		groups[index].HasIncome = groups[index].HasIncome || slice.Type == domain.TransactionIncome
	}
	return groups
}

// bucket totals one month of slices, half-open on the far end.
func bucket(slices []TransactionSlice, start time.Time) (string, domain.Money, domain.Money) {
	end := domain.AddMonths(start, 1)

	inMonth := make([]TransactionSlice, 0, len(slices))
	for _, slice := range slices {
		if !slice.Date.Before(start) && slice.Date.Before(end) {
			inMonth = append(inMonth, slice)
		}
	}

	return MonthFrom(start),
		totalOf(inMonth, domain.TransactionIncome),
		totalOf(inMonth, domain.TransactionExpense)
}

func totalOf(slices []TransactionSlice, transactionType domain.TransactionType) domain.Money {
	total := domain.Zero()
	for _, slice := range slices {
		if slice.Type == transactionType {
			total = total.Add(slice.Amount)
		}
	}
	return total
}

func clampMonths(months int) int {
	if months < minWindowMonth {
		return minWindowMonth
	}
	if months > maxWindowMonth {
		return maxWindowMonth
	}
	return months
}
