package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// Budgets serves /api/budgets.
type Budgets struct {
	service *application.BudgetService
}

// NewBudgets builds the handler around a BudgetService.
func NewBudgets(service *application.BudgetService) *Budgets {
	return &Budgets{service: service}
}

// Routes registers the endpoints of BudgetsController.
func (h *Budgets) Routes(api *gin.RouterGroup) {
	budgets := api.Group("/budgets")
	budgets.GET("", h.List)
	budgets.POST("", h.Create)
	budgets.PUT("/:id", h.Update)
	budgets.DELETE("/:id", h.Delete)
}

// List handles GET /api/budgets, defaulting to the current month.
func (h *Budgets) List(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	// The month is passed through as text: an absent key means "this month",
	// and anything else is the service's to validate.
	month := newQueryReader(c).text("month")

	budgets, err := h.service.List(c.Request.Context(), userID, month)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, budgets)
}

// Create handles POST /api/budgets.
func (h *Budgets) Create(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	var request application.CreateBudgetRequest
	if !bindJSON(c, &request) {
		return
	}

	budget, err := h.service.Create(c.Request.Context(), userID, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, budget)
}

// Update handles PUT /api/budgets/{id}.
func (h *Budgets) Update(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	var request application.UpdateBudgetRequest
	if !bindJSON(c, &request) {
		return
	}

	budget, err := h.service.Update(c.Request.Context(), userID, id, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, budget)
}

// Delete handles DELETE /api/budgets/{id}.
func (h *Budgets) Delete(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
