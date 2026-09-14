package service

import (
	"context"
	"errors"
	"strings"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/repository"
)

type ProductRepository interface {
	Create(ctx context.Context, p *models.Product) (*models.Product, error)
	GetByID(ctx context.Context, id int64) (*models.Product, error)
	List(ctx context.Context, p models.Pagination) ([]models.Product, int64, error)
	Update(ctx context.Context, p *models.Product) (*models.Product, error)
	Delete(ctx context.Context, id int64) error
}

type ProductService struct {
	repo ProductRepository
}

func NewProductService(repo ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Create(ctx context.Context, in models.ProductInput) (*models.Product, error) {
	p, err := productFromInput(in)
	if err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, p)
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, conflictError("a product with SKU %q already exists", p.SKU)
	}
	return created, err
}

func (s *ProductService) Get(ctx context.Context, id int64) (*models.Product, error) {
	p, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, notFoundError("product %d not found", id)
	}
	return p, err
}

func (s *ProductService) List(ctx context.Context, p models.Pagination) (*models.PaginatedResponse[models.Product], error) {
	products, total, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, err
	}
	return models.NewPaginatedResponse(products, p, total), nil
}

func (s *ProductService) Update(ctx context.Context, id int64, in models.ProductInput) (*models.Product, error) {
	p, err := productFromInput(in)
	if err != nil {
		return nil, err
	}
	p.ID = id

	updated, err := s.repo.Update(ctx, p)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil, notFoundError("product %d not found", id)
	case errors.Is(err, repository.ErrDuplicate):
		return nil, conflictError("a product with SKU %q already exists", p.SKU)
	}
	return updated, err
}

func (s *ProductService) Delete(ctx context.Context, id int64) error {
	err := s.repo.Delete(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return notFoundError("product %d not found", id)
	case errors.Is(err, repository.ErrForeignKeyViolation):
		return conflictError("product %d is used on invoices and cannot be deleted", id)
	}
	return err
}

func productFromInput(in models.ProductInput) (*models.Product, error) {
	p := &models.Product{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		SKU:         strings.ToUpper(strings.TrimSpace(in.SKU)),
		Price:       round2(in.Price),
		Stock:       in.Stock,
	}
	if len(p.Name) < 2 {
		return nil, validationError("name must contain at least 2 non-space characters")
	}
	if p.SKU == "" {
		return nil, validationError("sku must not be blank")
	}
	if p.Price <= 0 {
		return nil, validationError("price must be at least 0.01")
	}
	return p, nil
}
