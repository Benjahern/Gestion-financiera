package orden

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testMail() gmailMessage {
	m := gmailMessage{ID: "message-1", InternalDate: fmt.Sprint(time.Now().UnixMilli())}
	m.Payload.Mime = "text/html"
	m.Payload.Headers = []gmailHeader{{"From", "MACHBANK <contacto@mail.machbank.cl>"}, {"Subject", "Has hecho una compra con tu Tarjeta de Crédito MACHBANK"}, {"Authentication-Results", "mx.google.com; dkim=pass; dmarc=pass header.from=mail.machbank.cl; spf=pass"}}
	body := `<h1>Comprobante de compra con tu Tarjeta de Crédito MACHBANK</h1><table><tr><td>Comercio</td><td>NO DEBE GUARDARSE</td></tr><tr><td>Monto pagado</td><td>$4.000</td></tr><tr><td>Últimos 4 dígitos de la tarjeta</td><td>4100</td></tr><tr><td>Fecha y hora</td><td>10/10/2026 - 12:22</td></tr><tr><td>Nr. identificador de la compra</td><td>purchase123</td></tr></table>`
	m.Payload.Body.Data = base64.RawURLEncoding.EncodeToString([]byte(body))
	return m
}
func TestMACHParser(t *testing.T) {
	m := testMail()
	p, ok := parseMACH(m)
	if !ok || p.Amount != "4000" || p.LastFour != "4100" || p.Date != "2026-10-10" {
		t.Fatalf("unexpected purchase: %+v %v", p, ok)
	}
	for _, field := range []string{"From", "Subject", "Authentication-Results"} {
		bad := testMail()
		for i, h := range bad.Payload.Headers {
			if h.Name == field {
				bad.Payload.Headers[i].Value = "invalid"
			}
		}
		if _, ok = parseMACH(bad); ok {
			t.Fatal("accepted unauthenticated/non-purchase mail", field)
		}
	}
	bad := testMail()
	text, _ := base64.RawURLEncoding.DecodeString(bad.Payload.Body.Data)
	bad.Payload.Body.Data = base64.RawURLEncoding.EncodeToString([]byte(strings.ReplaceAll(string(text), "$4.000", "US$4.000 USD")))
	if _, ok = parseMACH(bad); ok {
		t.Fatal("accepted USD")
	}
	bad = testMail()
	text, _ = base64.RawURLEncoding.DecodeString(bad.Payload.Body.Data)
	bad.Payload.Body.Data = base64.RawURLEncoding.EncodeToString(append(text, []byte(" Monto pagado $5.000 ")...))
	if _, ok = parseMACH(bad); ok {
		t.Fatal("accepted ambiguous amount")
	}
}
func TestClassifier(t *testing.T) {
	if p := classify("viaje", nil); p.Category != "" || p.Automatic {
		t.Fatal(p)
	}
	examples := []Example{}
	for i := 0; i < 30; i++ {
		examples = append(examples, Example{"pasaje micro bus", "Transporte > Micro"}, Example{"pan palta almuerzo", "Vida diaria > Comida"})
	}
	p := classify("pasaje de micro", examples)
	if p.Category != "Transporte > Micro" || !p.Automatic {
		t.Fatal(p)
	}
	if p = classify("palta", examples); p.Category != "Vida diaria > Comida" {
		t.Fatal(p)
	}
	if p = classify("viaje desconocido", examples); p.Category != "" || p.Automatic {
		t.Fatal(p)
	}
	if p = classify("micro", examples[:1]); p.Automatic {
		t.Fatal("single class is not enough")
	}
	if p = classify("micro pan", examples); p.Automatic {
		t.Fatal("ambiguous prediction")
	}
}
func TestTokenEncryption(t *testing.T) {
	c := Config{TokenEncryptionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))}
	data := []byte("private-refresh-token")
	encrypted, err := c.seal(data)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := c.unseal(encrypted)
	if err != nil || string(plain) != string(data) {
		t.Fatal(err)
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, err = c.unseal(encrypted); err == nil {
		t.Fatal("accepted tampered ciphertext")
	}
}
func BenchmarkClassifier(b *testing.B) {
	examples := make([]Example, 10000)
	for i := range examples {
		if i%2 == 0 {
			examples[i] = Example{"pasaje micro trabajo", "Transporte"}
		} else {
			examples[i] = Example{"pan palta almuerzo", "Comida"}
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classify("pasaje micro", examples)
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestImportsIntegration(t *testing.T) {
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
	schema := fmt.Sprintf("orden_imports_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE") }()
	cfg, _ := pgxpool.ParseConfig(dsn)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO users(id,email,password_hash) VALUES(1,$1,'test-only-hash')", testOwnerEmail); err != nil {
		t.Fatal(err)
	}
	token, _ := randomToken()
	csrf, _ := randomToken()
	if _, err = db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,1,$2,now()+interval '1 day')", tokenHash(token), csrf); err != nil {
		t.Fatal(err)
	}
	s := NewServer(db, Config{OwnerEmail: testOwnerEmail, Origin: "http://localhost:8088", GoogleClientID: "test-client", GoogleClientSecret: "test-secret", TokenEncryptionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))})
	handler := s.Handler()
	call := func(method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, strings.NewReader(string(data)))
		r.Header.Set("Origin", s.config.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: s.cookieName(), Value: token})
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("want %d got %d %s", status, w.Code, w.Body.String())
		}
	}
	check(call("GET", "/api/categories", nil), 200)
	var account int64
	if err = db.QueryRow(ctx, "INSERT INTO accounts(user_id,name,type,currency) VALUES(1,'MACH','Tarjeta','CLP') RETURNING id").Scan(&account); err != nil {
		t.Fatal(err)
	}
	check(call("POST", "/api/gmail/rules", map[string]any{"bank": "mach", "last_four": "4100", "account_id": account}), 200)
	check(call("POST", "/api/gmail/rules", map[string]any{"bank": "bci", "last_four": "4100", "account_id": account}), 400)
	original := googleHTTP
	defer func() { googleHTTP = original }()
	calls := 0
	recoveryMode := false
	recoveryMessage := testMail()
	recoveryMessage.InternalDate = fmt.Sprint(time.Now().Add(-24 * time.Hour).UnixMilli())
	recoveryBody, _ := base64.RawURLEncoding.DecodeString(recoveryMessage.Payload.Body.Data)
	recoveryMessage.Payload.Body.Data = base64.RawURLEncoding.EncodeToString([]byte(strings.ReplaceAll(string(recoveryBody), "purchase123", "synthetic-recovery-456")))
	for i := range recoveryMessage.Payload.Headers {
		if recoveryMessage.Payload.Headers[i].Name == "Authentication-Results" {
			recoveryMessage.Payload.Headers[i].Value = "mx.google.com; dkim=pass header.i=@mail.machbank.cl; dmarc=pass (p=NONE) header.from=machbank.cl"
		}
	}
	googleHTTP = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") == "authorization_code" && r.Form.Get("code_verifier") == "" {
				t.Fatal("missing PKCE")
			}
			writeJSON(w, 200, map[string]any{"access_token": "access-test", "refresh_token": "refresh-test", "expires_in": 3600, "scope": gmailScope})
		case "/gmail/v1/users/me/profile":
			writeJSON(w, 200, map[string]string{"emailAddress": testOwnerEmail})
		case "/gmail/v1/users/me/messages":
			if !strings.Contains(r.URL.Query().Get("q"), "from:contacto@mail.machbank.cl") {
				t.Fatal("unfiltered mailbox")
			}
			if recoveryMode {
				var after int64
				for _, field := range strings.Fields(r.URL.Query().Get("q")) {
					if strings.HasPrefix(field, "after:") {
						after, _ = strconv.ParseInt(strings.TrimPrefix(field, "after:"), 10, 64)
					}
				}
				var connected time.Time
				if err := db.QueryRow(ctx, "SELECT connected_at FROM gmail_connections").Scan(&connected); err != nil || after < connected.Unix() {
					t.Fatal("rescan must not search before consent", err)
				}
				if r.URL.Query().Get("maxResults") != "5" {
					t.Fatal("unbounded rescan")
				}
				received, _ := strconv.ParseInt(recoveryMessage.InternalDate, 10, 64)
				if after*1000 < received && r.URL.Query().Get("pageToken") == "" {
					writeJSON(w, 200, map[string]any{"messages": []map[string]string{{"id": "recovered-id"}, {"id": "before-consent-id"}}, "nextPageToken": "recovery-page-two"})
				} else {
					writeJSON(w, 200, map[string]any{"messages": []any{}})
				}
			} else {
				writeJSON(w, 200, map[string]any{"messages": []map[string]string{{"id": "message-1"}, {"id": "duplicate-id"}}})
			}
		case "/gmail/v1/users/me/messages/recovered-id":
			writeJSON(w, 200, recoveryMessage)
		case "/gmail/v1/users/me/messages/before-consent-id":
			older := recoveryMessage
			older.InternalDate = fmt.Sprint(time.Now().Add(-72 * time.Hour).UnixMilli())
			writeJSON(w, 200, older)
		case "/gmail/v1/users/me/messages/message-1", "/gmail/v1/users/me/messages/duplicate-id":
			writeJSON(w, 200, testMail())
		case "/gmail/v1/users/me/messages/gone-id":
			writeJSON(w, 404, map[string]string{"error": "deleted fixture"})
		case "/revoke":
			writeJSON(w, 200, map[string]bool{"ok": true})
		default:
			t.Fatalf("unexpected request %s", r.URL)
		}
		return w.Result(), nil
	})}
	start := call("POST", "/api/gmail/connect", nil)
	check(start, 200)
	var authURL struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(start.Body.Bytes(), &authURL)
	u, _ := url.Parse(authURL.URL)
	state := u.Query().Get("state")
	if u.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("PKCE missing")
	}
	cookie := start.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe oauth cookie")
	}
	callback := call("GET", "/api/gmail/callback?state="+state+"&code=test-code", nil, cookie)
	check(callback, 303)
	if !strings.Contains(callback.Header().Get("Location"), "connected") {
		t.Fatal(callback.Header())
	}
	before := calls
	replay := call("GET", "/api/gmail/callback?state="+state+"&code=test-code", nil, cookie)
	check(replay, 303)
	if calls != before {
		t.Fatal("OAuth state replay")
	}
	// Encrypted OAuth tokens must never appear in API status.
	status := call("GET", "/api/gmail", nil)
	check(status, 200)
	if strings.Contains(status.Body.String(), "access-test") || strings.Contains(status.Body.String(), "refresh-test") {
		t.Fatal("token leaked")
	}
	// An unmapped purchase is queued, not allowed to stall the incremental cursor.
	_, _ = db.Exec(ctx, "DELETE FROM gmail_card_rules")
	check(call("POST", "/api/gmail/sync", nil), 200)
	var queued int
	var scanAfter *time.Time
	if err = db.QueryRow(ctx, "SELECT count(*) FROM gmail_pending_messages").Scan(&queued); err != nil || queued != 2 {
		t.Fatal("unmapped queue", queued, err)
	}
	if err = db.QueryRow(ctx, "SELECT scan_after FROM gmail_connections").Scan(&scanAfter); err != nil || scanAfter == nil {
		t.Fatal("incremental cursor did not advance", err)
	}
	check(call("POST", "/api/gmail/rules", map[string]any{"bank": "mach", "last_four": "4100", "account_id": account}), 200)
	_, _ = db.Exec(ctx, "UPDATE gmail_connections SET last_sync=NULL")
	check(call("POST", "/api/gmail/sync", nil), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM gmail_pending_messages").Scan(&queued); err != nil || queued != 0 {
		t.Fatal("mapped queue not drained", queued, err)
	}
	var count int
	var movement int64
	var title, category string
	var pending bool
	if err = db.QueryRow(ctx, "SELECT count(*) FROM movements").Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate import", count, err)
	}
	if err = db.QueryRow(ctx, "SELECT id,title,category,pending FROM movements").Scan(&movement, &title, &category, &pending); err != nil || title != "Pendiente de completar" || category != "Sin categoría" || !pending {
		t.Fatal("not pending", err, title)
	}
	overviewResponse := call("GET", "/api/overview?currency=CLP&month=2026-10", nil)
	check(overviewResponse, 200)
	var overview struct {
		Expense string `json:"expense"`
	}
	if err = json.Unmarshal(overviewResponse.Body.Bytes(), &overview); err != nil || overview.Expense != "4000.00" {
		t.Fatal("pending expense excluded from totals", err, overview)
	}
	body := map[string]any{"account_id": account, "title": "Palta y pan", "category": "Vida diaria > Comida", "date": "2026-10-10", "amount": "4000", "kind": "expense", "category_confirmed": true}
	check(call("PUT", fmt.Sprintf("/api/movements/%d", movement), body), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM training_examples").Scan(&count); err != nil || count != 1 {
		t.Fatal("confirmed example missing", err)
	}
	body["category"] = "Transporte > Micro"
	body["title"] = "Pasaje micro"
	check(call("PUT", fmt.Sprintf("/api/movements/%d", movement), body), 200)
	var description string
	if err = db.QueryRow(ctx, "SELECT description FROM training_examples").Scan(&description); err != nil || description != "Pasaje micro" {
		t.Fatal("correction not replacing example", err)
	}
	predicted := call("POST", "/api/classifier/predict", map[string]string{"description": "micro"})
	check(predicted, 200)
	var prediction Prediction
	if err = json.Unmarshal(predicted.Body.Bytes(), &prediction); err != nil || prediction.Category != "Transporte > Micro" || prediction.Automatic {
		t.Fatal("corrected prediction", err, prediction)
	}
	var categoryID int64
	if err = db.QueryRow(ctx, "SELECT id FROM expense_categories WHERE name='Transporte > Micro'").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	check(call("DELETE", fmt.Sprintf("/api/categories/%d", categoryID), nil), 409)
	check(call("PUT", fmt.Sprintf("/api/categories/%d", categoryID), map[string]string{"name": "Transporte > Bus"}), 200)
	if err = db.QueryRow(ctx, "SELECT category FROM movements WHERE id=$1", movement).Scan(&category); err != nil || category != "Transporte > Bus" {
		t.Fatal("category rename", err, category)
	}
	body["category_confirmed"] = false
	body["category"] = "Sin categoría"
	check(call("PUT", fmt.Sprintf("/api/movements/%d", movement), body), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM training_examples").Scan(&count); err != nil || count != 0 {
		t.Fatal("unconfirmed prediction trained", err)
	}
	if err = db.QueryRow(ctx, "SELECT pending FROM movements WHERE id=$1", movement).Scan(&pending); err != nil || !pending {
		t.Fatal("uncategorized expense is no longer pending", err)
	}
	check(call("DELETE", fmt.Sprintf("/api/movements/%d", movement), nil), 200)
	_, _ = db.Exec(ctx, "UPDATE gmail_connections SET last_sync=NULL")
	check(call("POST", "/api/gmail/sync", nil), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM movements").Scan(&count); err != nil || count != 0 {
		t.Fatal("deleted purchase reimported", err)
	}
	_, _ = db.Exec(ctx, "INSERT INTO gmail_pending_messages(user_id,message_id) VALUES(1,'gone-id')")
	_, _ = db.Exec(ctx, "UPDATE gmail_connections SET last_sync=NULL")
	check(call("POST", "/api/gmail/sync", nil), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM gmail_pending_messages").Scan(&queued); err != nil || queued != 0 {
		t.Fatal("deleted retry stalls cursor", queued, err)
	}
	// A previously rejected mail is outside the incremental overlap. Explicit
	// recovery revisits it after consent, paginates, and never duplicates money.
	recoveryMode = true
	_, err = db.Exec(ctx, "UPDATE gmail_connections SET connected_at=now()-interval '2 days',scan_after=now(),scan_until=NULL,page_token='',last_sync=NULL")
	if err != nil {
		t.Fatal(err)
	}
	check(call("POST", "/api/gmail/sync", nil), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM movements").Scan(&count); err != nil || count != 0 {
		t.Fatal("incremental fixture should not find older mail", count, err)
	}
	_, _ = db.Exec(ctx, "UPDATE gmail_connections SET last_sync=NULL")
	recovered := call("POST", "/api/gmail/rescan", nil)
	check(recovered, 200)
	var result struct {
		Imported int `json:"imported"`
	}
	if err = json.Unmarshal(recovered.Body.Bytes(), &result); err != nil || result.Imported != 1 {
		t.Fatal("expected one recovered purchase", err, recovered.Body.String())
	}
	check(call("POST", "/api/gmail/rescan", nil), 503) // Throttle also applies to recovery.
	var pageToken string
	if err = db.QueryRow(ctx, "SELECT page_token FROM gmail_connections").Scan(&pageToken); err != nil || pageToken != "recovery-page-two" {
		t.Fatal("recovery pagination not saved", pageToken, err)
	}
	_, _ = db.Exec(ctx, "UPDATE gmail_connections SET last_sync=NULL")
	check(call("POST", "/api/gmail/sync", nil), 200)
	if err = db.QueryRow(ctx, "SELECT page_token FROM gmail_connections").Scan(&pageToken); err != nil || pageToken != "" {
		t.Fatal("normal polling did not finish recovery page", pageToken, err)
	}
	_, _ = db.Exec(ctx, "UPDATE gmail_connections SET last_sync=NULL")
	check(call("POST", "/api/gmail/rescan", nil), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM movements").Scan(&count); err != nil || count != 1 {
		t.Fatal("rescan duplicated a purchase or imported pre-consent mail", count, err)
	}
	for _, withSession := range []bool{false, true} {
		r := httptest.NewRequest("POST", "/api/gmail/rescan", nil)
		r.Header.Set("Origin", s.config.Origin)
		if withSession {
			r.AddCookie(&http.Cookie{Name: s.cookieName(), Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatal("recovery accepted without session/CSRF")
		}
	}
	check(call("DELETE", "/api/gmail", nil), 200)
	if err = db.QueryRow(ctx, "SELECT count(*) FROM gmail_connections").Scan(&count); err != nil || count != 0 {
		t.Fatal("not disconnected", err)
	}
	check(call("POST", "/api/gmail/sync", nil), 503)
}
