package orden

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testOwnerEmail = "owner@example.invalid"

func TestPassword(t *testing.T) {
	if PasswordValid("short") || !PasswordValid("a long private password") || PasswordValid(strings.Repeat("a", 257)) {
		t.Fatal("password policy")
	}
	hash, err := HashPassword("a long private password")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "a long private password") || VerifyPassword(hash, "wrong password") || VerifyPassword("malformed", "a long private password") {
		t.Fatal("password verification")
	}
	second, _ := HashPassword("a long private password")
	if hash == second {
		t.Fatal("salts must differ")
	}
}

func TestOperationBudgets(t *testing.T) {
	s := &Server{config: Config{Origin: "https://orden.example.com"}}
	for path, want := range map[string]time.Duration{"/api/accounts": 10 * time.Second, "/api/gmail/callback": 25 * time.Second, "/api/gmail/sync": 75 * time.Second} {
		h := s.security(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deadline, ok := r.Context().Deadline()
			remaining := time.Until(deadline)
			if !ok || remaining < want-time.Second || remaining > want {
				t.Errorf("%s: unexpected deadline %v", path, remaining)
			}
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
}
func TestMoney(t *testing.T) {
	cases := []struct {
		value, unit    string
		positive, want bool
	}{
		{"1200", "CLP", true, true}, {"1200.00", "CLP", true, true}, {"1200.01", "CLP", true, false}, {"10.25", "USD", true, true}, {"0.10", "USD", true, true},
		{"0", "USD", true, false}, {"-1", "USD", true, false}, {"-20", "CLP", false, true}, {"1.001", "USD", true, false}, {"1e2", "USD", true, false}, {"NaN", "USD", true, false}, {"1,2", "USD", true, false}, {"9999999999999", "USD", false, false},
	}
	for _, c := range cases {
		if got := validMoney(c.value, c.unit, c.positive); got != c.want {
			t.Errorf("%+v got %v", c, got)
		}
	}
}
func TestConfig(t *testing.T) {
	t.Setenv("OWNER_EMAIL", testOwnerEmail)
	t.Setenv("INTERNAL_PROXY_SECRET", strings.Repeat("a", 64))
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_ORIGIN", "http://example.com")
	if _, e := LoadConfig(); e == nil {
		t.Fatal("production HTTP allowed")
	}
	t.Setenv("APP_ORIGIN", "https://orden.example.com")
	c, e := LoadConfig()
	if e != nil || !c.SecureCookies {
		t.Fatal("HTTPS config")
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_ORIGIN", "http://public.example.com")
	if _, e = LoadConfig(); e == nil {
		t.Fatal("public development forbidden")
	}
	t.Setenv("APP_ORIGIN", "http://localhost:8443")
	c, e = LoadConfig()
	if e != nil || c.SecureCookies {
		t.Fatal("local config")
	}
	t.Setenv("OWNER_EMAIL", "")
	if _, e = LoadConfig(); e == nil {
		t.Fatal("missing owner accepted")
	}
	t.Setenv("OWNER_EMAIL", "Name <owner@example.invalid>")
	if _, e = LoadConfig(); e == nil {
		t.Fatal("display-name owner accepted")
	}
	t.Setenv("OWNER_EMAIL", testOwnerEmail)
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_ORIGIN", "https://orden.example.com")
	t.Setenv("INTERNAL_PROXY_SECRET", "weak")
	if _, e = LoadConfig(); e == nil {
		t.Fatal("weak proxy secret accepted")
	}
}
func TestSecurityMiddleware(t *testing.T) {
	s := &Server{config: Config{Origin: "https://orden.example.com", SecureCookies: true}}
	h := s.security(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]bool{"ok": true}) }))
	for _, origin := range []string{"", "https://evil.example", "https://orden.example.com"} {
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("{}"))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 403
		if origin == s.config.Origin {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("origin %s: %d", origin, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("security headers")
		}
	}
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("{}"))
	r.Header.Set("Origin", s.config.Origin)
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatal("content type")
	}
}
