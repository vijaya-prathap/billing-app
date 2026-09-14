package repository

import (
	"context"
	"database/sql"

	"billing-app/backend/internal/models"
)

const customerColumns = "id, name, email, phone, address, created_at, updated_at"

type CustomerRepository struct {
	db *sql.DB
}

func NewCustomerRepository(db *sql.DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) Create(ctx context.Context, c *models.Customer) (*models.Customer, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO customers (name, email, phone, address) VALUES (?, ?, ?, ?)`,
		c.Name, c.Email, c.Phone, c.Address,
	)
	if err != nil {
		return nil, translateError(err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *CustomerRepository) GetByID(ctx context.Context, id int64) (*models.Customer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+customerColumns+` FROM customers WHERE id = ?`, id,
	)
	return scanCustomer(row)
}

func (r *CustomerRepository) List(ctx context.Context, p models.Pagination) ([]models.Customer, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM customers`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+customerColumns+` FROM customers ORDER BY id DESC LIMIT ? OFFSET ?`,
		p.Limit, p.Offset(),
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	customers := make([]models.Customer, 0, p.Limit)
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, 0, err
		}
		customers = append(customers, *c)
	}
	return customers, total, rows.Err()
}

func (r *CustomerRepository) Update(ctx context.Context, c *models.Customer) (*models.Customer, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE customers SET name = ?, email = ?, phone = ?, address = ? WHERE id = ?`,
		c.Name, c.Email, c.Phone, c.Address, c.ID,
	)
	if err != nil {
		return nil, translateError(err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, c.ID)
}

func (r *CustomerRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM customers WHERE id = ?`, id)
	if err != nil {
		return translateError(err)
	}
	return requireRowsAffected(res)
}

func scanCustomer(s rowScanner) (*models.Customer, error) {
	var c models.Customer
	if err := s.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Address, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, translateError(err)
	}
	return &c, nil
}
