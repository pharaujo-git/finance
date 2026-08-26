// Command api runs the Go edition of the FinanceTracker API. It is deployable
// beside the .NET API: same database, same JWT secret, same password hashes.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	httpapi "github.com/pharaujo/finance/backend-go/internal/http"
	"github.com/pharaujo/finance/backend-go/internal/http/handlers"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/jobs"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/persistence"
)

const (
	readHeaderTimeout = 10 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownGrace     = 15 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := persistence.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := persistence.Ping(ctx, pool); err != nil {
		return err
	}

	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	tokens := identity.NewTokenService(cfg.JWTSecret)
	auth := handlers.NewAuth(application.NewAuthService(
		persistence.NewUserRepository(pool),
		tokens,
		identity.NewPasswordHasher(),
	))

	// One repository set per table, all bound to the pool. The recurring worker
	// builds a second set of its own, bound to the transaction that holds the
	// pass lock.
	accountRepo := persistence.NewAccountRepository(pool)
	categoryRepo := persistence.NewCategoryRepository(pool)
	transactionRepo := persistence.NewTransactionRepository(pool)
	recurringRepo := persistence.NewRecurringRepository(pool)
	budgetRepo := persistence.NewBudgetRepository(pool)
	goalRepo := persistence.NewGoalRepository(pool)

	categories := application.NewCategoryService(categoryRepo)
	transactions := application.NewTransactionService(transactionRepo, accountRepo, categories)

	accountsHandler := handlers.NewAccounts(application.NewAccountService(accountRepo, transactionRepo))
	categoriesHandler := handlers.NewCategories(categories)
	transactionsHandler := handlers.NewTransactions(
		transactions,
		application.NewTransactionCsvService(transactionRepo, accountRepo, categoryRepo),
	)
	recurringHandler := handlers.NewRecurring(
		application.NewRecurringService(recurringRepo, transactionRepo, accountRepo, categories))
	budgetsHandler := handlers.NewBudgets(
		application.NewBudgetService(budgetRepo, transactionRepo, categories))
	goalsHandler := handlers.NewGoals(application.NewGoalService(goalRepo))
	analyticsHandler := handlers.NewAnalytics(
		application.NewAnalyticsService(transactions, accountRepo, categories))

	engine := httpapi.New(httpapi.Options{
		Tokens:            tokens,
		AllowedOrigins:    cfg.AllowedOrigins,
		RegisterAnonymous: []httpapi.RegisterFunc{auth.AnonymousRoutes},
		Register: []httpapi.RegisterFunc{
			auth.AuthenticatedRoutes,
			accountsHandler.Routes,
			categoriesHandler.Routes,
			transactionsHandler.Routes,
			recurringHandler.Routes,
			budgetsHandler.Routes,
			goalsHandler.Routes,
			analyticsHandler.Routes,
		},
	})

	// The worker runs for as long as the process does; the signal context stops
	// it at the same moment the HTTP server starts draining.
	worker := jobs.NewRecurringWorker(pool, func(db persistence.Querier) jobs.Materializer {
		return application.NewRecurringService(
			persistence.NewRecurringRepository(db),
			persistence.NewTransactionRepository(db),
			persistence.NewAccountRepository(db),
			application.NewCategoryService(persistence.NewCategoryRepository(db)),
		)
	}, logger)

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		worker.Run(ctx)
	}()

	server := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           engine,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("listening",
			slog.Int("port", cfg.Port),
			slog.Any("allowedOrigins", cfg.AllowedOrigins))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- fmt.Errorf("http server: %w", err)
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	// A pass in flight is cancelled with the context, and its transaction rolls
	// back, so the wait here is only for the goroutine to notice.
	select {
	case <-workerDone:
	case <-shutdownCtx.Done():
		logger.Warn("recurring worker did not stop within the shutdown grace period")
	}

	logger.Info("stopped")
	return nil
}
