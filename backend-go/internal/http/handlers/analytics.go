package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// The window defaults the .NET controller declares on its parameters.
const (
	defaultNetWorthMonths = 12
	defaultCashflowMonths = 6
)

// Analytics serves /api/dashboard and /api/reports, both of which the .NET API
// backs with the one AnalyticsService.
type Analytics struct {
	service *application.AnalyticsService

	now func() time.Time
}

// NewAnalytics builds the handler around an AnalyticsService.
func NewAnalytics(service *application.AnalyticsService) *Analytics {
	return &Analytics{service: service, now: func() time.Time { return time.Now().UTC() }}
}

// WithClock returns a copy that resolves the default report year from now.
func (h *Analytics) WithClock(now func() time.Time) *Analytics {
	clone := *h
	clone.now = now
	return &clone
}

// Routes registers the endpoints of DashboardController and ReportsController.
func (h *Analytics) Routes(api *gin.RouterGroup) {
	dashboard := api.Group("/dashboard")
	dashboard.GET("/summary", h.Summary)
	dashboard.GET("/networth", h.NetWorth)
	dashboard.GET("/cashflow", h.Cashflow)
	dashboard.GET("/spending", h.Spending)

	reports := api.Group("/reports")
	reports.GET("/monthly", h.Monthly)
	reports.GET("/categories", h.Categories)
}

// Summary handles GET /api/dashboard/summary.
func (h *Analytics) Summary(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	summary, err := h.service.Summary(c.Request.Context(), userID)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// NetWorth handles GET /api/dashboard/networth.
func (h *Analytics) NetWorth(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	reader := newQueryReader(c)
	months := reader.numberOr("months", "months", defaultNetWorthMonths)
	if !reader.ok() {
		return
	}

	points, err := h.service.NetWorth(c.Request.Context(), userID, months)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, points)
}

// Cashflow handles GET /api/dashboard/cashflow.
func (h *Analytics) Cashflow(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	reader := newQueryReader(c)
	months := reader.numberOr("months", "months", defaultCashflowMonths)
	if !reader.ok() {
		return
	}

	points, err := h.service.Cashflow(c.Request.Context(), userID, months)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, points)
}

// Spending handles GET /api/dashboard/spending.
func (h *Analytics) Spending(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	month := newQueryReader(c).text("month")

	slices, err := h.service.Spending(c.Request.Context(), userID, month)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, slices)
}

// Monthly handles GET /api/reports/monthly, defaulting to the current year.
func (h *Analytics) Monthly(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	reader := newQueryReader(c)
	year := reader.numberOr("year", "year", h.now().UTC().Year())
	if !reader.ok() {
		return
	}

	months, err := h.service.MonthlyReport(c.Request.Context(), userID, year)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, months)
}

// Categories handles GET /api/reports/categories.
func (h *Analytics) Categories(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	reader := newQueryReader(c)
	from := reader.moment("from", "from")
	to := reader.moment("to", "to")
	if !reader.ok() {
		return
	}

	report, err := h.service.CategoryReport(c.Request.Context(), userID, from, to)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}
