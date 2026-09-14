package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/config"
	"billing-app/backend/internal/database"
	"billing-app/backend/internal/repository"
	"billing-app/backend/internal/router"
	"billing-app/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

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

	engine := router.New(router.Dependencies{
		Logger:       logger,
		DB:           db,
		Customers:    service.NewCustomerService(customerRepo),
		Products:     service.NewProductService(productRepo),
		Invoices:     service.NewInvoiceService(invoiceRepo, customerRepo, productRepo),
		InvoiceItems: service.NewInvoiceItemService(invoiceItemRepo, invoiceRepo, productRepo),
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
