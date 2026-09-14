package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"billing-app/backend/internal/models"
)

const invoiceColumns = "id, customer_id, invoice_number, status, issue_date, due_date, subtotal, tax_rate, tax_amount, total, notes, created_at, updated_at"

type InvoiceRepository struct {
	db *sql.DB
}

func NewInvoiceRepository(db *sql.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(ctx context.Context, inv *models.Invoice) (*models.Invoice, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO invoices (customer_id, invoice_number, status, issue_date, due_date, subtotal, tax_rate, tax_amount, total, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.CustomerID, inv.InvoiceNumber, inv.Status, inv.IssueDate, inv.DueDate,
		inv.Subtotal, inv.TaxRate, inv.TaxAmount, inv.Total, inv.Notes,
	)
	if err != nil {
		return nil, translateError(err)
	}

	invoiceID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	for _, item := range inv.Items {
		if _, err := tx.ExecContext(ctx, insertInvoiceItemQuery,
			invoiceID, item.ProductID, item.Description, item.Quantity, item.UnitPrice, item.LineTotal,
		); err != nil {
			return nil, translateError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, invoiceID)
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id int64) (*models.Invoice, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+invoiceColumns+` FROM invoices WHERE id = ?`, id,
	)
	inv, err := scanInvoice(row)
	if err != nil {
		return nil, err
	}

	items, err := listInvoiceItems(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	inv.Items = items
	return inv, nil
}

func (r *InvoiceRepository) List(ctx context.Context, f models.InvoiceFilter) ([]models.Invoice, int64, error) {
	var where []string
	var args []any

	if f.CustomerID > 0 {
		where = append(where, "customer_id = ?")
		args = append(args, f.CustomerID)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}

	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices`+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(args, f.Limit, f.Offset())
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+invoiceColumns+` FROM invoices`+clause+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	invoices := make([]models.Invoice, 0, f.Limit)
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, err
		}
		invoices = append(invoices, *inv)
	}
	return invoices, total, rows.Err()
}

// Update applies only while the invoice is still in expectedStatus, so two
// concurrent status changes cannot both succeed.
func (r *InvoiceRepository) Update(ctx context.Context, inv *models.Invoice, expectedStatus models.InvoiceStatus) (*models.Invoice, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE invoices SET status = ?, due_date = ?, notes = ? WHERE id = ? AND status = ?`,
		inv.Status, inv.DueDate, inv.Notes, inv.ID, expectedStatus,
	)
	if err != nil {
		return nil, translateError(err)
	}
	if err := requireRowsAffected(res); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrStateChanged
		}
		return nil, err
	}
	return r.GetByID(ctx, inv.ID)
}

func (r *InvoiceRepository) Delete(ctx context.Context, id int64, allowedStatuses ...models.InvoiceStatus) error {
	query := `DELETE FROM invoices WHERE id = ?`
	args := []any{id}
	if len(allowedStatuses) > 0 {
		query += ` AND status IN (?` + strings.Repeat(", ?", len(allowedStatuses)-1) + `)`
		for _, s := range allowedStatuses {
			args = append(args, s)
		}
	}

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return translateError(err)
	}
	if err := requireRowsAffected(res); err != nil {
		if errors.Is(err, ErrNotFound) && len(allowedStatuses) > 0 {
			return ErrStateChanged
		}
		return err
	}
	return nil
}

func scanInvoice(s rowScanner) (*models.Invoice, error) {
	var inv models.Invoice
	if err := s.Scan(
		&inv.ID, &inv.CustomerID, &inv.InvoiceNumber, &inv.Status, &inv.IssueDate, &inv.DueDate,
		&inv.Subtotal, &inv.TaxRate, &inv.TaxAmount, &inv.Total, &inv.Notes, &inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		return nil, translateError(err)
	}
	return &inv, nil
}
