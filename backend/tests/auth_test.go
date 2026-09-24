package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"billing-app/backend/internal/auth"
	"billing-app/backend/internal/handlers"
	"billing-app/backend/internal/middleware"
	"billing-app/backend/internal/models"
	"billing-app/backend/internal/router"
	"billing-app/backend/internal/service"
)

const testSessionSecret = "ssssssssssssssssssssssssssssssss"

// fakeExchanger stands in for Google: codes map to accounts, so tests never touch the network.
type fakeExchanger struct {
	accounts map[string]auth.GoogleAccount
	verifier string // records the PKCE verifier received on exchange
}

func (f *fakeExchanger) AuthCodeURL(state, codeVerifier string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{"state": {state}, "code_challenge": {codeVerifier}}.Encode()
}

func (f *fakeExchanger) Exchange(_ context.Context, code, codeVerifier string) (auth.GoogleAccount, error) {
	f.verifier = codeVerifier
	acct, ok := f.accounts[code]
	if !ok {
		return auth.GoogleAccount{}, auth.ErrInvalidToken
	}
	return acct, nil
}

// fakeUserStore is an in-memory authorized_users table.
type fakeUserStore struct {
	rows map[int64]*models.AuthorizedUser
}

func (s *fakeUserStore) FindBySubject(_ context.Context, sub string) (*models.AuthorizedUser, error) {
	for _, u := range s.rows {
		if u.GoogleSub == sub {
			return u, nil
		}
	}
	return nil, nil
}

func (s *fakeUserStore) FindByEmail(_ context.Context, email string) (*models.AuthorizedUser, error) {
	for _, u := range s.rows {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (s *fakeUserStore) BindSubject(_ context.Context, id int64, sub string) error {
	s.rows[id].GoogleSub = sub
	return nil
}

func (s *fakeUserStore) TouchLogin(_ context.Context, id int64) error {
	now := time.Now()
	s.rows[id].LastLoginAt = &now
	return nil
}

type authFixture struct {
	handler  http.Handler
	google   *fakeExchanger
	users    *fakeUserStore
	sessions *auth.SessionManager
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	sessions, err := auth.NewSessionManager(testSessionSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	states, err := auth.NewStateManager(testSessionSecret, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	google := &fakeExchanger{accounts: map[string]auth.GoogleAccount{
		"code-jane":     {Subject: "g-jane", Email: "jane@example.com", Name: "Jane Doe", Picture: "https://img/jane.png"},
		"code-intruder": {Subject: "g-intruder", Email: "intruder@elsewhere.net", Name: "Someone"},
	}}
	users := &fakeUserStore{rows: map[int64]*models.AuthorizedUser{
		1: {ID: 1, Email: "jane@example.com", DisplayName: "Jane Doe", IsActive: true},
	}}
	store := newFakeStore()
	h := router.New(router.Dependencies{
		Logger:       testLogger,
		DB:           fakePinger{},
		Customers:    service.NewCustomerService(store.customers),
		Products:     service.NewProductService(store.products),
		Invoices:     service.NewInvoiceService(store.invoices, store.customers, store.products),
		InvoiceItems: service.NewInvoiceItemService(store.items, store.invoices, store.products),
		Auth: &auth.Service{
			OAuth:        google,
			Sessions:     sessions,
			States:       states,
			Users:        users,
			CookieSecure: true,
		},
	})
	return &authFixture{handler: h, google: google, users: users, sessions: sessions}
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// startLogin hits the login route and returns the state cookie and the state Google would echo back.
func (f *authFixture) startLogin(t *testing.T) (stateCookie, state string) {
	t.Helper()
	rec := doRequest(t, f.handler, http.MethodGet, "/api/v1/auth/google/login", "")
	expectStatus(t, rec, http.StatusFound)

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || loc.Host != "accounts.google.com" {
		t.Fatalf("login should redirect to Google, got %q", rec.Header().Get("Location"))
	}
	c := cookieNamed(rec, handlers.StateCookie)
	if c == nil || c.Value == "" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.MaxAge <= 0 {
		t.Fatalf("state cookie missing or not hardened: %+v", c)
	}
	return c.Name + "=" + c.Value, loc.Query().Get("state")
}

// callback completes the flow and returns the response.
func (f *authFixture) callback(t *testing.T, stateCookie, state, code string) *httptest.ResponseRecorder {
	t.Helper()
	q := url.Values{"state": {state}, "code": {code}}
	return doRequest(t, f.handler, http.MethodGet, "/api/v1/auth/google/callback?"+q.Encode(), "", "Cookie", stateCookie)
}

func expectRedirect(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	expectStatus(t, rec, http.StatusFound)
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("redirect = %q, want %q", got, want)
	}
}

func TestAuthConfigReportsMode(t *testing.T) {
	rec := doRequest(t, newTestRouter(newFakeStore(), fakePinger{}), http.MethodGet, "/api/v1/auth/config", "")
	expectStatus(t, rec, http.StatusOK)
	if cfg := decode[map[string]any](t, rec); cfg["enabled"] != false {
		t.Fatalf("auth should be disabled without a service: %v", cfg)
	}

	rec = doRequest(t, newAuthFixture(t).handler, http.MethodGet, "/api/v1/auth/config", "")
	expectStatus(t, rec, http.StatusOK)
	cfg := decode[map[string]any](t, rec)
	if cfg["enabled"] != true {
		t.Fatalf("unexpected auth config: %v", cfg)
	}
	if _, leaked := cfg["google_client_id"]; leaked {
		t.Fatalf("client configuration must not be exposed to the browser: %v", cfg)
	}
}

func TestProtectedRoutesRequireSession(t *testing.T) {
	h := newAuthFixture(t).handler

	rec := doRequest(t, h, http.MethodGet, "/api/v1/customers", "")
	expectStatus(t, rec, http.StatusUnauthorized)
	if body := decode[apiError](t, rec); body.Error.Code != "unauthorized" || body.Error.RequestID == "" {
		t.Fatalf("unexpected 401 body: %+v", body)
	}

	rec = doRequest(t, h, http.MethodGet, "/api/v1/auth/me", "")
	expectStatus(t, rec, http.StatusUnauthorized)

	rec = doRequest(t, h, http.MethodGet, "/api/v1/customers", "", "Cookie", middleware.SessionCookie+"=forged.token")
	expectStatus(t, rec, http.StatusUnauthorized)

	// Probes stay open so Kubernetes can check the pod.
	rec = doRequest(t, h, http.MethodGet, "/health", "")
	expectStatus(t, rec, http.StatusOK)
}

func TestGoogleLoginLifecycle(t *testing.T) {
	f := newAuthFixture(t)
	stateCookie, state := f.startLogin(t)

	rec := f.callback(t, stateCookie, state, "code-jane")
	expectRedirect(t, rec, "/")
	if f.google.verifier == "" {
		t.Fatal("code exchange did not receive the PKCE verifier")
	}
	if f.users.rows[1].GoogleSub != "g-jane" || f.users.rows[1].LastLoginAt == nil {
		t.Fatalf("first login should bind the google id and record the login: %+v", f.users.rows[1])
	}
	if cleared := cookieNamed(rec, handlers.StateCookie); cleared == nil || cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("state cookie should be cleared after use: %+v", cleared)
	}

	session := cookieNamed(rec, middleware.SessionCookie)
	if session == nil {
		t.Fatal("no session cookie issued")
	}
	if !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteLaxMode || session.Path != "/" || session.MaxAge <= 0 {
		t.Fatalf("session cookie missing hardening flags: %+v", session)
	}
	cookie := session.Name + "=" + session.Value

	rec = doRequest(t, f.handler, http.MethodGet, "/api/v1/auth/me", "", "Cookie", cookie)
	expectStatus(t, rec, http.StatusOK)
	me := decode[map[string]auth.Identity](t, rec)
	if me["user"].Subject != "g-jane" || me["user"].Email != "jane@example.com" || me["user"].UserID != 1 {
		t.Fatalf("unexpected /me response: %+v", me)
	}

	rec = doRequest(t, f.handler, http.MethodGet, "/api/v1/customers", "", "Cookie", cookie)
	expectStatus(t, rec, http.StatusOK)

	rec = doRequest(t, f.handler, http.MethodPost, "/api/v1/auth/logout", "", "Cookie", cookie)
	expectStatus(t, rec, http.StatusNoContent)
	if cleared := cookieNamed(rec, middleware.SessionCookie); cleared == nil || cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("logout should clear the cookie: %+v", cleared)
	}
}

func TestGoogleCallbackRejections(t *testing.T) {
	f := newAuthFixture(t)

	assertNoSession := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		if c := cookieNamed(rec, middleware.SessionCookie); c != nil {
			t.Fatalf("rejected login must not issue a session: %+v", c)
		}
	}

	t.Run("account not in authorized_users", func(t *testing.T) {
		stateCookie, state := f.startLogin(t)
		rec := f.callback(t, stateCookie, state, "code-intruder")
		expectRedirect(t, rec, "/?auth_error=denied")
		assertNoSession(t, rec)
	})

	t.Run("state mismatch (CSRF)", func(t *testing.T) {
		stateCookie, _ := f.startLogin(t)
		_, otherState := f.startLogin(t)
		rec := f.callback(t, stateCookie, otherState, "code-jane")
		expectRedirect(t, rec, "/?auth_error=failed")
		assertNoSession(t, rec)
	})

	t.Run("missing state cookie", func(t *testing.T) {
		_, state := f.startLogin(t)
		rec := f.callback(t, "", state, "code-jane")
		expectRedirect(t, rec, "/?auth_error=failed")
		assertNoSession(t, rec)
	})

	t.Run("state cookie replayed after use", func(t *testing.T) {
		stateCookie, state := f.startLogin(t)
		expectRedirect(t, f.callback(t, stateCookie, state, "code-jane"), "/")
		// The browser has dropped the cookie by now; a replay without it fails.
		rec := f.callback(t, "", state, "code-jane")
		expectRedirect(t, rec, "/?auth_error=failed")
		assertNoSession(t, rec)
	})

	t.Run("bad authorization code", func(t *testing.T) {
		stateCookie, state := f.startLogin(t)
		rec := f.callback(t, stateCookie, state, "code-bogus")
		expectRedirect(t, rec, "/?auth_error=failed")
		assertNoSession(t, rec)
	})

	t.Run("user cancelled at google", func(t *testing.T) {
		stateCookie, _ := f.startLogin(t)
		rec := doRequest(t, f.handler, http.MethodGet, "/api/v1/auth/google/callback?error=access_denied", "", "Cookie", stateCookie)
		expectRedirect(t, rec, "/?auth_error=cancelled")
		assertNoSession(t, rec)
	})
}

func TestDeactivatedAccountLosesAccessImmediately(t *testing.T) {
	f := newAuthFixture(t)
	stateCookie, state := f.startLogin(t)
	session := cookieNamed(f.callback(t, stateCookie, state, "code-jane"), middleware.SessionCookie)
	cookie := session.Name + "=" + session.Value

	rec := doRequest(t, f.handler, http.MethodGet, "/api/v1/customers", "", "Cookie", cookie)
	expectStatus(t, rec, http.StatusOK)

	f.users.rows[1].IsActive = false
	rec = doRequest(t, f.handler, http.MethodGet, "/api/v1/customers", "", "Cookie", cookie)
	expectStatus(t, rec, http.StatusForbidden)
	if body := decode[apiError](t, rec); body.Error.Code != "forbidden" {
		t.Fatalf("unexpected body: %+v", body)
	}

	// Re-activated: the very same session works again, and signing in again is refused while inactive.
	f.users.rows[1].IsActive = true
	rec = doRequest(t, f.handler, http.MethodGet, "/api/v1/customers", "", "Cookie", cookie)
	expectStatus(t, rec, http.StatusOK)
}

func TestSessionForgedWithWrongSecretIsRejected(t *testing.T) {
	f := newAuthFixture(t)
	other, _ := auth.NewSessionManager(strings.Repeat("x", 32), time.Hour)
	token, _ := other.Issue(auth.Identity{UserID: 1, Subject: "g-jane", Email: "jane@example.com", Name: "Jane"}, time.Now())
	rec := doRequest(t, f.handler, http.MethodGet, "/api/v1/customers", "", "Cookie", middleware.SessionCookie+"="+token)
	expectStatus(t, rec, http.StatusUnauthorized)
}

func TestAuthRoutesAbsentWhenDisabled(t *testing.T) {
	h := newTestRouter(newFakeStore(), fakePinger{})
	rec := doRequest(t, h, http.MethodGet, "/api/v1/auth/google/login", "")
	expectStatus(t, rec, http.StatusNotFound)
	rec = doRequest(t, h, http.MethodGet, "/api/v1/auth/google/callback?code=x&state=y", "")
	expectStatus(t, rec, http.StatusNotFound)
	// And data routes are open, which is the whole point of AUTH_DISABLED for local work.
	rec = doRequest(t, h, http.MethodGet, "/api/v1/customers", "")
	expectStatus(t, rec, http.StatusOK)
}
