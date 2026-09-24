package models

import "time"

// AuthorizedUser is a row of authorized_users: a Google account that may use the app.
type AuthorizedUser struct {
	ID          int64      `json:"id"`
	GoogleSub   string     `json:"google_sub,omitempty"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
