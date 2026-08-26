package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// Goals serves /api/goals.
type Goals struct {
	service *application.GoalService
}

// NewGoals builds the handler around a GoalService.
func NewGoals(service *application.GoalService) *Goals {
	return &Goals{service: service}
}

// Routes registers the endpoints of GoalsController.
func (h *Goals) Routes(api *gin.RouterGroup) {
	goals := api.Group("/goals")
	goals.GET("", h.List)
	goals.POST("", h.Create)
	goals.PUT("/:id", h.Update)
	goals.DELETE("/:id", h.Delete)
	goals.POST("/:id/contribute", h.Contribute)
}

// List handles GET /api/goals.
func (h *Goals) List(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	goals, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, goals)
}

// Create handles POST /api/goals.
func (h *Goals) Create(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	var request application.GoalRequest
	if !bindJSON(c, &request) {
		return
	}

	goal, err := h.service.Create(c.Request.Context(), userID, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, goal)
}

// Update handles PUT /api/goals/{id}.
func (h *Goals) Update(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	var request application.GoalRequest
	if !bindJSON(c, &request) {
		return
	}

	goal, err := h.service.Update(c.Request.Context(), userID, id, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, goal)
}

// Delete handles DELETE /api/goals/{id}.
func (h *Goals) Delete(c *gin.Context) {
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

// Contribute handles POST /api/goals/{id}/contribute.
func (h *Goals) Contribute(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	var request application.ContributeRequest
	if !bindJSON(c, &request) {
		return
	}

	goal, err := h.service.Contribute(c.Request.Context(), userID, id, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, goal)
}
