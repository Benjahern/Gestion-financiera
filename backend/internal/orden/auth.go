package orden

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (s *Server) owner(email string) bool {
	return strings.ToLower(strings.TrimSpace(email)) == s.config.OwnerEmail
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !s.owner(body.Email) {
		fail(w, 403, "Acceso no permitido")
		return
	}
	var exists bool
	if err := s.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE id=1)").Scan(&exists); err != nil {
		fail(w, 503, "Servicio no disponible")
		return
	}
	next := "activate"
	if exists {
		next = "login"
	}
	writeJSON(w, 200, map[string]string{"next": next})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body credentials
	if !decode(w, r, &body) {
		return
	}
	if len(body.Password) > 256 {
		fail(w, 401, "Credenciales inválidas")
		return
	}
	s.passwordWork(w, r, func() {
		var hash string
		err := s.db.QueryRow(r.Context(), "SELECT password_hash FROM users WHERE id=1 AND email=$1 AND NOT EXISTS(SELECT 1 FROM activation_codes WHERE purpose='recover' AND expires_at>now())", strings.ToLower(strings.TrimSpace(body.Email))).Scan(&hash)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			fail(w, 503, "Servicio no disponible")
			return
		}
		if hash == "" {
			hash = s.dummyHash
		}
		valid := VerifyPassword(hash, body.Password)
		if err != nil || !s.owner(body.Email) || !valid {
			fail(w, 401, "Credenciales inválidas")
			return
		}
		// Lock against recovery so a concurrent password reset cannot leave an old login alive.
		tx, err := s.db.Begin(r.Context())
		if err != nil {
			fail(w, 503, "Servicio no disponible")
			return
		}
		defer tx.Rollback(r.Context())
		if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(728015)"); err != nil {
			fail(w, 503, "Servicio no disponible")
			return
		}
		var current string
		err = tx.QueryRow(r.Context(), "SELECT password_hash FROM users WHERE id=1 AND NOT EXISTS(SELECT 1 FROM activation_codes WHERE purpose='recover' AND expires_at>now())").Scan(&current)
		if err != nil || current != hash {
			fail(w, 401, "Credenciales inválidas")
			return
		}
		s.newSession(w, r, tx)
	})
}

func (s *Server) activate(w http.ResponseWriter, r *http.Request) { s.consumeCode(w, r, "activate") }
func (s *Server) recoverPassword(w http.ResponseWriter, r *http.Request) {
	s.consumeCode(w, r, "recover")
}

func (s *Server) consumeCode(w http.ResponseWriter, r *http.Request, purpose string) {
	var body credentials
	if !decode(w, r, &body) {
		return
	}
	if !s.owner(body.Email) || len(body.Code) != 43 {
		fail(w, 403, "Correo o código inválido")
		return
	}
	if !PasswordValid(body.Password) {
		fail(w, 400, "La contraseña debe tener al menos 12 caracteres y un máximo de 256 bytes")
		return
	}
	s.passwordWork(w, r, func() {
		tx, err := s.db.Begin(r.Context())
		if err != nil {
			fail(w, 503, "Servicio no disponible")
			return
		}
		defer tx.Rollback(r.Context())
		if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(728015)"); err != nil {
			fail(w, 503, "Servicio no disponible")
			return
		}
		var expected []byte
		err = tx.QueryRow(r.Context(), "SELECT token_hash FROM activation_codes WHERE purpose=$1 AND expires_at>now() FOR UPDATE", purpose).Scan(&expected)
		if err != nil || subtle.ConstantTimeCompare(expected, tokenHash(body.Code)) != 1 {
			fail(w, 403, "Código inválido, caducado o ya utilizado")
			return
		}
		hash, err := HashPassword(body.Password)
		if err != nil {
			fail(w, 500, "Error interno")
			return
		}
		if purpose == "activate" {
			_, err = tx.Exec(r.Context(), "INSERT INTO users(id,email,password_hash) VALUES(1,$1,$2)", s.config.OwnerEmail, hash)
		} else {
			_, err = tx.Exec(r.Context(), "UPDATE users SET password_hash=$1 WHERE id=1", hash)
		}
		if err != nil {
			fail(w, 409, "No se pudo completar la activación")
			return
		}
		if _, err = tx.Exec(r.Context(), "DELETE FROM activation_codes"); err != nil {
			fail(w, 500, "Error interno")
			return
		}
		if _, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id=1"); err != nil {
			fail(w, 500, "Error interno")
			return
		}
		s.newSession(w, r, tx)
	})
}

func (s *Server) newSession(w http.ResponseWriter, r *http.Request, tx pgx.Tx) {
	token, err := randomToken()
	if err != nil {
		fail(w, 500, "Error interno")
		return
	}
	csrf, err := randomToken()
	if err != nil {
		fail(w, 500, "Error interno")
		return
	}
	expires := time.Now().Add(7 * 24 * time.Hour)
	if _, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE expires_at <= now()"); err != nil {
		fail(w, 500, "Error interno")
		return
	}
	if _, err = tx.Exec(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,1,$2,$3)", tokenHash(token), csrf, expires); err != nil {
		fail(w, 500, "Error interno")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 500, "Error interno")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: token, Path: "/", HttpOnly: true, Secure: s.config.SecureCookies, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: 7 * 24 * 3600})
	writeJSON(w, 200, map[string]string{"email": s.config.OwnerEmail, "csrf_token": csrf})
}

func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	auth := r.Context().Value(sessionKey{}).(session)
	writeJSON(w, 200, map[string]string{"email": s.config.OwnerEmail, "csrf_token": auth.CSRF})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request)    { s.endSessions(w, r, false) }
func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request) { s.endSessions(w, r, true) }
func (s *Server) endSessions(w http.ResponseWriter, r *http.Request, all bool) {
	auth := r.Context().Value(sessionKey{}).(session)
	query := "DELETE FROM sessions WHERE token_hash=$1"
	args := []any{auth.Hash}
	if all {
		query = "DELETE FROM sessions WHERE user_id=1"
		args = nil
	}
	if _, err := s.db.Exec(r.Context(), query, args...); err != nil {
		fail(w, 503, "Servicio no disponible")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Path: "/", HttpOnly: true, Secure: s.config.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	writeJSON(w, 200, map[string]bool{"ok": true})
}
