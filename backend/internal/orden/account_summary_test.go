package orden

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testAccountSummary(t *testing.T, db *pgxpool.Pool, s *Server, request func(string, string, any, *http.Cookie, string, string) *httptest.ResponseRecorder, cookie *http.Cookie, csrf, origin string, clp, usd int64) {
	t.Helper()
	check := func(w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("want %d got %d: %s", want, w.Code, w.Body.String())
		}
	}
	var spare int64
	if err := db.QueryRow(context.Background(), "INSERT INTO accounts(user_id,name,type,currency,opening_balance) VALUES(1,'Synthetic summary spare','Personal','CLP',100) RETURNING id").Scan(&spare); err != nil {
		t.Fatal(err)
	}
	path := func(id int64) string { return fmt.Sprintf("/api/accounts/%d/summary", id) }
	body := map[string]bool{"show_in_summary": true}
	set := func(id int64, value bool) *httptest.ResponseRecorder {
		return request("PUT", path(id), map[string]bool{"show_in_summary": value}, cookie, csrf, origin)
	}
	accounts := func() []Account {
		t.Helper()
		w := request("GET", "/api/accounts", nil, cookie, "", "")
		check(w, 200)
		var items []Account
		if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		return items
	}
	before := accounts()
	movements := request("GET", "/api/movements", nil, cookie, "", "").Body.String()
	overview := request("GET", "/api/overview?currency=CLP&month=2026-10", nil, cookie, "", "").Body.String()
	for _, a := range before {
		if a.ShowInSummary {
			t.Fatal("selection should initially be empty")
		}
	}
	check(request("PUT", path(clp), body, nil, "", origin), 401)
	check(request("PUT", path(clp), body, cookie, "", origin), 403)
	check(request("PUT", path(clp), body, cookie, csrf, "https://evil.example"), 403)
	for _, id := range []string{"0", "-1", "bad", "9223372036854775808"} {
		check(request("PUT", "/api/accounts/"+id+"/summary", body, cookie, csrf, origin), 400)
	}
	for _, invalid := range []any{map[string]any{}, map[string]any{"show_in_summary": nil}, map[string]any{"show_in_summary": "true"}, map[string]any{"show_in_summary": true, "user_id": 2}} {
		check(request("PUT", path(clp), invalid, cookie, csrf, origin), 400)
	}
	check(set(clp, true), 200)
	check(set(clp, true), 200)
	check(set(usd, true), 200)
	check(set(spare, true), 200)
	check(set(9223372036854775807, true), 404)
	for _, a := range accounts() {
		if a.ShowInSummary != (a.ID == spare || a.ID == usd) {
			t.Fatal("selection/currency isolation failed")
		}
	}
	// A missing or foreign target must not clear an existing selection.
	r := httptest.NewRequest("PUT", path(spare), strings.NewReader(`{"show_in_summary":true}`))
	r.SetPathValue("id", fmt.Sprint(spare))
	r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, session{UserID: 2}))
	w := httptest.NewRecorder()
	s.setAccountSummary(w, r)
	check(w, 404)
	// Editing a selected account into a currency with another selection must fail atomically.
	check(request("PUT", fmt.Sprintf("/api/accounts/%d", spare), map[string]string{"name": "Synthetic summary spare", "type": "Personal", "currency": "USD", "opening_balance": "100"}, cookie, csrf, origin), 409)
	var wg sync.WaitGroup
	statuses := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := clp
			if i%2 == 0 {
				id = spare
			}
			statuses <- set(id, true).Code
		}(i)
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatalf("concurrent selection returned %d", status)
		}
	}
	count := 0
	for _, a := range accounts() {
		if a.Currency == "CLP" && a.ShowInSummary {
			count++
		}
	}
	if count != 1 {
		t.Fatal("concurrent selections must leave exactly one selected account")
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	count = 0
	for _, a := range accounts() {
		if a.Currency == "CLP" && a.ShowInSummary {
			count++
		}
	}
	if count != 1 {
		t.Fatal("migration rerun reset preference")
	}
	check(set(clp, false), 200)
	check(set(spare, false), 200)
	check(set(usd, false), 200)
	if !reflect.DeepEqual(before, accounts()) {
		t.Fatal("selection changed account fields or balances")
	}
	if movements != request("GET", "/api/movements", nil, cookie, "", "").Body.String() || overview != request("GET", "/api/overview?currency=CLP&month=2026-10", nil, cookie, "", "").Body.String() {
		t.Fatal("selection changed financial records or totals")
	}
	// Deleting a featured account needs no dangling preference cleanup.
	check(set(spare, true), 200)
	check(request("DELETE", fmt.Sprintf("/api/accounts/%d", spare), nil, cookie, csrf, origin), 200)
}
