package orden

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInvestmentAccountType(t *testing.T) {
	for _, value := range []string{"inversión", " INVERSION ", "Inversiones", "Cuenta de inversión", "cuenta de inversion"} {
		if !investmentAccountType(value) {
			t.Errorf("expected investment type %q", value)
		}
	}
	for _, value := range []string{"", "Banco", "no inversión", "inversiones pendientes", "cuenta de inversiones"} {
		if investmentAccountType(value) {
			t.Errorf("unexpected investment type %q", value)
		}
	}
}

func TestInvestmentAccountsMigration(t *testing.T) {
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
	schema := pgx.Identifier{fmt.Sprintf("orden_investment_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
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
	// Reproduce an existing installation; 006 must not depend on a 005 migration.
	if _, err = db.Exec(ctx, "CREATE TABLE schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "005" {
			continue
		}
		data, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(ctx, "INSERT INTO users(id,email,password_hash) VALUES(1,$1,'unused')", testOwnerEmail); err != nil {
		t.Fatal(err)
	}
	types := []string{"inversión", " INVERSION ", "Inversiones", "Cuenta de inversión", "cuenta de inversion", "no inversión", "Banco", "inversiones pendientes"}
	for i, kind := range types {
		if _, err = db.Exec(ctx, "INSERT INTO accounts(user_id,name,type,currency,opening_balance) VALUES(1,$1,$2,'CLP',1000)", fmt.Sprintf("Account %d", i), kind); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(ctx, "INSERT INTO movements(user_id,account_id,title,category,date,amount,kind) SELECT 1,min(id),'Existing','Personal','2026-10-12',50,'investment' FROM accounts"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(ctx, "SELECT type,is_investment,opening_balance::text FROM accounts ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var kind, balance string
		var flagged bool
		if err = rows.Scan(&kind, &flagged, &balance); err != nil {
			t.Fatal(err)
		}
		if flagged != (count < 5) || balance != "1000.00" {
			t.Fatalf("incorrect migration result for %q", kind)
		}
		count++
	}
	rows.Close()
	if rows.Err() != nil || count != len(types) {
		t.Fatal("migration rows", rows.Err(), count)
	}
	// Unmarking must survive subsequent startup migration runs.
	if _, err = db.Exec(ctx, "UPDATE accounts SET is_investment=false"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var flagged, movements int
	if err = db.QueryRow(ctx, "SELECT (SELECT count(*) FROM accounts WHERE is_investment),(SELECT count(*) FROM movements WHERE amount=50 AND kind='investment')").Scan(&flagged, &movements); err != nil {
		t.Fatal(err)
	}
	if flagged != 0 || movements != 1 {
		t.Fatal("migration replay changed classification or movements")
	}
	// Production permits only owner 1. Inject another authenticated identity at
	// the handler boundary to verify that the UPDATE still scopes ownership.
	var accountID int64
	if err = db.QueryRow(ctx, "SELECT min(id) FROM accounts").Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("PUT", "/api/accounts/1/investment", strings.NewReader(`{"is_investment":true}`))
	r.SetPathValue("id", fmt.Sprint(accountID))
	r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, session{UserID: 2}))
	w := httptest.NewRecorder()
	s := NewServer(db, Config{OwnerEmail: testOwnerEmail, Origin: "https://orden.example.com"})
	s.setAccountInvestment(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign account update status: %d", w.Code)
	}
	if err = db.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE is_investment").Scan(&flagged); err != nil || flagged != 0 {
		t.Fatal("foreign identity changed classification", err)
	}
}

func testInvestmentClassification(t *testing.T, request func(string, string, any, *http.Cookie, string, string) *httptest.ResponseRecorder, cookie *http.Cookie, csrf, origin string, clp, usd int64) {
	t.Helper()
	check := func(w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("want status %d, got %d: %s", want, w.Code, w.Body.String())
		}
	}
	get := func(path string, out any) {
		t.Helper()
		w := request("GET", path, nil, cookie, "", "")
		check(w, 200)
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatal(err)
		}
	}
	var before, after []Account
	var movementsBefore, movementsAfter any
	var overviewBefore, overviewAfter any
	get("/api/accounts", &before)
	get("/api/movements", &movementsBefore)
	get("/api/overview?currency=CLP&month=2026-10", &overviewBefore)
	path := fmt.Sprintf("/api/accounts/%d/investment", clp)
	body := map[string]bool{"is_investment": true}
	check(request("PUT", path, body, nil, "", origin), 401)
	check(request("PUT", path, body, cookie, "", origin), 403)
	check(request("PUT", path, body, cookie, "wrong", origin), 403)
	check(request("PUT", path, body, cookie, csrf, "https://evil.example"), 403)
	for _, id := range []string{"0", "-1", "bad", "9223372036854775808"} {
		check(request("PUT", "/api/accounts/"+id+"/investment", body, cookie, csrf, origin), 400)
	}
	// Single-owner schema forbids foreign owners; missing IDs must never be updated.
	check(request("PUT", "/api/accounts/9223372036854775807/investment", body, cookie, csrf, origin), 404)
	for _, invalid := range []any{map[string]any{}, map[string]any{"is_investment": nil}, map[string]any{"is_investment": "true"}} {
		check(request("PUT", path, invalid, cookie, csrf, origin), 400)
	}
	check(request("PUT", path, body, cookie, csrf, origin), 200)
	check(request("PUT", path, body, cookie, csrf, origin), 200)
	get("/api/accounts", &after)
	for i := range before {
		want := before[i]
		if want.ID == clp {
			want.IsInvestment = true
		}
		if !reflect.DeepEqual(want, after[i]) {
			t.Fatal("classification changed account fields")
		}
		if after[i].ID == usd && after[i].IsInvestment {
			t.Fatal("classification leaked to another currency")
		}
	}
	get("/api/movements", &movementsAfter)
	get("/api/overview?currency=CLP&month=2026-10", &overviewAfter)
	if !reflect.DeepEqual(movementsBefore, movementsAfter) || !reflect.DeepEqual(overviewBefore, overviewAfter) {
		t.Fatal("classification changed movements or balances")
	}
	check(request("PUT", path, map[string]bool{"is_investment": false}, cookie, csrf, origin), 200)
	get("/api/accounts", &after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("removal changed account or balance")
	}
	for i, kind := range []string{" INVERSION ", "no inversión"} {
		w := request("POST", "/api/accounts", map[string]string{"name": fmt.Sprintf("Investment test %d", i), "type": kind, "currency": "USD", "opening_balance": "12.34"}, cookie, csrf, origin)
		check(w, 201)
		var created struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		verify := func(want bool) {
			t.Helper()
			get("/api/accounts", &after)
			for _, a := range after {
				if a.ID == created.ID {
					if a.IsInvestment != want || a.Balance != "12.34" {
						t.Fatal("incorrect persisted classification")
					}
					return
				}
			}
			t.Fatal("account missing")
		}
		verify(i == 0)
		if i == 0 {
			check(request("PUT", fmt.Sprintf("/api/accounts/%d/investment", created.ID), map[string]bool{"is_investment": false}, cookie, csrf, origin), 200)
			check(request("PUT", fmt.Sprintf("/api/accounts/%d", created.ID), map[string]string{"name": "Investment test 0", "type": "inversión", "currency": "USD", "opening_balance": "12.34"}, cookie, csrf, origin), 200)
			verify(false)
		} else {
			check(request("PUT", fmt.Sprintf("/api/accounts/%d/investment", created.ID), body, cookie, csrf, origin), 200)
			check(request("PUT", fmt.Sprintf("/api/accounts/%d", created.ID), map[string]string{"name": "Investment test 1", "type": "Banco", "currency": "USD", "opening_balance": "12.34"}, cookie, csrf, origin), 200)
			verify(true)
		}
	}
}
