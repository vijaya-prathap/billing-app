-- Idempotent: safe to run on an empty database, a legacy database, or an already-migrated one.

CREATE TABLE IF NOT EXISTS customers (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name       VARCHAR(255)    NOT NULL,
    email      VARCHAR(255)    NOT NULL,
    phone      VARCHAR(50)     NOT NULL DEFAULT '',
    address    VARCHAR(1000)   NOT NULL DEFAULT '',
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_customers_email (email)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- The steps below upgrade the original customers table (INT id, nullable created_at,
-- no phone/address/updated_at, non-unique email) in place without losing rows.
-- MySQL has no ADD COLUMN IF NOT EXISTS, so each change is guarded via information_schema.

UPDATE customers SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;

SET @ddl := IF(
    (SELECT COLUMN_TYPE FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'customers' AND COLUMN_NAME = 'id') <> 'bigint unsigned',
    'ALTER TABLE customers
        MODIFY id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
        MODIFY name       VARCHAR(255)    NOT NULL,
        MODIFY email      VARCHAR(255)    NOT NULL,
        MODIFY created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP',
    'DO 0'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := IF(
    NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'customers' AND COLUMN_NAME = 'phone'),
    'ALTER TABLE customers ADD COLUMN phone VARCHAR(50) NOT NULL DEFAULT '''' AFTER email',
    'DO 0'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := IF(
    NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'customers' AND COLUMN_NAME = 'address'),
    'ALTER TABLE customers ADD COLUMN address VARCHAR(1000) NOT NULL DEFAULT '''' AFTER phone',
    'DO 0'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := IF(
    NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS
                 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'customers' AND COLUMN_NAME = 'updated_at'),
    'ALTER TABLE customers ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER created_at',
    'DO 0'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := IF(
    NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS
                 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'customers' AND INDEX_NAME = 'uq_customers_email'),
    'ALTER TABLE customers ADD UNIQUE KEY uq_customers_email (email)',
    'DO 0'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
