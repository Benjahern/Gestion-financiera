package orden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func calculatorFixture() calculatorSettings {
	return calculatorSettings{Currency: "CLP", ReferenceIncome: "1000", General: "500", Wants: "100", Investment: "400", IndexFund: "250", Reserve: "150", EmergencyGoal: "100", EmergencyBalance: "99", OwnHomePercent: "50", RentalPercent: "30", TravelPercent: "20"}
}
func TestCalculatorValidation(t *testing.T) {
	c := calculatorFixture()
	if !validCalculator(c) {
		t.Fatal("synthetic valid configuration rejected")
	}
	for _, mutate := range []func(*calculatorSettings){
		func(c *calculatorSettings) { c.General = "501" }, func(c *calculatorSettings) { c.Reserve = "149" },
		func(c *calculatorSettings) { c.EmergencyGoal = "0" }, func(c *calculatorSettings) { c.Currency = "EUR" },
		func(c *calculatorSettings) { c.EmergencyBalance = "-1" }, func(c *calculatorSettings) { c.General = "0.01" },
		func(c *calculatorSettings) { c.OwnHomePercent = "101" }, func(c *calculatorSettings) { c.TravelPercent = "19.99" },
	} {
		invalid := c
		mutate(&invalid)
		if validCalculator(invalid) {
			t.Fatal("invalid configuration accepted")
		}
	}
	if u, ok := planningUnits("999999999999.99", "USD"); !ok || u != 99999999999999 {
		t.Fatal("decimal precision lost")
	}
	c.Currency = "USD"
	c.EmergencyBalance = "0.01"
	if !validCalculator(c) {
		t.Fatal("USD cents rejected")
	}
}

func TestCalculatorPersistenceIntegration(t *testing.T) {
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
	schema := fmt.Sprintf("calculator_test_%d", time.Now().UnixNano())
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
		t.Fatal("migration re-run", err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO users(id,email,password_hash) VALUES(1,$1,'unused-test-hash')", testOwnerEmail); err != nil {
		t.Fatal(err)
	}
	token, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,1,'test-csrf',now()+interval '1 hour')", tokenHash(token)); err != nil {
		t.Fatal(err)
	}
	var account int64
	if err = db.QueryRow(ctx, "INSERT INTO accounts(user_id,name,type,currency,opening_balance) VALUES(1,'Synthetic savings','Personal','CLP',1234) RETURNING id").Scan(&account); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO movements(user_id,account_id,title,category,date,amount,kind) VALUES(1,$1,'Synthetic expense','Test','2026-01-01',34,'expense')", account); err != nil {
		t.Fatal(err)
	}
	s := NewServer(db, Config{OwnerEmail: testOwnerEmail, Origin: "https://calculator.example.invalid"})
	handler := s.Handler()
	request := func(method, path string, body any, auth, csrf bool) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&b).Encode(body)
		}
		r := httptest.NewRequest(method, path, &b)
		r.Header.Set("Origin", s.config.Origin)
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.AddCookie(&http.Cookie{Name: s.cookieName(), Value: token})
		}
		if csrf {
			r.Header.Set("X-CSRF-Token", "test-csrf")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("want %d got %d: %s", want, w.Code, w.Body.String())
		}
	}
	check(request("GET", "/api/calculator?currency=CLP", nil, false, false), 401)
	check(request("PUT", "/api/calculator", calculatorFixture(), true, false), 403)
	check(request("GET", "/api/calculator?currency=EUR", nil, true, false), 400)
	empty := request("GET", "/api/calculator?currency=USD", nil, true, false)
	check(empty, 200)
	if empty.Body.String() != "{\"settings\":null}\n" {
		t.Fatal("new configuration must be empty")
	}
	c := calculatorFixture()
	check(request("PUT", "/api/calculator", c, true, true), 200)
	// Recreate HTTP server to verify persistence is in PostgreSQL, not process state.
	handler = NewServer(db, s.config).Handler()
	loaded := request("GET", "/api/calculator?currency=CLP", nil, true, false)
	check(loaded, 200)
	var response struct {
		Settings calculatorSettings `json:"settings"`
	}
	if err = json.Unmarshal(loaded.Body.Bytes(), &response); err != nil || response.Settings != c {
		t.Fatal("configuration not persisted", err)
	}
	check(request("GET", "/api/calculator?currency=USD", nil, true, false), 200)
	c.General = "501"
	check(request("PUT", "/api/calculator", c, true, true), 400)
	var total, opening string
	var count int
	if err = db.QueryRow(ctx, "SELECT opening_balance::text FROM accounts WHERE id=$1", account).Scan(&opening); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, "SELECT count(*),sum(amount)::text FROM movements").Scan(&count, &total); err != nil {
		t.Fatal(err)
	}
	if opening != "1234.00" || count != 1 || total != "34.00" {
		t.Fatal("planning changed financial data")
	}
}
