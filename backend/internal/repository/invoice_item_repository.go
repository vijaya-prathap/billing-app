package repository

import (
	"context"
	"database/sql"

	"billing-app/backend/internal/models"
)

const (
	invoiceItemColumns = "id, invoice_id, product_id, description, quantity, unit_price, line_total, created_at"

	insertInvoiceItemQuery = `INSERT INTO invoice_items (invoice_id, product_id, description, quantity, unit_price, line_total)
		VALUES (?, ?, ?, ?, ?, ?)`

	// MySQL evaluates single-table SET assignments left to right, so tax_amount and
	// total see the freshly computed subtotal within this one statement.
	recalculateInvoiceTotalsQuery = `UPDATE invoices SET
		subtotal   = (SELECT COALESCE(SUM(line_total), 0) FROM invoice_items WHERE invoice_id = ?),
		tax_amount = ROUND(subtotal * tax_rate / 100, 2),
		total      = subtotal + tax_amount
		WHERE id = ?`
)

type InvoiceItemRepository struct {
	db *sql.DB
}

func NewInvoiceItemRepository(db *sql.DB) *InvoiceItemRepository {
	return &InvoiceItemRepository{db: db}
}

func (r *InvoiceItemRepository) Create(ctx context.Context, item *models.InvoiceItem) (*models.InvoiceItem, error) {
	var id int64
	err := r.withDraftInvoice(ctx, item.InvoiceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, insertInvoiceItemQuery,
			item.InvoiceID, item.ProductID, item.Description, item.Quantity, item.UnitPrice, item.LineTotal,
		)
		if err != nil {
			return translateError(err)
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, item.InvoiceID, id)
}

func (r *InvoiceItemRepository) GetByID(ctx context.Context, invoiceID, itemID int64) (*models.InvoiceItem, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+invoiceItemColumns+` FROM invoice_items WHERE id = ? AND invoice_id = ?`,
		itemID, invoiceID,
	)
	return scanInvoiceItem(row)
}

func (r *InvoiceItemRepository) Update(ctx context.Context, item *models.InvoiceItem) (*models.InvoiceItem, error) {
	err := r.withDraftInvoice(ctx, item.InvoiceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE invoice_items SET description = ?, quantity = ?, unit_price = ?, line_total = ?
			 WHERE id = ? AND invoice_id = ?`,
			item.Description, item.Quantity, item.UnitPrice, item.LineTotal, item.ID, item.InvoiceID,
		)
		if err != nil {
			return translateError(err)
		}
		return requireRowsAffected(res)
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, item.InvoiceID, item.ID)
}

func (r *InvoiceItemRepository) Delete(ctx context.Context, invoiceID, itemID int64) error {
	return r.withDraftInvoice(ctx, invoiceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM invoice_items WHERE id = ? AND invoice_id = ?`, itemID, invoiceID,
		)
		if err != nil {
			return translateError(err)
		}
		return requireRowsAffected(res)
	})
}

// withDraftInvoice row-locks the parent invoice so its status cannot change while
// items are modified, then recalculates the invoice totals in the same transaction.
func (r *InvoiceItemRepository) withDraftInvoice(ctx context.Context, invoiceID int64, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status models.InvoiceStatus
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM invoices WHERE id = ? FOR UPDATE`, invoiceID,
	).Scan(&status); err != nil {
		return translateError(err)
	}
	if status != models.InvoiceStatusDraft {
		return ErrInvoiceNotEditable
	}

	if err := fn(tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, recalculateInvoiceTotalsQuery, invoiceID, invoiceID); err != nil {
		return err
	}
	return tx.Commit()
}

func listInvoiceItems(ctx context.Context, q queryer, invoiceID int64) ([]models.InvoiceItem, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+invoiceItemColumns+` FROM invoice_items WHERE invoice_id = ? ORDER BY id`, invoiceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.InvoiceItem, 0)
	for rows.Next() {
		item, err := scanInvoiceItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanInvoiceItem(s rowScanner) (*models.InvoiceItem, error) {
	var item models.InvoiceItem
	if err := s.Scan(
		&item.ID, &item.InvoiceID, &item.ProductID, &item.Description,
		&item.Quantity, &item.UnitPrice, &item.LineTotal, &item.CreatedAt,
	); err != nil {
		return nil, translateError(err)
	}
	return &item, nil
}
