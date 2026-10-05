package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"billing-app/backend/internal/models"
)

const authorizedUserColumns = "id, google_sub, email, display_name, is_active, last_login_at, created_at, updated_at"

// AuthorizedUserRepository reads the authorized_users table. Lookups return (nil, nil)
// when no row matches, because "not authorized" is an ordinary outcome for sign-in.
type AuthorizedUserRepository struct {
	db *sql.DB
}

func NewAuthorizedUserRepository(db *sql.DB) *AuthorizedUserRepository {
	return &AuthorizedUserRepository{db: db}
}

func (r *AuthorizedUserRepository) FindBySubject(ctx context.Context, sub string) (*models.AuthorizedUser, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+authorizedUserColumns+` FROM authorized_users WHERE google_sub = ?`, sub)
	return scanAuthorizedUser(row)
}

func (r *AuthorizedUserRepository) FindByEmail(ctx context.Context, email string) (*models.AuthorizedUser, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+authorizedUserColumns+` FROM authorized_users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email)))
	return scanAuthorizedUser(row)
}

// BindSubject records Google's account ID against a row the first time it signs in.
func (r *AuthorizedUserRepository) BindSubject(ctx context.Context, id int64, sub string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE authorized_users SET google_sub = ? WHERE id = ? AND google_sub IS NULL`, sub, id)
	if err != nil {
		return translateError(err)
	}
	return requireRowsAffected(res)
}

func (r *AuthorizedUserRepository) TouchLogin(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE authorized_users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return translateError(err)
}

func scanAuthorizedUser(row rowScanner) (*models.AuthorizedUser, error) {
	var (
		u         models.AuthorizedUser
		sub       sql.NullString
		lastLogin sql.NullTime
	)
	err := row.Scan(&u.ID, &sub, &u.Email, &u.DisplayName, &u.IsActive, &lastLogin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.GoogleSub = sub.String
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}
