package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/auth"
	"billing-app/backend/internal/config"
	"billing-app/backend/internal/database"
	"billing-app/backend/internal/repository"
	"billing-app/backend/internal/router"
	"billing-app/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	fmt.Println("logger initialized with oauthStateTTL:", logger)

	if err := run(logger); err != nil {
		logger.Error("server exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	gin.SetMode(cfg.GinMode)

	db, err := database.NewMySQL(cfg.DB, logger)
	if err != nil {
		return err
	}
	defer db.Close()

	customerRepo := repository.NewCustomerRepository(db)
	productRepo := repository.NewProductRepository(db)
	invoiceRepo := repository.NewInvoiceRepository(db)
	invoiceItemRepo := repository.NewInvoiceItemRepository(db)
	userRepo := repository.NewAuthorizedUserRepository(db)

	authSvc, err := buildAuth(cfg, userRepo, logger)
	if err != nil {
		return err
	}

	engine := router.New(router.Dependencies{
		Logger:       logger,
		DB:           db,
		Customers:    service.NewCustomerService(customerRepo),
		Products:     service.NewProductService(productRepo),
		Invoices:     service.NewInvoiceService(invoiceRepo, customerRepo, productRepo),
		InvoiceItems: service.NewInvoiceItemService(invoiceItemRepo, invoiceRepo, productRepo),
		Auth:         authSvc,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           engine,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", srv.Addr), slog.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutdown signal received, draining connections", slog.String("timeout", cfg.Server.ShutdownTimeout.String()))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("server stopped cleanly")
	return nil
}

// oauthStateTTL bounds how long a login started at Google may take to come back.
const oauthStateTTL = 10 * time.Minute

// buildAuth wires Google sign-in, or returns nil when AUTH_DISABLED=true.
func buildAuth(cfg *config.Config, users auth.UserStore, logger *slog.Logger) (*auth.Service, error) {
	if !cfg.Auth.Enabled() {
		logger.Warn("authentication is disabled; every /api/v1 route is open (AUTH_DISABLED=true)")
		return nil, nil
	}

	sessions, err := auth.NewSessionManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	if err != nil {
		return nil, fmt.Errorf("SESSION_SECRET: %w", err)
	}
	states, err := auth.NewStateManager(cfg.Auth.SessionSecret, oauthStateTTL)
	if err != nil {
		return nil, fmt.Errorf("SESSION_SECRET: %w", err)
	}
	logger.Info("google sign-in enabled",
		slog.String("redirect_url", cfg.Auth.GoogleRedirectURL),
		slog.String("session_ttl", cfg.Auth.SessionTTL.String()))

	// Google's signing keys are fetched on first use and cached for the process lifetime.
	return &auth.Service{
		OAuth:        auth.NewGoogleOAuth(context.Background(), cfg.Auth.GoogleClientID, cfg.Auth.GoogleClientSecret, cfg.Auth.GoogleRedirectURL),
		Sessions:     sessions,
		States:       states,
		Users:        users,
		CookieSecure: cfg.Auth.CookieSecure,
	}, nil
}
