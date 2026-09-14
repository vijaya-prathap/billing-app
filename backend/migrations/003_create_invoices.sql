CREATE TABLE IF NOT EXISTS invoices (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    customer_id    BIGINT UNSIGNED NOT NULL,
    invoice_number VARCHAR(50)     NOT NULL,
    status         VARCHAR(20)     NOT NULL DEFAULT 'draft',
    issue_date     DATE            NOT NULL,
    due_date       DATE            NOT NULL,
    subtotal       DECIMAL(12, 2)  NOT NULL DEFAULT 0,
    tax_rate       DECIMAL(5, 2)   NOT NULL DEFAULT 0,
    tax_amount     DECIMAL(12, 2)  NOT NULL DEFAULT 0,
    total          DECIMAL(12, 2)  NOT NULL DEFAULT 0,
    notes          VARCHAR(2000)   NOT NULL DEFAULT '',
    created_at     TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_invoices_invoice_number (invoice_number),
    KEY idx_invoices_status (status),
    KEY idx_invoices_due_date (due_date),
    CONSTRAINT fk_invoices_customer
        FOREIGN KEY (customer_id) REFERENCES customers (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT chk_invoices_status CHECK (status IN ('draft', 'sent', 'paid', 'cancelled')),
    CONSTRAINT chk_invoices_dates CHECK (due_date >= issue_date),
    CONSTRAINT chk_invoices_tax_rate CHECK (tax_rate BETWEEN 0 AND 100)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
