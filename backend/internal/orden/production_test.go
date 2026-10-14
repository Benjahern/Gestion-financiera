package orden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProductionIsolationAndUnicode(t *testing.T) {
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
	schema := fmt.Sprintf("orden_production_test_%d", time.Now().UnixNano())
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
	// Simulate an existing installation's owner-specific constraint.
	if _, err = db.Exec(ctx, `CREATE TABLE users(id smallint PRIMARY KEY CHECK(id=1),email text NOT NULL UNIQUE CONSTRAINT users_email_check CHECK(email='owner@example.invalid'),password_hash text NOT NULL,created_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO users(id,email,password_hash) VALUES(1,$1,'test-only-unused-hash')", testOwnerEmail); err != nil {
		t.Fatal(err)
	}
	if err = ValidateOwner(ctx, db, testOwnerEmail); err != nil {
		t.Fatal(err)
	}
	if err = ValidateOwner(ctx, db, "different@example.invalid"); err == nil {
		t.Fatal("existing ownership can be transferred")
	}
	var constraintCount int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM pg_constraint WHERE conname='users_email_check' AND connamespace=$1::regnamespace", schema).Scan(&constraintCount); err != nil || constraintCount != 0 {
		t.Fatal("legacy email constraint not removed", err)
	}
	secret := strings.Repeat("a", 64)
	s := NewServer(db, Config{OwnerEmail: testOwnerEmail, Origin: "https://orden.example.com", SecureCookies: true, ProxySecret: secret})
	handler := s.Handler()
	token, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	csrf := "test-csrf"
	if _, err = db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,1,$2,now()+interval '1 hour')", tokenHash(token), csrf); err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body any, client, proxy string, authorized bool) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&b).Encode(body)
		}
		r := httptest.NewRequest(method, path, &b)
		r.Header.Set("Origin", s.config.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Orden-Proxy-Secret", proxy)
		r.Header.Set("X-Orden-Client-IP", client)
		r.Header.Set("X-Forwarded-For", "203.0.113.99") // Must never determine identity.
		if authorized {
			r.AddCookie(&http.Cookie{Name: s.cookieName(), Value: token})
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
	check(request("GET", "/api/accounts", nil, "192.0.2.10", "forged", true), 403)
	check(request("GET", "/api/accounts", nil, "not-an-ip", secret, true), 403)
	check(request("GET", "/api/accounts", nil, "192.0.2.10", secret, false), 401)
	for range 30 {
		check(request("POST", "/api/auth/start", map[string]string{"email": "other@example.invalid"}, "192.0.2.10", secret, false), 403)
	}
	check(request("POST", "/api/auth/start", map[string]string{"email": testOwnerEmail}, "192.0.2.10", secret, false), 429)
	check(request("POST", "/api/auth/start", map[string]string{"email": testOwnerEmail}, "192.0.2.11", secret, false), 200)
	category := strings.Repeat("á", 80)
	check(request("POST", "/api/categories", map[string]string{"name": category}, "192.0.2.11", secret, true), 200)
	check(request("GET", "/api/movements?category="+url.QueryEscape(category), nil, "192.0.2.11", secret, true), 200)
	check(request("GET", "/api/movements?search="+url.QueryEscape(strings.Repeat("ñ", 150)), nil, "192.0.2.11", secret, true), 200)
	check(request("GET", "/api/movements?category="+url.QueryEscape(strings.Repeat("á", 101)), nil, "192.0.2.11", secret, true), 400)
	check(request("GET", "/api/movements?category=%FF", nil, "192.0.2.11", secret, true), 400)
	var account int64
	if err = db.QueryRow(ctx, "INSERT INTO accounts(user_id,name,type,currency) VALUES(1,'MACH','Tarjeta','CLP') RETURNING id").Scan(&account); err != nil {
		t.Fatal(err)
	}
	check(request("POST", "/api/gmail/rules", map[string]any{"bank": "mach", "last_four": "4100", "account_id": account}, "192.0.2.11", secret, true), 200)
	check(request("PUT", fmt.Sprintf("/api/accounts/%d", account), map[string]string{"name": "MACH", "type": "Tarjeta", "currency": "USD", "opening_balance": "0"}, "192.0.2.11", secret, true), 409)
}
