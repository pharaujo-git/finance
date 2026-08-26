package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// Accounts serves /api/accounts.
type Accounts struct {
	service *application.AccountService
}

// NewAccounts builds the handler around an AccountService.
func NewAccounts(service *application.AccountService) *Accounts {
	return &Accounts{service: service}
}

// Routes registers the endpoints of AccountsController.
func (h *Accounts) Routes(api *gin.RouterGroup) {
	accounts := api.Group("/accounts")
	accounts.GET("", h.List)
	accounts.POST("", h.Create)
	accounts.GET("/:id", h.Get)
	accounts.PUT("/:id", h.Update)
	accounts.DELETE("/:id", h.Archive)
}

// List handles GET /api/accounts.
func (h *Accounts) List(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	accounts, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, accounts)
}

// Get handles GET /api/accounts/{id}.
func (h *Accounts) Get(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	account, err := h.service.Get(c.Request.Context(), userID, id)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

// Create handles POST /api/accounts. A create answers 200, not 201, which is
// what the .NET controller returns.
func (h *Accounts) Create(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	var request application.CreateAccountRequest
	if !bindJSON(c, &request) {
		return
	}

	account, err := h.service.Create(c.Request.Context(), userID, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

// Update handles PUT /api/accounts/{id}.
func (h *Accounts) Update(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	var request application.UpdateAccountRequest
	if !bindJSON(c, &request) {
		return
	}

	account, err := h.service.Update(c.Request.Context(), userID, id, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

// Archive handles DELETE /api/accounts/{id}, which archives rather than
// deletes and answers 204.
func (h *Accounts) Archive(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	if err := h.service.Archive(c.Request.Context(), userID, id); err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
