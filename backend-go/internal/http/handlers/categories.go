package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// Categories serves /api/categories.
type Categories struct {
	service *application.CategoryService
}

// NewCategories builds the handler around a CategoryService.
func NewCategories(service *application.CategoryService) *Categories {
	return &Categories{service: service}
}

// Routes registers the endpoints of CategoriesController.
func (h *Categories) Routes(api *gin.RouterGroup) {
	categories := api.Group("/categories")
	categories.GET("", h.List)
	categories.POST("", h.Create)
	categories.PUT("/:id", h.Update)
	categories.DELETE("/:id", h.Delete)
}

// List handles GET /api/categories: the shared defaults plus the caller's own.
func (h *Categories) List(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	categories, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, categories)
}

// Create handles POST /api/categories.
func (h *Categories) Create(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	var request application.CategoryRequest
	if !bindJSON(c, &request) {
		return
	}

	category, err := h.service.Create(c.Request.Context(), userID, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, category)
}

// Update handles PUT /api/categories/{id}.
func (h *Categories) Update(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	var request application.CategoryRequest
	if !bindJSON(c, &request) {
		return
	}

	category, err := h.service.Update(c.Request.Context(), userID, id, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, category)
}

// Delete handles DELETE /api/categories/{id}.
func (h *Categories) Delete(c *gin.Context) {
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
