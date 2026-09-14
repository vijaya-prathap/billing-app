package service

import (
	"context"
	"errors"
	"strings"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/repository"
)

type CustomerRepository interface {
	Create(ctx context.Context, c *models.Customer) (*models.Customer, error)
	GetByID(ctx context.Context, id int64) (*models.Customer, error)
	List(ctx context.Context, p models.Pagination) ([]models.Customer, int64, error)
	Update(ctx context.Context, c *models.Customer) (*models.Customer, error)
	Delete(ctx context.Context, id int64) error
}

type CustomerService struct {
	repo CustomerRepository
}

func NewCustomerService(repo CustomerRepository) *CustomerService {
	return &CustomerService{repo: repo}
}

func (s *CustomerService) Create(ctx context.Context, in models.CustomerInput) (*models.Customer, error) {
	c, err := customerFromInput(in)
	if err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, c)
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, conflictError("a customer with email %q already exists", c.Email)
	}
	return created, err
}

func (s *CustomerService) Get(ctx context.Context, id int64) (*models.Customer, error) {
	c, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, notFoundError("customer %d not found", id)
	}
	return c, err
}

func (s *CustomerService) List(ctx context.Context, p models.Pagination) (*models.PaginatedResponse[models.Customer], error) {
	customers, total, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, err
	}
	return models.NewPaginatedResponse(customers, p, total), nil
}

func (s *CustomerService) Update(ctx context.Context, id int64, in models.CustomerInput) (*models.Customer, error) {
	c, err := customerFromInput(in)
	if err != nil {
		return nil, err
	}
	c.ID = id

	updated, err := s.repo.Update(ctx, c)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil, notFoundError("customer %d not found", id)
	case errors.Is(err, repository.ErrDuplicate):
		return nil, conflictError("a customer with email %q already exists", c.Email)
	}
	return updated, err
}

func (s *CustomerService) Delete(ctx context.Context, id int64) error {
	err := s.repo.Delete(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return notFoundError("customer %d not found", id)
	case errors.Is(err, repository.ErrForeignKeyViolation):
		return conflictError("customer %d has invoices and cannot be deleted", id)
	}
	return err
}

func customerFromInput(in models.CustomerInput) (*models.Customer, error) {
	c := &models.Customer{
		Name:    strings.TrimSpace(in.Name),
		Email:   strings.ToLower(strings.TrimSpace(in.Email)),
		Phone:   strings.TrimSpace(in.Phone),
		Address: strings.TrimSpace(in.Address),
	}
	if len(c.Name) < 2 {
		return nil, validationError("name must contain at least 2 non-space characters")
	}
	return c, nil
}
