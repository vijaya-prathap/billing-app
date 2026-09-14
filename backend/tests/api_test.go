package tests

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/middleware"
	"billing-app/backend/internal/models"
)

type apiError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Details   []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"details"`
	} `json:"error"`
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return v
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d: %s", want, rec.Code, rec.Body.String())
	}
}

func TestCustomerCRUDLifecycle(t *testing.T) {
	h := newTestRouter(newFakeStore(), fakePinger{})

	rec := doRequest(t, h, http.MethodPost, "/api/v1/customers",
		`{"name":"  Jane Doe ","email":"Jane@Example.COM","phone":"+1-555-0199"}`)
	expectStatus(t, rec, http.StatusCreated)
	created := decode[models.Customer](t, rec)
	if created.ID == 0 || created.Name != "Jane Doe" || created.Email != "jane@example.com" {
		t.Fatalf("customer not normalized on create: %+v", created)
	}

	rec = doRequest(t, h, http.MethodGet, "/api/v1/customers/1", "")
	expectStatus(t, rec, http.StatusOK)

	rec = doRequest(t, h, http.MethodGet, "/api/v1/customers?page=1&limit=10", "")
	expectStatus(t, rec, http.StatusOK)
	page := decode[models.PaginatedResponse[models.Customer]](t, rec)
	if page.Total != 1 || len(page.Data) != 1 || page.TotalPages != 1 || page.Limit != 10 {
		t.Fatalf("unexpected list page: %+v", page)
	}

	rec = doRequest(t, h, http.MethodPut, "/api/v1/customers/1",
		`{"name":"Jane Smith","email":"jane@example.com","address":"1 Main St"}`)
	expectStatus(t, rec, http.StatusOK)
	if updated := decode[models.Customer](t, rec); updated.Name != "Jane Smith" || updated.Address != "1 Main St" {
		t.Fatalf("update not applied: %+v", updated)
	}

	rec = doRequest(t, h, http.MethodDelete, "/api/v1/customers/1", "")
	expectStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body on 204, got %q", rec.Body.String())
	}

	rec = doRequest(t, h, http.MethodGet, "/api/v1/customers/1", "")
	expectStatus(t, rec, http.StatusNotFound)
	if body := decode[apiError](t, rec); body.Error.Code != "not_found" {
		t.Fatalf("expected not_found code, got %+v", body)
	}
}

func TestCustomerErrorResponses(t *testing.T) {
	store := newFakeStore()
	h := newTestRouter(store, fakePinger{})

	expectStatus(t, doRequest(t, h, http.MethodPost, "/api/v1/customers",
		`{"name":"Acme","email":"billing@acme.test"}`), http.StatusCreated)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{"missing and invalid fields", http.MethodPost, "/api/v1/customers", `{"email":"not-an-email"}`, http.StatusUnprocessableEntity, "validation_failed", "name"},
		{"blank name after trimming", http.MethodPost, "/api/v1/customers", `{"name":"   ","email":"x@y.test"}`, http.StatusUnprocessableEntity, "validation_failed", ""},
		{"malformed json", http.MethodPost, "/api/v1/customers", `{"name":`, http.StatusBadRequest, "invalid_request", ""},
		{"duplicate email is case-insensitive", http.MethodPost, "/api/v1/customers", `{"name":"Acme 2","email":"BILLING@acme.test"}`, http.StatusConflict, "conflict", ""},
		{"non-numeric id", http.MethodGet, "/api/v1/customers/abc", "", http.StatusBadRequest, "invalid_request", ""},
		{"zero id", http.MethodGet, "/api/v1/customers/0", "", http.StatusBadRequest, "invalid_request", ""},
		{"invalid pagination", http.MethodGet, "/api/v1/customers?limit=ten", "", http.StatusBadRequest, "invalid_request", ""},
		{"update missing customer", http.MethodPut, "/api/v1/customers/99", `{"name":"Nobody","email":"nobody@x.test"}`, http.StatusNotFound, "not_found", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, h, tc.method, tc.path, tc.body)
			expectStatus(t, rec, tc.wantStatus)
			body := decode[apiError](t, rec)
			if body.Error.Code != tc.wantCode {
				t.Fatalf("expected code %q, got %+v", tc.wantCode, body)
			}
			if body.Error.RequestID == "" {
				t.Fatal("expected request_id in error body")
			}
			if tc.wantField != "" {
				found := false
				for _, d := range body.Error.Details {
					found = found || d.Field == tc.wantField
				}
				if !found {
					t.Fatalf("expected validation detail for field %q, got %+v", tc.wantField, body.Error.Details)
				}
			}
		})
	}

	t.Run("delete customer with invoices", func(t *testing.T) {
		store.customers.referenced[1] = true
		rec := doRequest(t, h, http.MethodDelete, "/api/v1/customers/1", "")
		expectStatus(t, rec, http.StatusConflict)
	})
}

func TestInvoiceValidationReportsNestedFieldPaths(t *testing.T) {
	h := newTestRouter(newFakeStore(), fakePinger{})

	rec := doRequest(t, h, http.MethodPost, "/api/v1/invoices",
		`{"customer_id":1,"issue_date":"2026-09-01","due_date":"09/30/2026","items":[{"product_id":1,"quantity":0}]}`)
	expectStatus(t, rec, http.StatusUnprocessableEntity)

	fields := map[string]bool{}
	for _, d := range decode[apiError](t, rec).Error.Details {
		fields[d.Field] = true
	}
	for _, want := range []string{"due_date", "items[0].quantity"} {
		if !fields[want] {
			t.Fatalf("expected validation detail for %q, got %v", want, fields)
		}
	}
}

func TestInvoiceItemsCannotChangeOnceSent(t *testing.T) {
	store := newFakeStore()
	h := newTestRouter(store, fakePinger{})

	expectStatus(t, doRequest(t, h, http.MethodPost, "/api/v1/customers", `{"name":"Acme","email":"a@acme.test"}`), http.StatusCreated)
	expectStatus(t, doRequest(t, h, http.MethodPost, "/api/v1/products", `{"name":"Widget","sku":"w-1","price":10}`), http.StatusCreated)
	expectStatus(t, doRequest(t, h, http.MethodPost, "/api/v1/invoices",
		`{"customer_id":1,"issue_date":"2026-09-01","due_date":"2026-09-30","items":[{"product_id":1,"quantity":2}]}`), http.StatusCreated)

	expectStatus(t, doRequest(t, h, http.MethodPost, "/api/v1/invoices/1/items", `{"product_id":1,"quantity":1}`), http.StatusCreated)
	expectStatus(t, doRequest(t, h, http.MethodPut, "/api/v1/invoices/1", `{"status":"sent","due_date":"2026-09-30"}`), http.StatusOK)

	rec := doRequest(t, h, http.MethodPost, "/api/v1/invoices/1/items", `{"product_id":1,"quantity":1}`)
	expectStatus(t, rec, http.StatusConflict)

	rec = doRequest(t, h, http.MethodDelete, "/api/v1/invoices/1/items/1", "")
	expectStatus(t, rec, http.StatusConflict)

	rec = doRequest(t, h, http.MethodGet, "/api/v1/invoices/1/items", "")
	expectStatus(t, rec, http.StatusOK)
	if items := decode[struct{ Data []models.InvoiceItem }](t, rec); len(items.Data) != 2 {
		t.Fatalf("expected 2 items to remain, got %d", len(items.Data))
	}

	expectStatus(t, doRequest(t, h, http.MethodPost, "/api/v1/invoices/99/items", `{"product_id":1,"quantity":1}`), http.StatusNotFound)
}

func TestHealthReadinessAndRouting(t *testing.T) {
	t.Run("liveness does not depend on the database", func(t *testing.T) {
		h := newTestRouter(newFakeStore(), fakePinger{err: errors.New("db down")})
		expectStatus(t, doRequest(t, h, http.MethodGet, "/health", ""), http.StatusOK)
	})

	t.Run("readiness reflects database connectivity", func(t *testing.T) {
		expectStatus(t, doRequest(t, newTestRouter(newFakeStore(), fakePinger{}), http.MethodGet, "/ready", ""), http.StatusOK)
		expectStatus(t, doRequest(t, newTestRouter(newFakeStore(), fakePinger{err: errors.New("db down")}), http.MethodGet, "/ready", ""), http.StatusServiceUnavailable)
	})

	h := newTestRouter(newFakeStore(), fakePinger{})

	t.Run("unknown route returns JSON 404", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodGet, "/api/v1/nope", "")
		expectStatus(t, rec, http.StatusNotFound)
		if body := decode[apiError](t, rec); body.Error.Code != "not_found" {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		expectStatus(t, doRequest(t, h, http.MethodPatch, "/api/v1/customers", ""), http.StatusMethodNotAllowed)
	})

	t.Run("request id is generated and propagated", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodGet, "/health", "")
		if len(rec.Header().Get(middleware.RequestIDHeader)) != 32 {
			t.Fatalf("expected generated 32-char request id, got %q", rec.Header().Get(middleware.RequestIDHeader))
		}

		rec = doRequest(t, h, http.MethodGet, "/health", "", middleware.RequestIDHeader, "trace-abc_123")
		if got := rec.Header().Get(middleware.RequestIDHeader); got != "trace-abc_123" {
			t.Fatalf("expected inbound request id to be propagated, got %q", got)
		}

		rec = doRequest(t, h, http.MethodGet, "/health", "", middleware.RequestIDHeader, "bad id\r\ninjected")
		if got := rec.Header().Get(middleware.RequestIDHeader); got == "bad id\r\ninjected" || len(got) != 32 {
			t.Fatalf("expected unsafe request id to be replaced, got %q", got)
		}
	})
}

func TestRecoveryMiddlewareReturnsJSON500(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestLogger(testLogger), middleware.Recovery(testLogger))
	r.GET("/boom", func(*gin.Context) { panic("unexpected nil pointer") })

	rec := doRequest(t, r, http.MethodGet, "/boom", "")
	expectStatus(t, rec, http.StatusInternalServerError)
	body := decode[apiError](t, rec)
	if body.Error.Code != "internal_error" || body.Error.RequestID == "" {
		t.Fatalf("unexpected panic response: %s", rec.Body.String())
	}
}
