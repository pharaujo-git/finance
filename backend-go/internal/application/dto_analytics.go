package application

import (
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// DashboardSummaryDto is the body of GET /api/dashboard/summary.
type DashboardSummaryDto struct {
	NetWorth      domain.Amount `json:"netWorth"`
	TotalIncome   domain.Amount `json:"totalIncome"`
	TotalExpenses domain.Amount `json:"totalExpenses"`
	SavingsRate   domain.Amount `json:"savingsRate"`
}

// NetWorthPointDto is one month of GET /api/dashboard/networth.
type NetWorthPointDto struct {
	Month string        `json:"month"`
	Value domain.Amount `json:"value"`
}

// CashflowPointDto is one month of GET /api/dashboard/cashflow.
type CashflowPointDto struct {
	Month    string        `json:"month"`
	Income   domain.Amount `json:"income"`
	Expenses domain.Amount `json:"expenses"`
}

// SpendingSliceDto is one category's share of GET /api/dashboard/spending.
type SpendingSliceDto struct {
	CategoryID   *uuid.UUID    `json:"categoryId"`
	CategoryName string        `json:"categoryName"`
	Color        string        `json:"color"`
	Amount       domain.Amount `json:"amount"`
}

// MonthlyReportDto is one month of GET /api/reports/monthly.
type MonthlyReportDto struct {
	Month    string        `json:"month"`
	Income   domain.Amount `json:"income"`
	Expenses domain.Amount `json:"expenses"`
	Net      domain.Amount `json:"net"`
}

// CategoryReportDto is one category's total in GET /api/reports/categories.
type CategoryReportDto struct {
	CategoryID   *uuid.UUID          `json:"categoryId"`
	CategoryName string              `json:"categoryName"`
	Type         domain.CategoryType `json:"type"`
	Color        string              `json:"color"`
	Amount       domain.Amount       `json:"amount"`
}
