-- Idempotent: safe to run on an empty database, a legacy database, or an already-migrated one.
--
-- Who may use the application. Google authenticates people; this table decides whether
-- they are authorized. Rows are added by the administrator (`make authorize EMAIL=...`).
-- google_sub is Google's stable account ID and is bound on the account's first login,
-- after which it is the identity used for every check.

CREATE TABLE IF NOT EXISTS authorized_users (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    google_sub    VARCHAR(64)     NULL,
    email         VARCHAR(255)    NOT NULL,
    display_name  VARCHAR(255)    NOT NULL DEFAULT '',
    is_active     TINYINT(1)      NOT NULL DEFAULT 1,
    last_login_at TIMESTAMP       NULL,
    created_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_authorized_users_google_sub (google_sub),
    UNIQUE KEY uq_authorized_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
