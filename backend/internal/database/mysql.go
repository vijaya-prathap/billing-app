package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"billing-app/backend/internal/config"
)

const (
	pingAttempts = 10
	pingTimeout  = 3 * time.Second
)

// NewMySQL retries the initial ping because in Compose/Kubernetes the database
// is frequently still booting when the API container starts.
func NewMySQL(cfg config.DBConfig, log *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	var pingErr error
	for attempt := 1; attempt <= pingAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		pingErr = db.PingContext(ctx)
		cancel()

		if pingErr == nil {
			log.Info("mysql connected", "host", cfg.Host, "database", cfg.Name)
			return db, nil
		}

		log.Warn("mysql not ready, retrying",
			"attempt", attempt,
			"max_attempts", pingAttempts,
			"error", pingErr,
		)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	_ = db.Close()
	return nil, fmt.Errorf("connect to mysql after %d attempts: %w", pingAttempts, pingErr)
}
