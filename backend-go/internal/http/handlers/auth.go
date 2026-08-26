// Package handlers holds the gin handlers. They bind a request, call one
// application service and render the result: every rule, message and status
// code lives in the layers below.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// Auth serves /api/auth.
type Auth struct {
	service *application.AuthService
}

// NewAuth builds the handler around an AuthService.
func NewAuth(service *application.AuthService) *Auth {
	return &Auth{service: service}
}

// AnonymousRoutes registers the two endpoints that hand out tokens. They must
// be mounted on a group without the auth middleware.
func (h *Auth) AnonymousRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
}

// AuthenticatedRoutes registers the profile endpoints, which need a bearer
// token.
func (h *Auth) AuthenticatedRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.GET("/me", h.Profile)
	auth.PUT("/me", h.UpdateProfile)
}

// Register handles POST /api/auth/register.
func (h *Auth) Register(c *gin.Context) {
	var request application.RegisterRequest
	if !bindJSON(c, &request) {
		return
	}

	response, err := h.service.Register(c.Request.Context(), request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// Login handles POST /api/auth/login.
func (h *Auth) Login(c *gin.Context) {
	var request application.LoginRequest
	if !bindJSON(c, &request) {
		return
	}

	response, err := h.service.Login(c.Request.Context(), request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// Profile handles GET /api/auth/me.
func (h *Auth) Profile(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		middleware.WriteAppError(c, domain.Unauthorized("The access token does not identify a user."))
		return
	}

	user, err := h.service.Profile(c.Request.Context(), userID)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateProfile handles PUT /api/auth/me.
func (h *Auth) UpdateProfile(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		middleware.WriteAppError(c, domain.Unauthorized("The access token does not identify a user."))
		return
	}

	var request application.UpdateProfileRequest
	if !bindJSON(c, &request) {
		return
	}

	user, err := h.service.UpdateProfile(c.Request.Context(), userID, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

// bindJSON decodes the body into target, rendering a validation problem when it
// cannot be parsed. The key mirrors the one MVC's JSON reader uses ("$"); the
// message is Go's, since the two parsers phrase their complaints differently.
//
// The request structs carry no `binding` tags on purpose: field rules belong to
// the application layer, so gin's validator has nothing to do here.
func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		problem := domain.NewValidationError()
		problem.Add(domain.JSONBodyField, err.Error())
		middleware.WriteAppError(c, problem)
		return false
	}
	return true
}
