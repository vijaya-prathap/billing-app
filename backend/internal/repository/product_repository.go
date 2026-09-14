package repository

import (
	"context"
	"database/sql"

	"billing-app/backend/internal/models"
)

const productColumns = "id, name, description, sku, price, stock, created_at, updated_at"

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, p *models.Product) (*models.Product, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO products (name, description, sku, price, stock) VALUES (?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.SKU, p.Price, p.Stock,
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

func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*models.Product, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+productColumns+` FROM products WHERE id = ?`, id,
	)
	return scanProduct(row)
}

func (r *ProductRepository) List(ctx context.Context, p models.Pagination) ([]models.Product, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+productColumns+` FROM products ORDER BY id DESC LIMIT ? OFFSET ?`,
		p.Limit, p.Offset(),
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	products := make([]models.Product, 0, p.Limit)
	for rows.Next() {
		prod, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, *prod)
	}
	return products, total, rows.Err()
}

func (r *ProductRepository) Update(ctx context.Context, p *models.Product) (*models.Product, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE products SET name = ?, description = ?, sku = ?, price = ?, stock = ? WHERE id = ?`,
		p.Name, p.Description, p.SKU, p.Price, p.Stock, p.ID,
	)
	if err != nil {
		return nil, translateError(err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, p.ID)
}

func (r *ProductRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM products WHERE id = ?`, id)
	if err != nil {
		return translateError(err)
	}
	return requireRowsAffected(res)
}

func scanProduct(s rowScanner) (*models.Product, error) {
	var p models.Product
	if err := s.Scan(&p.ID, &p.Name, &p.Description, &p.SKU, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, translateError(err)
	}
	return &p, nil
}
