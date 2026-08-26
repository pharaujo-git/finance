// Package httpapi builds the gin engine: middleware, the unauthenticated
// probes, and the authenticated /api group later phases hang controllers on.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// RegisterFunc adds routes to the authenticated /api group. Later phases pass
// one per controller; tests pass a stub.
type RegisterFunc func(api *gin.RouterGroup)

// serviceDocument is the body GET / returns, mirroring the anonymous object
// the .NET API maps on the same route.
type serviceDocument struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Docs    string `json:"docs"`
}

// Options carries everything the router needs from the outside.
type Options struct {
	Tokens         application.TokenService
	AllowedOrigins []string
	// Register runs against the /api group after RequireAuth is attached.
	Register []RegisterFunc
	// RegisterAnonymous runs against an /api group with no auth middleware. It
	// exists for the endpoints that hand out tokens (register, login), which
	// carry [AllowAnonymous] in the .NET API.
	RegisterAnonymous []RegisterFunc
}

// New builds the engine. gin.New is used rather than gin.Default so the
// logger is opt-in; Recovery is attached explicitly.
func New(opts Options) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), cors(opts.AllowedOrigins))

	engine.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// A struct rather than gin.H so the members keep the .NET API's order. The
	// service name differs from the .NET API's on purpose: it is the one
	// response that tells an operator which backend answered.
	engine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, serviceDocument{
			Service: "FinanceTracker API (Go)",
			Status:  "ok",
			Docs:    "/swagger",
		})
	})

	// Two groups share the /api prefix. Middleware is per-group in gin, so the
	// anonymous one is the only way to expose a route under /api without
	// RequireAuth; the routes themselves never collide.
	anonymous := engine.Group("/api")
	for _, register := range opts.RegisterAnonymous {
		if register != nil {
			register(anonymous)
		}
	}

	api := engine.Group("/api")
	api.Use(middleware.RequireAuth(opts.Tokens))
	for _, register := range opts.Register {
		if register != nil {
			register(api)
		}
	}

	return engine
}

// cors answers preflights and echoes an exact-matched origin. It mirrors the
// .NET policy: WithOrigins(list).AllowAnyHeader().AllowAnyMethod(), with no
// credentials support.
func cors(allowed []string) gin.HandlerFunc {
	permitted := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		permitted[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := permitted[origin]; ok && origin != "" {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Add("Vary", "Origin")

			if c.Request.Method == http.MethodOptions {
				header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				if requested := c.GetHeader("Access-Control-Request-Headers"); requested != "" {
					header.Set("Access-Control-Allow-Headers", requested)
				} else {
					header.Set("Access-Control-Allow-Headers", "*")
				}
				header.Set("Access-Control-Max-Age", "86400")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
