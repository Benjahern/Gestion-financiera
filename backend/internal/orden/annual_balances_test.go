package orden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAnnualYear(t *testing.T) {
	for _, value := range []string{"1899", "2026", "2027", "25", "2025x", "+2025", "0001"} {
		if _, ok := validAnnualYear(value, 2026); ok {
			t.Fatalf("accepted invalid year %q", value)
		}
	}
	if y, ok := validAnnualYear("2025", 2026); !ok || y != 2025 {
		t.Fatal("valid closing rejected")
	}
}

func TestAnnualBalancesIntegration(t *testing.T) {
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
	schema := fmt.Sprintf("annual_test_%d", time.Now().UnixNano())
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
	var id int64
	if err = db.QueryRow(ctx, "INSERT INTO accounts(user_id,name,type,currency,opening_balance,is_investment) VALUES(1,'Synthetic fund','Inversión','CLP',1000,true) RETURNING id").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO movements(user_id,account_id,title,category,date,amount,kind) VALUES(1,$1,'Synthetic expense','Test','2026-01-01',100,'expense')", id); err != nil {
		t.Fatal(err)
	}
	s := NewServer(db, Config{OwnerEmail: testOwnerEmail, Origin: "https://annual.example.invalid"})
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
	path := "/api/annual-balances/" + strconv.Itoa(time.Now().UTC().Year()-1)
	body := map[string]any{"currency": "CLP", "balance": "1000"}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		p := path
		if method == "GET" {
			p = "/api/annual-balances?currency=CLP"
		}
		check(request(method, p, body, false, false), 401)
	}
	check(request("PUT", path, body, true, false), 403)
	check(request("DELETE", path+"?currency=CLP", nil, true, false), 403)
	check(request("GET", "/api/annual-balances?currency=EUR", nil, true, false), 400)
	empty := request("GET", "/api/annual-balances?currency=CLP", nil, true, false)
	check(empty, 200)
	var list struct {
		Currency    string          `json:"currency"`
		CurrentYear int             `json:"current_year"`
		Items       []annualBalance `json:"items"`
	}
	if err = json.Unmarshal(empty.Body.Bytes(), &list); err != nil || list.CurrentYear != time.Now().UTC().Year() || len(list.Items) != 0 {
		t.Fatal("invalid empty history", err)
	}
	before := request("GET", "/api/accounts", nil, true, false).Body.String()
	check(request("PUT", path, body, true, true), 200)
	body["balance"] = "1200"
	check(request("PUT", path, body, true, true), 200)
	body["currency"] = "USD"
	body["balance"] = "-0.01"
	saved := request("PUT", path, body, true, true)
	check(saved, 200)
	var closing annualBalance
	if err = json.Unmarshal(saved.Body.Bytes(), &closing); err != nil || closing.Balance != "-0.01" {
		t.Fatal("USD precision lost", err)
	}
	for _, invalid := range []map[string]any{{"currency": "CLP", "balance": "0.01"}, {"currency": "USD", "balance": "1.001"}, {"currency": "USD", "balance": "1e3"}, {"currency": "EUR", "balance": "1"}, {"currency": "CLP", "balance": 1}, {"currency": "CLP", "balance": "1", "user_id": 2}} {
		check(request("PUT", path, invalid, true, true), 400)
	}
	check(request("PUT", "/api/annual-balances/"+strconv.Itoa(time.Now().UTC().Year()), body, true, true), 400)
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("migration re-run", err)
	}
	handler = NewServer(db, s.config).Handler()
	loaded := request("GET", "/api/annual-balances?currency=CLP", nil, true, false)
	check(loaded, 200)
	if err = json.Unmarshal(loaded.Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.Items[0].Balance != "1200.00" {
		t.Fatal("upsert or persistence failed", err)
	}
	check(request("DELETE", path+"?currency=CLP", nil, true, true), 200)
	check(request("DELETE", path+"?currency=CLP", nil, true, true), 404)
	usd := request("GET", "/api/annual-balances?currency=USD", nil, true, false)
	check(usd, 200)
	if err = json.Unmarshal(usd.Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.Items[0].Balance != "-0.01" {
		t.Fatal("currencies not isolated", err)
	}
	after := request("GET", "/api/accounts", nil, true, false).Body.String()
	if after != before {
		t.Fatal("closing changed account balances")
	}
	var count int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM movements").Scan(&count); err != nil || count != 1 {
		t.Fatal("closing changed movements", err)
	}
}
