package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv  string
	AppPort string
	GinMode string
	DB      DBConfig
	Server  ServerConfig
	Auth    AuthConfig
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// AuthConfig controls Google sign-in (OAuth 2.0 authorization code flow).
// Authentication is on unless AUTH_DISABLED=true, so a production deployment
// cannot silently run without it. Who may sign in lives in the authorized_users
// table, not in configuration.
type AuthConfig struct {
	Disabled           bool
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	SessionSecret      string
	SessionTTL         time.Duration
	CookieSecure       bool
}

func (a AuthConfig) Enabled() bool {
	return !a.Disabled
}

type ServerConfig struct {
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", "development")
	cfg := &Config{
		AppEnv:  appEnv,
		AppPort: getEnv("APP_PORT", "8080"),
		GinMode: getEnv("GIN_MODE", "debug"),
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "3306"),
			User:            os.Getenv("DB_USER"),
			Password:        os.Getenv("DB_PASSWORD"),
			Name:            os.Getenv("DB_NAME"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Server: ServerConfig{
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Auth: AuthConfig{
			Disabled:           getEnvBool("AUTH_DISABLED", false),
			GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
			SessionSecret:      os.Getenv("SESSION_SECRET"),
			SessionTTL:         getEnvDuration("SESSION_TTL", 12*time.Hour),
			// Browsers only send Secure cookies over HTTPS, so default it off for plain-http development.
			CookieSecure: getEnvBool("AUTH_COOKIE_SECURE", appEnv == "production"),
		},
	}

	if cfg.DB.User == "" || cfg.DB.Name == "" {
		return nil, fmt.Errorf("DB_USER and DB_NAME environment variables are required")
	}
	if cfg.Auth.Enabled() {
		if err := cfg.Auth.validate(); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func (a AuthConfig) validate() error {
	const hint = " (set AUTH_DISABLED=true to run without authentication)"
	for name, value := range map[string]string{
		"GOOGLE_CLIENT_ID":     a.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": a.GoogleClientSecret,
		"GOOGLE_REDIRECT_URL":  a.GoogleRedirectURL,
		"SESSION_SECRET":       a.SessionSecret,
	} {
		if value == "" {
			return fmt.Errorf("%s is required for Google sign-in%s", name, hint)
		}
	}
	u, err := url.Parse(a.GoogleRedirectURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("GOOGLE_REDIRECT_URL must be an absolute http(s) URL, e.g. http://localhost:5173/api/v1/auth/google/callback")
	}
	return nil
}

// DSN enables parseTime so DATE/TIMESTAMP scan into time.Time, and clientFoundRows so
// RowsAffected reports matched rows (MySQL otherwise returns 0 for no-op updates).
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC&charset=utf8mb4&clientFoundRows=true",
		d.User, d.Password, d.Host, d.Port, d.Name,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
