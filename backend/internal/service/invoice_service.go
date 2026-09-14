package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/repository"
)

const (
	dateLayout             = "2006-01-02"
	invoiceNumberAttempts  = 3
	invoiceNumberRandBytes = 4
)

var deletableInvoiceStatuses = []models.InvoiceStatus{
	models.InvoiceStatusDraft,
	models.InvoiceStatusCancelled,
}

type InvoiceRepository interface {
	Create(ctx context.Context, inv *models.Invoice) (*models.Invoice, error)
	GetByID(ctx context.Context, id int64) (*models.Invoice, error)
	List(ctx context.Context, f models.InvoiceFilter) ([]models.Invoice, int64, error)
	Update(ctx context.Context, inv *models.Invoice, expectedStatus models.InvoiceStatus) (*models.Invoice, error)
	Delete(ctx context.Context, id int64, allowedStatuses ...models.InvoiceStatus) error
}

type CustomerFinder interface {
	GetByID(ctx context.Context, id int64) (*models.Customer, error)
}

type ProductFinder interface {
	GetByID(ctx context.Context, id int64) (*models.Product, error)
}

type InvoiceService struct {
	invoices  InvoiceRepository
	customers CustomerFinder
	products  ProductFinder
	now       func() time.Time
}

func NewInvoiceService(invoices InvoiceRepository, customers CustomerFinder, products ProductFinder) *InvoiceService {
	return &InvoiceService{
		invoices:  invoices,
		customers: customers,
		products:  products,
		now:       time.Now,
	}
}

func (s *InvoiceService) Create(ctx context.Context, req models.CreateInvoiceRequest) (*models.Invoice, error) {
	issueDate, err := parseDate("issue_date", req.IssueDate)
	if err != nil {
		return nil, err
	}
	dueDate, err := parseDate("due_date", req.DueDate)
	if err != nil {
		return nil, err
	}
	if dueDate.Before(issueDate) {
		return nil, validationError("due_date must be on or after issue_date")
	}
	if len(req.Items) == 0 {
		return nil, validationError("an invoice requires at least one item")
	}

	if _, err := s.customers.GetByID(ctx, req.CustomerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, validationError("customer %d does not exist", req.CustomerID)
		}
		return nil, err
	}

	inv := &models.Invoice{
		CustomerID: req.CustomerID,
		Status:     models.InvoiceStatusDraft,
		IssueDate:  issueDate,
		DueDate:    dueDate,
		TaxRate:    round2(req.TaxRate),
		Notes:      strings.TrimSpace(req.Notes),
		Items:      make([]models.InvoiceItem, 0, len(req.Items)),
	}

	var subtotal float64
	for _, itemReq := range req.Items {
		item, err := buildInvoiceItem(ctx, s.products, itemReq)
		if err != nil {
			return nil, err
		}
		subtotal += item.LineTotal
		inv.Items = append(inv.Items, *item)
	}
	inv.Subtotal = round2(subtotal)
	inv.TaxAmount = round2(inv.Subtotal * inv.TaxRate / 100)
	inv.Total = round2(inv.Subtotal + inv.TaxAmount)

	for attempt := 1; ; attempt++ {
		inv.InvoiceNumber, err = s.newInvoiceNumber()
		if err != nil {
			return nil, err
		}

		created, err := s.invoices.Create(ctx, inv)
		switch {
		case err == nil:
			return created, nil
		case errors.Is(err, repository.ErrDuplicate) && attempt < invoiceNumberAttempts:
			continue
		case errors.Is(err, repository.ErrForeignKeyViolation):
			return nil, validationError("customer or product was removed while creating the invoice")
		default:
			return nil, err
		}
	}
}

func (s *InvoiceService) Get(ctx context.Context, id int64) (*models.Invoice, error) {
	inv, err := s.invoices.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, notFoundError("invoice %d not found", id)
	}
	return inv, err
}

func (s *InvoiceService) List(ctx context.Context, f models.InvoiceFilter) (*models.PaginatedResponse[models.Invoice], error) {
	if f.Status != "" && !f.Status.Valid() {
		return nil, validationError("status must be one of: draft, sent, paid, cancelled")
	}

	invoices, total, err := s.invoices.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return models.NewPaginatedResponse(invoices, f.Pagination, total), nil
}

func (s *InvoiceService) Update(ctx context.Context, id int64, req models.UpdateInvoiceRequest) (*models.Invoice, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if existing.Status.IsTerminal() {
		return nil, conflictError("invoice %d is %s and can no longer be modified", id, existing.Status)
	}
	if !existing.Status.CanTransitionTo(req.Status) {
		return nil, conflictError("invoice status cannot change from %s to %s", existing.Status, req.Status)
	}

	dueDate, err := parseDate("due_date", req.DueDate)
	if err != nil {
		return nil, err
	}
	if dueDate.Before(existing.IssueDate) {
		return nil, validationError("due_date must be on or after issue_date")
	}

	next := *existing
	next.Status = req.Status
	next.DueDate = dueDate
	next.Notes = strings.TrimSpace(req.Notes)

	updated, err := s.invoices.Update(ctx, &next, existing.Status)
	if errors.Is(err, repository.ErrStateChanged) {
		return nil, conflictError("invoice %d was modified by another request; reload and retry", id)
	}
	return updated, err
}

func (s *InvoiceService) Delete(ctx context.Context, id int64) error {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if existing.Status != models.InvoiceStatusDraft && existing.Status != models.InvoiceStatusCancelled {
		return conflictError("only draft or cancelled invoices can be deleted; invoice %d is %s", id, existing.Status)
	}

	err = s.invoices.Delete(ctx, id, deletableInvoiceStatuses...)
	if errors.Is(err, repository.ErrStateChanged) {
		return conflictError("invoice %d was modified by another request; reload and retry", id)
	}
	return err
}

func (s *InvoiceService) newInvoiceNumber() (string, error) {
	buf := make([]byte, invoiceNumberRandBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate invoice number: %w", err)
	}
	return fmt.Sprintf("INV-%s-%s", s.now().UTC().Format("20060102"), strings.ToUpper(hex.EncodeToString(buf))), nil
}

func buildInvoiceItem(ctx context.Context, products ProductFinder, req models.CreateInvoiceItemRequest) (*models.InvoiceItem, error) {
	if req.Quantity <= 0 {
		return nil, validationError("quantity must be greater than 0")
	}

	product, err := products.GetByID(ctx, req.ProductID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, validationError("product %d does not exist", req.ProductID)
		}
		return nil, err
	}

	unitPrice := product.Price
	if req.UnitPrice != nil {
		if *req.UnitPrice < 0 {
			return nil, validationError("unit_price must not be negative")
		}
		unitPrice = round2(*req.UnitPrice)
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = product.Name
	}

	return &models.InvoiceItem{
		ProductID:   product.ID,
		Description: description,
		Quantity:    req.Quantity,
		UnitPrice:   unitPrice,
		LineTotal:   round2(float64(req.Quantity) * unitPrice),
	}, nil
}

func parseDate(field, value string) (time.Time, error) {
	t, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, validationError("%s must be a date in YYYY-MM-DD format", field)
	}
	return t, nil
}
