package orden

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const gmailScope = "https://www.googleapis.com/auth/gmail.readonly"
const googleTokenURL = "https://oauth2.googleapis.com/token"
const gmailBase = "https://gmail.googleapis.com/gmail/v1/users/me"

var googleHTTP = &http.Client{Timeout: 7 * time.Second}

type gmailAPIError struct{ status int }

func (e *gmailAPIError) Error() string {
	return fmt.Sprintf("Gmail no disponible (%d); revisa la conexión", e.status)
}

type gmailTokens struct {
	Access  string    `json:"access_token"`
	Refresh string    `json:"refresh_token"`
	Expiry  time.Time `json:"expiry"`
}

func (c Config) gmailEnabled() bool {
	key, err := base64.StdEncoding.DecodeString(c.TokenEncryptionKey)
	return err == nil && len(key) == 32 && c.GoogleClientID != "" && c.GoogleClientSecret != ""
}
func (c Config) seal(data []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(c.TokenEncryptionKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, data, []byte("orden-gmail-v1")), nil
}
func (c Config) unseal(data []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(c.TokenEncryptionKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < g.NonceSize() {
		return nil, errors.New("invalid credentials")
	}
	return g.Open(nil, data[:g.NonceSize()], data[g.NonceSize():], []byte("orden-gmail-v1"))
}
func (s *Server) importRoutes(mux *http.ServeMux) {
	for pattern, handler := range map[string]http.HandlerFunc{
		"GET /api/categories": s.categories, "POST /api/categories": s.saveCategory, "PUT /api/categories/{id}": s.saveCategory, "DELETE /api/categories/{id}": s.deleteCategory,
		"POST /api/classifier/predict": s.prediction, "GET /api/gmail": s.gmailStatus, "POST /api/gmail/connect": s.gmailConnect,
		"DELETE /api/gmail": s.gmailDisconnect, "POST /api/gmail/sync": s.gmailSync, "POST /api/gmail/rules": s.gmailRule, "DELETE /api/gmail/rules/{id}": s.deleteGmailRule,
	} {
		mux.Handle(pattern, s.authenticate(handler))
	}
	mux.HandleFunc("GET /api/gmail/callback", s.gmailCallback)
}
func (s *Server) oauthCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: "orden_gmail_oauth", Value: value, Path: "/api/gmail/callback", HttpOnly: true, Secure: s.config.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}
func (s *Server) gmailConnect(w http.ResponseWriter, r *http.Request) {
	if !s.config.gmailEnabled() {
		fail(w, 503, "Configura las credenciales OAuth de Google en el backend")
		return
	}
	state, err := randomToken()
	if err != nil {
		fail(w, 503, "No se pudo iniciar la conexión")
		return
	}
	verifier, err := randomToken()
	if err != nil {
		fail(w, 503, "No se pudo iniciar la conexión")
		return
	}
	auth := r.Context().Value(sessionKey{}).(session)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var generation int64
	if err = tx.QueryRow(r.Context(), "UPDATE users SET gmail_generation=gmail_generation+1 WHERE id=$1 RETURNING gmail_generation", auth.UserID).Scan(&generation); err != nil {
		databaseError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM gmail_oauth_states WHERE user_id=$1 OR expires_at<now()", auth.UserID); err == nil {
		_, err = tx.Exec(r.Context(), "INSERT INTO gmail_oauth_states(state_hash,user_id,session_hash,verifier,generation,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '10 minutes')", tokenHash(state), auth.UserID, auth.Hash, verifier, generation)
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		databaseError(w, err)
		return
	}
	hash := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {s.config.GoogleClientID}, "redirect_uri": {s.config.Origin + "/api/gmail/callback"}, "response_type": {"code"}, "scope": {gmailScope}, "state": {state}, "access_type": {"offline"}, "prompt": {"consent"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}, "code_challenge_method": {"S256"}}
	http.SetCookie(w, s.oauthCookie(state, 600))
	writeJSON(w, 200, map[string]string{"url": "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()})
}
func tokenRequest(ctx context.Context, values url.Values) (gmailTokens, error) {
	var t gmailTokens
	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return t, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := googleHTTP.Do(req)
	if err != nil {
		return t, errors.New("Google no disponible")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return t, errors.New("Vuelve a conectar Gmail: autorización no disponible")
	}
	var body struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Expires int    `json:"expires_in"`
		Scope   string `json:"scope"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(&body); err != nil {
		return t, err
	}
	if body.Access == "" || body.Expires <= 0 {
		return t, errors.New("respuesta OAuth inválida")
	}
	if body.Scope != "" && !strings.Contains(" "+body.Scope+" ", " "+gmailScope+" ") {
		return t, errors.New("Falta permiso de lectura de Gmail")
	}
	return gmailTokens{body.Access, body.Refresh, time.Now().Add(time.Duration(body.Expires) * time.Second)}, nil
}
func gmailGet(ctx context.Context, access, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", gmailBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := googleHTTP.Do(req)
	if err != nil {
		return errors.New("No se pudo consultar Gmail")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return &gmailAPIError{status: resp.StatusCode}
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(target)
}
func (s *Server) gmailCallback(w http.ResponseWriter, r *http.Request) {
	redirect := func(status string) {
		http.SetCookie(w, s.oauthCookie("", -1))
		http.Redirect(w, r, s.config.Origin+"/?gmail="+status, http.StatusSeeOther)
	}
	cookie, err := r.Cookie("orden_gmail_oauth")
	state := r.URL.Query().Get("state")
	if err != nil || len(state) != 43 || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		redirect("error")
		return
	}
	// Consume state atomically and bind it to a still-valid app session. The Lax
	// OAuth cookie works on Google's redirect even though the app cookie is Strict.
	var user int64
	var generation int64
	var sessionHash []byte
	var verifier string
	err = s.db.QueryRow(r.Context(), `DELETE FROM gmail_oauth_states o USING sessions s WHERE o.state_hash=$1 AND o.session_hash=s.token_hash AND s.expires_at>now() AND o.expires_at>now() RETURNING o.user_id,o.verifier,o.generation,o.session_hash`, tokenHash(state)).Scan(&user, &verifier, &generation, &sessionHash)
	if err != nil || !s.config.gmailEnabled() || r.URL.Query().Get("error") != "" {
		redirect("error")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > 4096 {
		redirect("error")
		return
	}
	tokens, err := tokenRequest(r.Context(), url.Values{"code": {code}, "client_id": {s.config.GoogleClientID}, "client_secret": {s.config.GoogleClientSecret}, "redirect_uri": {s.config.Origin + "/api/gmail/callback"}, "grant_type": {"authorization_code"}, "code_verifier": {verifier}})
	if err != nil || tokens.Refresh == "" {
		redirect("error")
		return
	}
	var profile struct {
		Email string `json:"emailAddress"`
	}
	if err = gmailGet(r.Context(), tokens.Access, "/profile", &profile); err != nil || profile.Email == "" {
		redirect("error")
		return
	}
	data, _ := json.Marshal(tokens)
	encrypted, err := s.config.seal(data)
	if err != nil {
		redirect("error")
		return
	}
	// Reconnecting preserves the import start date for this mailbox.
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		redirect("error")
		return
	}
	defer tx.Rollback(r.Context())
	var currentGeneration int64
	var validSession bool
	if err = tx.QueryRow(r.Context(), "SELECT gmail_generation FROM users WHERE id=$1 FOR UPDATE", user).Scan(&currentGeneration); err != nil || generation != currentGeneration {
		redirect("error")
		return
	}
	if err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1 AND user_id=$2 AND expires_at>now())", sessionHash, user).Scan(&validSession); err != nil || !validSession {
		redirect("error")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO gmail_connections(user_id,email,credentials) VALUES($1,$2,$3) ON CONFLICT(user_id) DO UPDATE SET email=excluded.email,credentials=excluded.credentials,last_error='',page_token='',scan_after=NULL,scan_until=NULL,connected_at=CASE WHEN gmail_connections.email=excluded.email THEN gmail_connections.connected_at ELSE now() END`, user, strings.ToLower(profile.Email), encrypted)
	if err != nil {
		redirect("error")
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM gmail_pending_messages WHERE user_id=$1", user); err != nil {
		redirect("error")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		redirect("error")
		return
	}
	redirect("connected")
}
func (s *Server) gmailStatus(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	result := map[string]any{"configured": s.config.gmailEnabled(), "connected": false, "rules": []any{}, "supported_banks": []string{"mach"}}
	var email, message string
	var last *time.Time
	err := s.db.QueryRow(r.Context(), "SELECT email,last_error,last_sync FROM gmail_connections WHERE user_id=$1", user).Scan(&email, &message, &last)
	if err == nil {
		result["connected"] = true
		result["email"] = email
		result["last_error"] = message
		result["last_sync"] = last
	} else if !errors.Is(err, pgx.ErrNoRows) {
		databaseError(w, err)
		return
	}
	rows, err := s.db.Query(r.Context(), "SELECT id,bank,last_four,account_id FROM gmail_card_rules WHERE user_id=$1 ORDER BY id", user)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	rules := []map[string]any{}
	for rows.Next() {
		var id, account int64
		var bank, lastFour string
		if err = rows.Scan(&id, &bank, &lastFour, &account); err != nil {
			databaseError(w, err)
			return
		}
		rules = append(rules, map[string]any{"id": id, "bank": bank, "last_four": lastFour, "account_id": account})
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	result["rules"] = rules
	writeJSON(w, 200, result)
}
func (s *Server) gmailDisconnect(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var encrypted []byte
	if _, err = tx.Exec(r.Context(), "UPDATE users SET gmail_generation=gmail_generation+1 WHERE id=$1", user); err != nil {
		databaseError(w, err)
		return
	}
	err = tx.QueryRow(r.Context(), "DELETE FROM gmail_connections WHERE user_id=$1 RETURNING credentials", user).Scan(&encrypted)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		databaseError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM gmail_oauth_states WHERE user_id=$1", user); err != nil {
		databaseError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM gmail_pending_messages WHERE user_id=$1", user); err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		databaseError(w, err)
		return
	}
	revoked := false
	if data, e := s.config.unseal(encrypted); e == nil {
		var t gmailTokens
		if json.Unmarshal(data, &t) == nil {
			req, _ := http.NewRequestWithContext(r.Context(), "POST", "https://oauth2.googleapis.com/revoke", strings.NewReader(url.Values{"token": {t.Refresh}}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if resp, e := googleHTTP.Do(req); e == nil {
				revoked = resp.StatusCode == 200
				resp.Body.Close()
			}
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "revoked": revoked})
}
func (s *Server) gmailRule(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Bank      string `json:"bank"`
		LastFour  string `json:"last_four"`
		AccountID int64  `json:"account_id"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Bank != "mach" {
		fail(w, 400, "Bci requiere un correo de ejemplo antes de habilitar su lector")
		return
	}
	if len(b.LastFour) != 4 || strings.Trim(b.LastFour, "0123456789") != "" {
		fail(w, 400, "Introduce los últimos cuatro dígitos")
		return
	}
	tag, err := s.db.Exec(r.Context(), `INSERT INTO gmail_card_rules(user_id,bank,last_four,account_id) SELECT $1,$2,$3,id FROM accounts WHERE id=$4 AND user_id=$1 AND currency='CLP' ON CONFLICT(user_id,bank,last_four) DO UPDATE SET account_id=excluded.account_id`, currentUser(r), b.Bank, b.LastFour, b.AccountID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 400, "Selecciona una cuenta CLP propia")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Server) deleteGmailRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	_, err := s.db.Exec(r.Context(), "DELETE FROM gmail_card_rules WHERE user_id=$1 AND id=$2", currentUser(r), id)
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Server) gmailSync(w http.ResponseWriter, r *http.Request) {
	n, err := s.syncGmail(r.Context(), currentUser(r))
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"imported": n})
}
func (s *Server) RunGmail(ctx context.Context) {
	if !s.config.gmailEnabled() {
		return
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		s.pollGmail(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Server) pollGmail(ctx context.Context) {
	rows, err := s.db.Query(ctx, "SELECT user_id FROM gmail_connections")
	if err != nil {
		return
	}
	users := []int64{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			users = append(users, id)
		}
	}
	rows.Close()
	for _, user := range users {
		work, cancel := context.WithTimeout(ctx, 75*time.Second)
		_, err = s.syncGmail(work, user)
		cancel()
		if err != nil {
			_, _ = s.db.Exec(ctx, "UPDATE gmail_connections SET last_error=$1 WHERE user_id=$2", "No se pudo sincronizar; revisa la conexión y las reglas de tarjeta", user)
		}
	}
}
func (s *Server) syncGmail(ctx context.Context, user int64) (int, error) {
	if !s.config.gmailEnabled() {
		return 0, errors.New("Gmail no está configurado")
	}
	// A session-level lock serializes workers without holding row locks during
	// network calls. Disconnect/account edits remain responsive.
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return 0, errors.New("No se pudo sincronizar")
	}
	defer conn.Release()
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(728016,$1)", int32(user)).Scan(&locked); err != nil || !locked {
		return 0, errors.New("Sincronización en curso")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, e := conn.Exec(cleanup, "SELECT pg_advisory_unlock(728016,$1)", int32(user)); e != nil {
			_ = conn.Conn().Close(cleanup)
		}
	}()
	var email, page string
	var encrypted []byte
	var connected, since time.Time
	var last, until *time.Time
	var generation int64
	err = conn.QueryRow(ctx, `SELECT c.email,c.credentials,c.connected_at,COALESCE(c.scan_after,c.connected_at),c.scan_until,c.page_token,c.last_sync,u.gmail_generation FROM gmail_connections c JOIN users u ON u.id=c.user_id WHERE c.user_id=$1`, user).Scan(&email, &encrypted, &connected, &since, &until, &page, &last, &generation)
	if err != nil {
		return 0, errors.New("Gmail no conectado")
	}
	if last != nil && time.Since(*last) < 30*time.Second {
		return 0, errors.New("Espera 30 segundos entre sincronizaciones")
	}
	data, err := s.config.unseal(encrypted)
	if err != nil {
		return 0, errors.New("Revisa TOKEN_ENCRYPTION_KEY y vuelve a conectar Gmail")
	}
	var tokens gmailTokens
	if json.Unmarshal(data, &tokens) != nil {
		return 0, errors.New("Vuelve a conectar Gmail")
	}
	if time.Until(tokens.Expiry) < time.Minute {
		fresh, e := tokenRequest(ctx, url.Values{"client_id": {s.config.GoogleClientID}, "client_secret": {s.config.GoogleClientSecret}, "refresh_token": {tokens.Refresh}, "grant_type": {"refresh_token"}})
		if e != nil {
			return 0, errors.New("Vuelve a conectar Gmail: no se pudo renovar el permiso")
		}
		if fresh.Refresh == "" {
			fresh.Refresh = tokens.Refresh
		}
		tokens = fresh
	}
	// Keep page boundaries fixed while paging; overlap protects late indexing.
	if until == nil {
		end := time.Now().Truncate(time.Second).Add(time.Second)
		until = &end
	}
	start := since.Add(-5 * time.Minute)
	if start.Before(connected) {
		start = connected
	}
	query := url.Values{"maxResults": {"5"}, "q": {fmt.Sprintf("after:%d before:%d from:contacto@mail.machbank.cl subject:(Has hecho una compra)", start.Unix(), until.Unix())}}
	if page != "" {
		query.Set("pageToken", page)
	}
	var list struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		Next string `json:"nextPageToken"`
	}
	if err = gmailGet(ctx, tokens.Access, "/messages?"+query.Encode(), &list); err != nil {
		return 0, errors.New("No se pudo leer Gmail; reconecta si el permiso fue revocado")
	}
	// Retry one unmapped purchase fairly without blocking newer pages. Only
	// identifiers are persisted, never the body, merchant, or raw email headers.
	var retry string
	err = conn.QueryRow(ctx, "SELECT message_id FROM gmail_pending_messages WHERE user_id=$1 ORDER BY last_attempt,message_id LIMIT 1", user).Scan(&retry)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, errors.New("No se pudieron consultar compras pendientes")
	}
	if retry != "" {
		found := false
		for _, item := range list.Messages {
			if item.ID == retry {
				found = true
			}
		}
		if !found {
			list.Messages = append(list.Messages, struct {
				ID string `json:"id"`
			}{retry})
		}
	}
	imported := 0
	unmapped := false
	unsupported := false
	clearPending := func(id string) error {
		tx, e := conn.Begin(ctx)
		if e != nil {
			return errors.New("No se pudo actualizar la compra pendiente")
		}
		defer tx.Rollback(ctx)
		if e = verifyGmailGeneration(ctx, tx, user, generation, email); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "DELETE FROM gmail_pending_messages WHERE user_id=$1 AND message_id=$2", user, id); e != nil {
			return errors.New("No se pudo actualizar la compra pendiente")
		}
		return tx.Commit(ctx)
	}
	for _, item := range list.Messages {
		messageID := email + ":" + item.ID
		var exists bool
		if err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM gmail_imports WHERE user_id=$1 AND message_id=$2)", user, messageID).Scan(&exists); err != nil {
			return imported, errors.New("No se pudo consultar importaciones")
		}
		if exists {
			continue
		}
		var m gmailMessage
		if err = gmailGet(ctx, tokens.Access, "/messages/"+url.PathEscape(item.ID)+"?format=full", &m); err != nil {
			var responseError *gmailAPIError
			if errors.As(err, &responseError) && responseError.status == 404 {
				if e := clearPending(item.ID); e != nil {
					return imported, e
				}
				continue // A deleted message must not stall the cursor forever.
			}
			return imported, errors.New("Sincronización incompleta; vuelve a intentarlo")
		}
		purchase, ok := parseMACH(m)
		received, dateErr := strconv.ParseInt(m.InternalDate, 10, 64)
		if !ok || dateErr != nil || received < connected.UnixMilli() {
			unsupported = true
			if e := clearPending(item.ID); e != nil {
				return imported, e
			}
			continue
		} // Unknown formats are never financial movements.
		tx, e := conn.Begin(ctx)
		if e != nil {
			return imported, errors.New("No se pudo registrar la compra")
		}
		if e = verifyGmailGeneration(ctx, tx, user, generation, email); e != nil {
			_ = tx.Rollback(ctx)
			return imported, e
		}
		var account int64
		err = tx.QueryRow(ctx, "SELECT a.id FROM gmail_card_rules r JOIN accounts a ON a.id=r.account_id AND a.user_id=r.user_id WHERE r.user_id=$1 AND r.bank='mach' AND r.last_four=$2 AND a.currency='CLP' FOR UPDATE OF a", user, purchase.LastFour).Scan(&account)
		if errors.Is(err, pgx.ErrNoRows) {
			unmapped = true
			_, e = tx.Exec(ctx, "INSERT INTO gmail_pending_messages(user_id,message_id) VALUES($1,$2) ON CONFLICT(user_id,message_id) DO UPDATE SET last_attempt=now()", user, item.ID)
			if e == nil {
				e = tx.Commit(ctx)
			}
			_ = tx.Rollback(ctx)
			if e != nil {
				return imported, errors.New("No se pudo guardar la compra pendiente")
			}
			continue
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return imported, errors.New("No se pudo asociar la tarjeta")
		}
		n, e := importPurchase(ctx, tx, user, account, messageID, "mach:"+purchase.ID, purchase)
		if e == nil {
			_, e = tx.Exec(ctx, "DELETE FROM gmail_pending_messages WHERE user_id=$1 AND message_id=$2", user, item.ID)
		}
		if e == nil {
			e = tx.Commit(ctx)
		}
		_ = tx.Rollback(ctx)
		if e != nil {
			return imported, errors.New("No se pudo registrar la compra")
		}
		imported += n
	}
	data, _ = json.Marshal(tokens)
	encrypted, err = s.config.seal(data)
	if err != nil {
		return 0, errors.New("No se pudo proteger la conexión")
	}
	message := ""
	var remaining int
	if err = conn.QueryRow(ctx, "SELECT count(*) FROM gmail_pending_messages WHERE user_id=$1", user).Scan(&remaining); err != nil {
		return imported, errors.New("No se pudieron consultar compras pendientes")
	}
	unmapped = unmapped || remaining > 0
	if unmapped {
		message = "Hay compras sin cuenta asociada; configura los últimos cuatro dígitos de tu tarjeta"
	}
	if unsupported {
		message += " Hay avisos con formato no compatible o autenticación insuficiente; no se importaron."
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return imported, errors.New("No se pudo guardar la sincronización")
	}
	defer tx.Rollback(ctx)
	if err = verifyGmailGeneration(ctx, tx, user, generation, email); err != nil {
		return imported, err
	}
	var nextUntil *time.Time
	if list.Next != "" {
		nextUntil = until
	} else {
		since = *until
	}
	if _, err = tx.Exec(ctx, "UPDATE gmail_connections SET credentials=$1,page_token=$2,last_sync=now(),last_error=$3,scan_after=$5,scan_until=$6 WHERE user_id=$4", encrypted, list.Next, message, user, since, nextUntil); err != nil {
		return 0, errors.New("No se pudo guardar la sincronización")
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, errors.New("No se pudo guardar la sincronización")
	}
	return imported, nil
}

func verifyGmailGeneration(ctx context.Context, tx pgx.Tx, user, expected int64, email string) error {
	var generation int64
	err := tx.QueryRow(ctx, "SELECT u.gmail_generation FROM users u JOIN gmail_connections c ON c.user_id=u.id WHERE u.id=$1 AND c.email=$2 FOR UPDATE OF u,c", user, email).Scan(&generation)
	if err != nil || generation != expected {
		return errors.New("La conexión Gmail cambió; vuelve a intentarlo")
	}
	return nil
}
func importPurchase(ctx context.Context, tx pgx.Tx, user, account int64, messageID, purchaseID string, p Purchase) (int, error) {
	tag, err := tx.Exec(ctx, "INSERT INTO gmail_imports(user_id,message_id,purchase_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", user, messageID, purchaseID)
	if err != nil || tag.RowsAffected() == 0 {
		return 0, err
	}
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO movements(user_id,account_id,title,category,date,amount,kind,pending) VALUES($1,$2,'Pendiente de completar','Sin categoría',$3::date,$4::numeric,'expense',true) RETURNING id`, user, account, p.Date, p.Amount).Scan(&id)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, "UPDATE gmail_imports SET movement_id=$1 WHERE user_id=$2 AND message_id=$3", id, user, messageID)
	return 1, err
}
