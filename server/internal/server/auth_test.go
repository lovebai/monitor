package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestHandlerAuth(t *testing.T, auth bool) *Handler {
	t.Helper()
	cfg := Config{AgentTokens: map[string]string{"n1": "t"}, DatabasePath: filepath.Join(t.TempDir(), "monitor-test.db"), OfflineAfter: time.Minute}
	if auth {
		cfg.AuthEnabled = true
		cfg.AuthUsername = "admin"
		cfg.AuthPassword = "secret"
	}
	h, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

func TestAuthDisabledAllowsAnonymous(t *testing.T) {
	h := newTestHandlerAuth(t, false)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("GET / without auth = %d, want 200", rr.Code)
	}
}

func TestAuthEnabledBlocksAnonymous(t *testing.T) {
	h := newTestHandlerAuth(t, true)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/login" {
		t.Errorf("GET / anonymous = %d loc=%q, want 302 /login", rr.Code, rr.Header().Get("Location"))
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/v1/nodes anonymous = %d, want 401", rr.Code)
	}
}

func TestAuthLoginAndSession(t *testing.T) {
	h := newTestHandlerAuth(t, true)
	form := url.Values{"username": {"admin"}, "password": {"wrong"}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/login?err=1" {
		t.Errorf("bad login = %d loc=%q, want 302 /login?err=1", rr.Code, rr.Header().Get("Location"))
	}

	form.Set("password", "secret")
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("login = %d, want 302", rr.Code)
	}
	var sess *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session" {
			sess = c
		}
	}
	if sess == nil || sess.Value == "" {
		t.Fatal("login should set session cookie")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sess)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET / with session = %d, want 200", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(sess)
	h.ServeHTTP(rr, req)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sess)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("GET / after logout = %d, want 302", rr.Code)
	}
}

func TestAuthDoesNotBlockIngest(t *testing.T) {
	h := newTestHandlerAuth(t, true)
	body := `{"node_id":"n1","hostname":"h1","os":{"name":"linux","architecture":"amd64"},"resources":{},"interfaces":[],"network":{}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("ingest with auth enabled = %d, want 204", rr.Code)
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := GeneratePasswordHash("S3cret!")
	if err != nil {
		t.Fatal(err)
	}
	if !IsPasswordHash(h) {
		t.Errorf("generated hash should be recognized: %q", h)
	}
	if !VerifyPasswordHash("S3cret!", h) {
		t.Error("correct password should verify against hash")
	}
	if VerifyPasswordHash("wrong", h) {
		t.Error("wrong password must not verify against hash")
	}
	if VerifyPasswordHash("S3cret!", "plaintext") {
		t.Error("plaintext value must not verify as hash")
	}
	if _, err := GeneratePasswordHash(""); err == nil {
		t.Error("empty password should fail")
	}
}

func TestLoginWithHashedPassword(t *testing.T) {
	hash, err := GeneratePasswordHash("secret")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{AgentTokens: map[string]string{"n1": "t"}, DatabasePath: filepath.Join(t.TempDir(), "monitor-test.db"), OfflineAfter: time.Minute, AuthEnabled: true, AuthUsername: "admin", AuthPassword: hash}
	h, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })

	post := func(pwd string) *httptest.ResponseRecorder {
		form := url.Values{"username": {"admin"}, "password": {pwd}}
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.ServeHTTP(rr, req)
		return rr
	}
	if rr := post("wrong"); rr.Code != http.StatusFound || rr.Header().Get("Location") != "/login?err=1" {
		t.Errorf("bad login = %d loc=%q, want 302 /login?err=1", rr.Code, rr.Header().Get("Location"))
	}
	rr := post("secret")
	if rr.Code != http.StatusFound || rr.Header().Get("Location") == "/login?err=1" {
		t.Fatalf("hashed login = %d loc=%q, want 302 to dashboard", rr.Code, rr.Header().Get("Location"))
	}
	var sess *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session" {
			sess = c
		}
	}
	if sess == nil || sess.Value == "" {
		t.Fatal("login should set session cookie")
	}
}
