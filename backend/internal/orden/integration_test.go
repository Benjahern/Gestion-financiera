package orden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not configured")
	}
	ctx := context.Background()
	base, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	schema := fmt.Sprintf("orden_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("migrations are not idempotent", err)
	}
	s := NewServer(db, Config{OwnerEmail: testOwnerEmail, Origin: "https://orden.example.com", SecureCookies: true})
	handler := s.Handler()
	request := func(method, path string, body any, cookie *http.Cookie, csrf, origin string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		r := httptest.NewRequest(method, path, &buf)
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("want %d got %d: %s", status, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/accounts", "/api/movements", "/api/overview", "/api/auth/session"} {
		check(request("GET", path, nil, nil, "", ""), 401)
	}
	check(request("POST", "/api/auth/start", map[string]string{"email": "other@example.com"}, nil, "", s.config.Origin), 403)
	check(request("POST", "/api/auth/start", map[string]string{"email": testOwnerEmail}, nil, "", s.config.Origin), 200)
	code, err := IssueCode(ctx, db, "activate")
	if err != nil {
		t.Fatal(err)
	}
	check(request("POST", "/api/auth/activate", credentials{testOwnerEmail, "a long private password", strings.Repeat("x", 43)}, nil, "", s.config.Origin), 403)
	// Competing activation requests must yield exactly one active owner/session.
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- request("POST", "/api/auth/activate", credentials{testOwnerEmail, "a long private password", code}, nil, "", s.config.Origin)
		}()
	}
	wg.Wait()
	close(results)
	var activated *httptest.ResponseRecorder
	successes := 0
	for result := range results {
		if result.Code == 200 {
			activated = result
			successes++
		} else if result.Code != 403 && result.Code != 409 {
			t.Fatal(result.Code, result.Body.String())
		}
	}
	if successes != 1 {
		t.Fatalf("activation successes %d", successes)
	}
	cookie := activated.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Name != "__Host-orden_session" || cookie.Domain != "" {
		t.Fatal("unsafe session cookie")
	}
	var auth struct {
		CSRF string `json:"csrf_token"`
	}
	_ = json.Unmarshal(activated.Body.Bytes(), &auth)
	check(request("POST", "/api/auth/activate", credentials{testOwnerEmail, "a long private password", code}, nil, "", s.config.Origin), 403)
	if _, err = IssueCode(ctx, db, "activate"); err == nil {
		t.Fatal("activation reopened")
	}
	check(request("POST", "/api/auth/login", credentials{Email: testOwnerEmail, Password: "wrong"}, nil, "", s.config.Origin), 401)
	check(request("POST", "/api/auth/login", credentials{Email: testOwnerEmail, Password: "a long private password"}, nil, "", s.config.Origin), 200)
	check(request("POST", "/api/accounts", map[string]string{}, cookie, "", s.config.Origin), 403)
	check(request("POST", "/api/accounts", map[string]string{}, cookie, auth.CSRF, "https://evil.example"), 403)
	createAccount := func(name, unit, balance string) int64 {
		w := request("POST", "/api/accounts", map[string]string{"name": name, "type": "Personal", "currency": unit, "opening_balance": balance}, cookie, auth.CSRF, s.config.Origin)
		check(w, 201)
		var result struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result.ID
	}
	clp := createAccount("Cuenta pesos", "CLP", "1000")
	usd := createAccount("Cuenta dólares", "USD", "20.10")
	check(request("POST", "/api/accounts", map[string]string{"name": "Fracción", "type": "Personal", "currency": "CLP", "opening_balance": "1.25"}, cookie, auth.CSRF, s.config.Origin), 400)
	check(request("POST", "/api/accounts", map[string]string{"name": "Cuenta pesos", "type": "Personal", "currency": "CLP", "opening_balance": "1"}, cookie, auth.CSRF, s.config.Origin), 409)
	createMovement := func(account int64, amount, kind string) int64 {
		w := request("POST", "/api/movements", map[string]any{"account_id": account, "amount": amount, "kind": kind, "title": "Movimiento", "category": "Personal", "date": "2026-10-12"}, cookie, auth.CSRF, s.config.Origin)
		check(w, 201)
		var result struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result.ID
	}
	expense := createMovement(clp, "100", "expense")
	createMovement(clp, "300", "income")
	createMovement(clp, "50", "investment")
	createMovement(usd, "0.10", "expense")
	createMovement(usd, "0.20", "expense")
	check(request("POST", "/api/movements", map[string]any{"account_id": clp, "amount": "0.50", "kind": "expense", "title": "Bad", "category": "Personal", "date": "2026-10-12"}, cookie, auth.CSRF, s.config.Origin), 400)
	check(request("DELETE", fmt.Sprintf("/api/accounts/%d", clp), nil, cookie, auth.CSRF, s.config.Origin), 409)
	check(request("PUT", fmt.Sprintf("/api/accounts/%d", clp), map[string]string{"name": "Cuenta pesos", "type": "Personal", "currency": "USD", "opening_balance": "1000"}, cookie, auth.CSRF, s.config.Origin), 409)
	w := request("GET", "/api/overview?currency=CLP&month=2026-10", nil, cookie, "", "")
	check(w, 200)
	var overview Overview
	_ = json.Unmarshal(w.Body.Bytes(), &overview)
	if overview.Balance != "1150.00" || overview.Expense != "100.00" || overview.Income != "300.00" || overview.Investment != "50.00" || len(overview.Flow) != 6 || len(overview.Categories) != 1 {
		t.Fatalf("wrong totals: %+v", overview)
	}
	w = request("GET", "/api/overview?currency=USD&month=2026-10", nil, cookie, "", "")
	check(w, 200)
	_ = json.Unmarshal(w.Body.Bytes(), &overview)
	if overview.Balance != "19.80" || overview.Expense != "0.30" {
		t.Fatalf("decimal precision: %+v", overview)
	}
	w = request("GET", "/api/movements?currency=CLP&kind=expense&month=2026-10&limit=1", nil, cookie, "", "")
	check(w, 200)
	var listed struct {
		Items []Movement `json:"items"`
		Total int        `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listed)
	if listed.Total != 1 || len(listed.Items) != 1 {
		t.Fatal("filters")
	}
	check(request("PUT", fmt.Sprintf("/api/movements/%d", expense), map[string]any{"account_id": clp, "amount": "200", "kind": "expense", "title": "Editado", "category": "Personal", "date": "2026-10-12"}, cookie, auth.CSRF, s.config.Origin), 200)
	check(request("DELETE", fmt.Sprintf("/api/movements/%d", expense), nil, cookie, auth.CSRF, s.config.Origin), 200)
	check(request("GET", "/api/movements?limit=1000", nil, cookie, "", ""), 400)
	check(request("GET", "/api/movements?search=%27%3BDROP%20TABLE%20users%3B--", nil, cookie, "", ""), 200)
	t.Run("investment account classification", func(t *testing.T) {
		testInvestmentClassification(t, request, cookie, auth.CSRF, s.config.Origin, clp, usd)
	})
	t.Run("summary account selection", func(t *testing.T) {
		testAccountSummary(t, db, s, request, cookie, auth.CSRF, s.config.Origin, clp, usd)
	})
	recovery, err := IssueCode(ctx, db, "recover")
	if err != nil {
		t.Fatal(err)
	}
	check(request("GET", "/api/accounts", nil, cookie, "", ""), 401)
	check(request("POST", "/api/auth/login", credentials{Email: testOwnerEmail, Password: "a long private password"}, nil, "", s.config.Origin), 401)
	w = request("POST", "/api/auth/recover", credentials{testOwnerEmail, "a new private password", recovery}, nil, "", s.config.Origin)
	check(w, 200)
	cookie = w.Result().Cookies()[0]
	_ = json.Unmarshal(w.Body.Bytes(), &auth)
	check(request("POST", "/api/auth/login", credentials{Email: testOwnerEmail, Password: "a long private password"}, nil, "", s.config.Origin), 401)
	check(request("POST", "/api/auth/logout-all", nil, cookie, auth.CSRF, s.config.Origin), 200)
	check(request("GET", "/api/accounts", nil, cookie, "", ""), 401)
	// Limits are durable across multiple server instances.
	_, _ = db.Exec(ctx, "DELETE FROM rate_limits WHERE key LIKE 'login:%'")
	for range 10 {
		check(request("POST", "/api/auth/login", map[string]string{"password": strings.Repeat("a", 257)}, nil, "", s.config.Origin), 401)
	}
	check(request("POST", "/api/auth/login", map[string]string{}, nil, "", s.config.Origin), 429)
}
