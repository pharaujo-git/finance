package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// Recurring serves /api/recurring.
type Recurring struct {
	service *application.RecurringService
}

// NewRecurring builds the handler around a RecurringService.
func NewRecurring(service *application.RecurringService) *Recurring {
	return &Recurring{service: service}
}

// Routes registers the endpoints of RecurringController.
func (h *Recurring) Routes(api *gin.RouterGroup) {
	recurring := api.Group("/recurring")
	recurring.GET("", h.List)
	recurring.POST("", h.Create)
	recurring.PUT("/:id", h.Update)
	recurring.DELETE("/:id", h.Delete)
}

// List handles GET /api/recurring.
func (h *Recurring) List(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	rules, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, rules)
}

// Create handles POST /api/recurring.
func (h *Recurring) Create(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	var request application.RecurringRuleRequest
	if !bindJSON(c, &request) {
		return
	}

	rule, err := h.service.Create(c.Request.Context(), userID, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

// Update handles PUT /api/recurring/{id}.
func (h *Recurring) Update(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	var request application.RecurringRuleRequest
	if !bindJSON(c, &request) {
		return
	}

	rule, err := h.service.Update(c.Request.Context(), userID, id, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

// Delete handles DELETE /api/recurring/{id}.
func (h *Recurring) Delete(c *gin.Context) {
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
