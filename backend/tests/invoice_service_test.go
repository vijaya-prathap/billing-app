package tests

import (
	"context"
	"strings"
	"testing"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/service"
)

type invoiceFixture struct {
	store    *fakeStore
	svc      *service.InvoiceService
	customer *models.Customer
	keyboard models.Product
	dock     models.Product
}

func newInvoiceFixture(t *testing.T) *invoiceFixture {
	t.Helper()
	store := newFakeStore()
	customer, err := store.customers.Create(context.Background(), &models.Customer{Name: "Acme", Email: "a@acme.test"})
	if err != nil {
		t.Fatal(err)
	}
	return &invoiceFixture{
		store:    store,
		svc:      service.NewInvoiceService(store.invoices, store.customers, store.products),
		customer: customer,
		keyboard: store.products.add(models.Product{Name: "Wireless Keyboard", SKU: "KBD", Price: 49.99}),
		dock:     store.products.add(models.Product{Name: "USB-C Dock", SKU: "DOCK", Price: 129.50}),
	}
}

func (f *invoiceFixture) createDraft(t *testing.T) *models.Invoice {
	t.Helper()
	inv, err := f.svc.Create(context.Background(), models.CreateInvoiceRequest{
		CustomerID: f.customer.ID,
		IssueDate:  "2026-09-01",
		DueDate:    "2026-09-30",
		Items:      []models.CreateInvoiceItemRequest{{ProductID: f.keyboard.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create draft invoice: %v", err)
	}
	return inv
}

func float64Ptr(v float64) *float64 { return &v }

func TestCreateInvoiceComputesTotalsFromItems(t *testing.T) {
	f := newInvoiceFixture(t)

	inv, err := f.svc.Create(context.Background(), models.CreateInvoiceRequest{
		CustomerID: f.customer.ID,
		IssueDate:  "2026-09-01",
		DueDate:    "2026-09-30",
		TaxRate:    18,
		Notes:      "  Net 30  ",
		Items: []models.CreateInvoiceItemRequest{
			{ProductID: f.keyboard.ID, Quantity: 3},
			{ProductID: f.dock.ID, Quantity: 2, UnitPrice: float64Ptr(119.5), Description: "Dock (volume discount)"},
		},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	// 3 x 49.99 = 149.97; 2 x 119.50 = 239.00; subtotal 388.97; 18% tax = 70.0146 -> 70.01.
	if got := inv.Items[0]; got.UnitPrice != 49.99 || got.LineTotal != 149.97 || got.Description != "Wireless Keyboard" {
		t.Fatalf("item defaults from product not applied: %+v", got)
	}
	if got := inv.Items[1]; got.UnitPrice != 119.50 || got.LineTotal != 239.00 || got.Description != "Dock (volume discount)" {
		t.Fatalf("item price override not applied: %+v", got)
	}
	if inv.Subtotal != 388.97 || inv.TaxAmount != 70.01 || inv.Total != 458.98 {
		t.Fatalf("unexpected totals: subtotal=%v tax=%v total=%v", inv.Subtotal, inv.TaxAmount, inv.Total)
	}
	if inv.Status != models.InvoiceStatusDraft {
		t.Fatalf("new invoices must start as draft, got %s", inv.Status)
	}
	if !strings.HasPrefix(inv.InvoiceNumber, "INV-") || len(inv.InvoiceNumber) != len("INV-20260901-ABCDEF12") {
		t.Fatalf("unexpected invoice number format: %q", inv.InvoiceNumber)
	}
	if inv.Notes != "Net 30" {
		t.Fatalf("notes not trimmed: %q", inv.Notes)
	}
}

func TestCreateInvoiceRejectsInvalidInput(t *testing.T) {
	f := newInvoiceFixture(t)
	valid := func() models.CreateInvoiceRequest {
		return models.CreateInvoiceRequest{
			CustomerID: f.customer.ID,
			IssueDate:  "2026-09-01",
			DueDate:    "2026-09-30",
			Items:      []models.CreateInvoiceItemRequest{{ProductID: f.keyboard.ID, Quantity: 1}},
		}
	}

	tests := []struct {
		name   string
		mutate func(*models.CreateInvoiceRequest)
	}{
		{"due date before issue date", func(r *models.CreateInvoiceRequest) { r.DueDate = "2026-08-31" }},
		{"malformed date", func(r *models.CreateInvoiceRequest) { r.IssueDate = "2026-13-01" }},
		{"unknown customer", func(r *models.CreateInvoiceRequest) { r.CustomerID = 404 }},
		{"unknown product", func(r *models.CreateInvoiceRequest) { r.Items[0].ProductID = 404 }},
		{"no items", func(r *models.CreateInvoiceRequest) { r.Items = nil }},
		{"negative unit price", func(r *models.CreateInvoiceRequest) { r.Items[0].UnitPrice = float64Ptr(-1) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.mutate(&req)
			_, err := f.svc.Create(context.Background(), req)
			assertServiceErrorKind(t, err, service.KindValidation)
		})
	}

	if len(f.store.invoices.rows) != 0 {
		t.Fatalf("rejected requests must not persist invoices, found %d", len(f.store.invoices.rows))
	}
}

func TestInvoiceStatusLifecycle(t *testing.T) {
	ctx := context.Background()
	f := newInvoiceFixture(t)
	inv := f.createDraft(t)

	update := func(status models.InvoiceStatus) error {
		_, err := f.svc.Update(ctx, inv.ID, models.UpdateInvoiceRequest{Status: status, DueDate: "2026-09-30"})
		return err
	}

	assertServiceErrorKind(t, update(models.InvoiceStatusPaid), service.KindConflict)

	if err := update(models.InvoiceStatusSent); err != nil {
		t.Fatalf("draft -> sent: %v", err)
	}
	assertServiceErrorKind(t, update(models.InvoiceStatusDraft), service.KindConflict)

	if err := update(models.InvoiceStatusPaid); err != nil {
		t.Fatalf("sent -> paid: %v", err)
	}
	assertServiceErrorKind(t, update(models.InvoiceStatusCancelled), service.KindConflict)
	assertServiceErrorKind(t, update(models.InvoiceStatusPaid), service.KindConflict)

	_, err := f.svc.Update(ctx, 404, models.UpdateInvoiceRequest{Status: models.InvoiceStatusSent, DueDate: "2026-09-30"})
	assertServiceErrorKind(t, err, service.KindNotFound)
}

func TestInvoiceUpdateRejectsDueDateBeforeIssueDate(t *testing.T) {
	f := newInvoiceFixture(t)
	inv := f.createDraft(t)

	_, err := f.svc.Update(context.Background(), inv.ID, models.UpdateInvoiceRequest{
		Status:  models.InvoiceStatusDraft,
		DueDate: "2026-08-01",
	})
	assertServiceErrorKind(t, err, service.KindValidation)
}

func TestOnlyDraftOrCancelledInvoicesCanBeDeleted(t *testing.T) {
	ctx := context.Background()
	f := newInvoiceFixture(t)

	sent := f.createDraft(t)
	if _, err := f.svc.Update(ctx, sent.ID, models.UpdateInvoiceRequest{Status: models.InvoiceStatusSent, DueDate: "2026-09-30"}); err != nil {
		t.Fatal(err)
	}
	assertServiceErrorKind(t, f.svc.Delete(ctx, sent.ID), service.KindConflict)

	if _, err := f.svc.Update(ctx, sent.ID, models.UpdateInvoiceRequest{Status: models.InvoiceStatusCancelled, DueDate: "2026-09-30"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx, sent.ID); err != nil {
		t.Fatalf("cancelled invoice should be deletable: %v", err)
	}

	draft := f.createDraft(t)
	if err := f.svc.Delete(ctx, draft.ID); err != nil {
		t.Fatalf("draft invoice should be deletable: %v", err)
	}
	assertServiceErrorKind(t, f.svc.Delete(ctx, draft.ID), service.KindNotFound)
}

func TestListInvoicesRejectsUnknownStatus(t *testing.T) {
	f := newInvoiceFixture(t)
	_, err := f.svc.List(context.Background(), models.InvoiceFilter{
		Status:     "overdue",
		Pagination: models.NewPagination(1, 20),
	})
	assertServiceErrorKind(t, err, service.KindValidation)
}

func TestNewPaginationClampsInput(t *testing.T) {
	tests := []struct {
		page, limit         int
		wantPage, wantLimit int
		wantOffset          int
	}{
		{0, 0, models.DefaultPage, models.DefaultLimit, 0},
		{-3, 5, 1, 5, 0},
		{3, 1000, 3, models.MaxLimit, 200},
		{2, 25, 2, 25, 25},
	}
	for _, tc := range tests {
		p := models.NewPagination(tc.page, tc.limit)
		if p.Page != tc.wantPage || p.Limit != tc.wantLimit || p.Offset() != tc.wantOffset {
			t.Errorf("NewPagination(%d, %d) = %+v offset %d; want page=%d limit=%d offset=%d",
				tc.page, tc.limit, p, p.Offset(), tc.wantPage, tc.wantLimit, tc.wantOffset)
		}
	}

	if got := models.NewPaginatedResponse[models.Customer](nil, models.NewPagination(1, 20), 41); got.TotalPages != 3 || got.Data == nil {
		t.Errorf("expected 3 total pages and non-nil data, got %+v", got)
	}
}
