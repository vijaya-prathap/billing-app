package service

import (
	"context"
	"errors"
	"strings"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/repository"
)

type InvoiceItemRepository interface {
	Create(ctx context.Context, item *models.InvoiceItem) (*models.InvoiceItem, error)
	GetByID(ctx context.Context, invoiceID, itemID int64) (*models.InvoiceItem, error)
	Update(ctx context.Context, item *models.InvoiceItem) (*models.InvoiceItem, error)
	Delete(ctx context.Context, invoiceID, itemID int64) error
}

type InvoiceFinder interface {
	GetByID(ctx context.Context, id int64) (*models.Invoice, error)
}

type InvoiceItemService struct {
	items    InvoiceItemRepository
	invoices InvoiceFinder
	products ProductFinder
}

func NewInvoiceItemService(items InvoiceItemRepository, invoices InvoiceFinder, products ProductFinder) *InvoiceItemService {
	return &InvoiceItemService{items: items, invoices: invoices, products: products}
}

func (s *InvoiceItemService) List(ctx context.Context, invoiceID int64) ([]models.InvoiceItem, error) {
	inv, err := s.invoices.GetByID(ctx, invoiceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, notFoundError("invoice %d not found", invoiceID)
	}
	if err != nil {
		return nil, err
	}
	return inv.Items, nil
}

func (s *InvoiceItemService) Create(ctx context.Context, invoiceID int64, req models.CreateInvoiceItemRequest) (*models.InvoiceItem, error) {
	item, err := buildInvoiceItem(ctx, s.products, req)
	if err != nil {
		return nil, err
	}
	item.InvoiceID = invoiceID

	created, err := s.items.Create(ctx, item)
	if errors.Is(err, repository.ErrForeignKeyViolation) {
		return nil, validationError("product %d does not exist", req.ProductID)
	}
	if err != nil {
		return nil, itemWriteError(err, invoiceID, 0)
	}
	return created, nil
}

func (s *InvoiceItemService) Update(ctx context.Context, invoiceID, itemID int64, req models.UpdateInvoiceItemRequest) (*models.InvoiceItem, error) {
	if req.Quantity <= 0 {
		return nil, validationError("quantity must be greater than 0")
	}
	if req.UnitPrice < 0 {
		return nil, validationError("unit_price must not be negative")
	}

	existing, err := s.items.GetByID(ctx, invoiceID, itemID)
	if err != nil {
		return nil, itemWriteError(err, invoiceID, itemID)
	}

	next := *existing
	if description := strings.TrimSpace(req.Description); description != "" {
		next.Description = description
	}
	next.Quantity = req.Quantity
	next.UnitPrice = round2(req.UnitPrice)
	next.LineTotal = round2(float64(next.Quantity) * next.UnitPrice)

	updated, err := s.items.Update(ctx, &next)
	if err != nil {
		return nil, itemWriteError(err, invoiceID, itemID)
	}
	return updated, nil
}

func (s *InvoiceItemService) Delete(ctx context.Context, invoiceID, itemID int64) error {
	if err := s.items.Delete(ctx, invoiceID, itemID); err != nil {
		return itemWriteError(err, invoiceID, itemID)
	}
	return nil
}

func itemWriteError(err error, invoiceID, itemID int64) error {
	switch {
	case errors.Is(err, repository.ErrInvoiceNotEditable):
		return conflictError("items can only be changed while invoice %d is a draft", invoiceID)
	case errors.Is(err, repository.ErrNotFound) && itemID > 0:
		return notFoundError("item %d not found on invoice %d", itemID, invoiceID)
	case errors.Is(err, repository.ErrNotFound):
		return notFoundError("invoice %d not found", invoiceID)
	}
	return err
}
