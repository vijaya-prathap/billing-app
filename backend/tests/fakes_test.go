package tests

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/repository"
	"billing-app/backend/internal/router"
	"billing-app/backend/internal/service"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakePinger struct{ err error }

func (p fakePinger) PingContext(context.Context) error { return p.err }

type fakeStore struct {
	customers *fakeCustomerRepo
	products  *fakeProductRepo
	invoices  *fakeInvoiceRepo
	items     *fakeInvoiceItemRepo
}

func newFakeStore() *fakeStore {
	invoices := &fakeInvoiceRepo{rows: map[int64]models.Invoice{}}
	return &fakeStore{
		customers: &fakeCustomerRepo{rows: map[int64]models.Customer{}, referenced: map[int64]bool{}},
		products:  &fakeProductRepo{rows: map[int64]models.Product{}},
		invoices:  invoices,
		items:     &fakeInvoiceItemRepo{invoices: invoices},
	}
}

func newTestRouter(store *fakeStore, db router.Pinger) http.Handler {
	return router.New(router.Dependencies{
		Logger:       testLogger,
		DB:           db,
		Customers:    service.NewCustomerService(store.customers),
		Products:     service.NewProductService(store.products),
		Invoices:     service.NewInvoiceService(store.invoices, store.customers, store.products),
		InvoiceItems: service.NewInvoiceItemService(store.items, store.invoices, store.products),
	})
}

func doRequest(t *testing.T, h http.Handler, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertServiceErrorKind(t *testing.T, err error, want service.ErrorKind) {
	t.Helper()
	var svcErr *service.Error
	if !errors.As(err, &svcErr) {
		t.Fatalf("expected *service.Error of kind %d, got %T: %v", want, err, err)
	}
	if svcErr.Kind != want {
		t.Fatalf("expected error kind %d, got %d (%s)", want, svcErr.Kind, svcErr.Message)
	}
}

type fakeCustomerRepo struct {
	mu         sync.Mutex
	nextID     int64
	rows       map[int64]models.Customer
	referenced map[int64]bool
}

func (r *fakeCustomerRepo) emailTaken(email string, exceptID int64) bool {
	for id, c := range r.rows {
		if id != exceptID && c.Email == email {
			return true
		}
	}
	return false
}

func (r *fakeCustomerRepo) Create(_ context.Context, c *models.Customer) (*models.Customer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.emailTaken(c.Email, 0) {
		return nil, repository.ErrDuplicate
	}
	r.nextID++
	row := *c
	row.ID = r.nextID
	row.CreatedAt, row.UpdatedAt = time.Now(), time.Now()
	r.rows[row.ID] = row
	return &row, nil
}

func (r *fakeCustomerRepo) GetByID(_ context.Context, id int64) (*models.Customer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &row, nil
}

func (r *fakeCustomerRepo) List(_ context.Context, p models.Pagination) ([]models.Customer, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]models.Customer, 0, len(r.rows))
	for _, c := range r.rows {
		all = append(all, c)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	return paginate(all, p), int64(len(all)), nil
}

func (r *fakeCustomerRepo) Update(_ context.Context, c *models.Customer) (*models.Customer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.rows[c.ID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if r.emailTaken(c.Email, c.ID) {
		return nil, repository.ErrDuplicate
	}
	row := *c
	row.CreatedAt, row.UpdatedAt = existing.CreatedAt, time.Now()
	r.rows[c.ID] = row
	return &row, nil
}

func (r *fakeCustomerRepo) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rows[id]; !ok {
		return repository.ErrNotFound
	}
	if r.referenced[id] {
		return repository.ErrForeignKeyViolation
	}
	delete(r.rows, id)
	return nil
}

type fakeProductRepo struct {
	mu     sync.Mutex
	nextID int64
	rows   map[int64]models.Product
}

func (r *fakeProductRepo) add(p models.Product) models.Product {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	p.ID = r.nextID
	r.rows[p.ID] = p
	return p
}

func (r *fakeProductRepo) Create(_ context.Context, p *models.Product) (*models.Product, error) {
	for _, existing := range r.rows {
		if existing.SKU == p.SKU {
			return nil, repository.ErrDuplicate
		}
	}
	row := r.add(*p)
	return &row, nil
}

func (r *fakeProductRepo) GetByID(_ context.Context, id int64) (*models.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &row, nil
}

func (r *fakeProductRepo) List(_ context.Context, p models.Pagination) ([]models.Product, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]models.Product, 0, len(r.rows))
	for _, prod := range r.rows {
		all = append(all, prod)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	return paginate(all, p), int64(len(all)), nil
}

func (r *fakeProductRepo) Update(_ context.Context, p *models.Product) (*models.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rows[p.ID]; !ok {
		return nil, repository.ErrNotFound
	}
	r.rows[p.ID] = *p
	row := *p
	return &row, nil
}

func (r *fakeProductRepo) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rows[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.rows, id)
	return nil
}

type fakeInvoiceRepo struct {
	mu     sync.Mutex
	nextID int64
	rows   map[int64]models.Invoice
}

func (r *fakeInvoiceRepo) Create(_ context.Context, inv *models.Invoice) (*models.Invoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	row := *inv
	row.ID = r.nextID
	row.Items = append([]models.InvoiceItem(nil), inv.Items...)
	for i := range row.Items {
		row.Items[i].ID = int64(i + 1)
		row.Items[i].InvoiceID = row.ID
	}
	r.rows[row.ID] = row
	return &row, nil
}

func (r *fakeInvoiceRepo) GetByID(_ context.Context, id int64) (*models.Invoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &row, nil
}

func (r *fakeInvoiceRepo) List(_ context.Context, f models.InvoiceFilter) ([]models.Invoice, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []models.Invoice
	for _, inv := range r.rows {
		if (f.CustomerID == 0 || inv.CustomerID == f.CustomerID) && (f.Status == "" || inv.Status == f.Status) {
			all = append(all, inv)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	return paginate(all, f.Pagination), int64(len(all)), nil
}

func (r *fakeInvoiceRepo) Update(_ context.Context, inv *models.Invoice, expected models.InvoiceStatus) (*models.Invoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.rows[inv.ID]
	if !ok || existing.Status != expected {
		return nil, repository.ErrStateChanged
	}
	r.rows[inv.ID] = *inv
	row := *inv
	return &row, nil
}

func (r *fakeInvoiceRepo) Delete(_ context.Context, id int64, allowed ...models.InvoiceStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.rows[id]
	if !ok {
		return repository.ErrNotFound
	}
	for _, s := range allowed {
		if existing.Status == s {
			delete(r.rows, id)
			return nil
		}
	}
	return repository.ErrStateChanged
}

// fakeInvoiceItemRepo mirrors the SQL repository's draft-only rule so service error mapping is exercised.
type fakeInvoiceItemRepo struct {
	invoices *fakeInvoiceRepo
}

func (r *fakeInvoiceItemRepo) draftInvoice(invoiceID int64) (models.Invoice, error) {
	inv, ok := r.invoices.rows[invoiceID]
	if !ok {
		return models.Invoice{}, repository.ErrNotFound
	}
	if inv.Status != models.InvoiceStatusDraft {
		return models.Invoice{}, repository.ErrInvoiceNotEditable
	}
	return inv, nil
}

func (r *fakeInvoiceItemRepo) Create(_ context.Context, item *models.InvoiceItem) (*models.InvoiceItem, error) {
	r.invoices.mu.Lock()
	defer r.invoices.mu.Unlock()
	inv, err := r.draftInvoice(item.InvoiceID)
	if err != nil {
		return nil, err
	}
	row := *item
	row.ID = int64(len(inv.Items) + 1)
	inv.Items = append(inv.Items, row)
	r.invoices.rows[inv.ID] = inv
	return &row, nil
}

func (r *fakeInvoiceItemRepo) GetByID(_ context.Context, invoiceID, itemID int64) (*models.InvoiceItem, error) {
	r.invoices.mu.Lock()
	defer r.invoices.mu.Unlock()
	for _, item := range r.invoices.rows[invoiceID].Items {
		if item.ID == itemID {
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *fakeInvoiceItemRepo) Update(_ context.Context, item *models.InvoiceItem) (*models.InvoiceItem, error) {
	r.invoices.mu.Lock()
	defer r.invoices.mu.Unlock()
	inv, err := r.draftInvoice(item.InvoiceID)
	if err != nil {
		return nil, err
	}
	for i := range inv.Items {
		if inv.Items[i].ID == item.ID {
			inv.Items[i] = *item
			row := *item
			return &row, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *fakeInvoiceItemRepo) Delete(_ context.Context, invoiceID, itemID int64) error {
	r.invoices.mu.Lock()
	defer r.invoices.mu.Unlock()
	inv, err := r.draftInvoice(invoiceID)
	if err != nil {
		return err
	}
	for i := range inv.Items {
		if inv.Items[i].ID == itemID {
			inv.Items = append(inv.Items[:i], inv.Items[i+1:]...)
			r.invoices.rows[invoiceID] = inv
			return nil
		}
	}
	return repository.ErrNotFound
}

func paginate[T any](all []T, p models.Pagination) []T {
	start := p.Offset()
	if start >= len(all) {
		return []T{}
	}
	end := min(start+p.Limit, len(all))
	return all[start:end]
}
