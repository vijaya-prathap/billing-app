CREATE TABLE IF NOT EXISTS invoice_items (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    invoice_id  BIGINT UNSIGNED NOT NULL,
    product_id  BIGINT UNSIGNED NOT NULL,
    description VARCHAR(500)    NOT NULL DEFAULT '',
    quantity    INT             NOT NULL,
    unit_price  DECIMAL(12, 2)  NOT NULL,
    line_total  DECIMAL(12, 2)  NOT NULL,
    created_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    CONSTRAINT fk_invoice_items_invoice
        FOREIGN KEY (invoice_id) REFERENCES invoices (id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_invoice_items_product
        FOREIGN KEY (product_id) REFERENCES products (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT chk_invoice_items_quantity CHECK (quantity > 0),
    CONSTRAINT chk_invoice_items_unit_price CHECK (unit_price >= 0)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
