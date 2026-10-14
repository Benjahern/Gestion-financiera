package orden

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	db            *pgxpool.Pool
	config        Config
	passwordSlots chan struct{}
	dummyHash     string
}

type sessionKey struct{}
type session struct {
	UserID int64
	Hash   []byte
	CSRF   string
}

func NewServer(db *pgxpool.Pool, config Config) *Server {
	dummy, _ := HashPassword("dummy-password-not-a-login")
	return &Server{db: db, config: config, passwordSlots: make(chan struct{}, 2), dummyHash: dummy}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.Handle("POST /api/auth/start", s.limit("start", 30, http.HandlerFunc(s.start)))
	mux.Handle("POST /api/auth/login", s.limit("login", 10, http.HandlerFunc(s.login)))
	mux.Handle("POST /api/auth/activate", s.limit("activate", 10, http.HandlerFunc(s.activate)))
	mux.Handle("POST /api/auth/recover", s.limit("recover", 10, http.HandlerFunc(s.recoverPassword)))
	mux.Handle("GET /api/auth/session", s.authenticate(http.HandlerFunc(s.currentSession)))
	mux.Handle("POST /api/auth/logout", s.authenticate(http.HandlerFunc(s.logout)))
	mux.Handle("POST /api/auth/logout-all", s.authenticate(http.HandlerFunc(s.logoutAll)))
	mux.Handle("GET /api/accounts", s.authenticate(http.HandlerFunc(s.listAccounts)))
	mux.Handle("POST /api/accounts", s.authenticate(http.HandlerFunc(s.saveAccount)))
	mux.Handle("PUT /api/accounts/{id}", s.authenticate(http.HandlerFunc(s.saveAccount)))
	mux.Handle("DELETE /api/accounts/{id}", s.authenticate(http.HandlerFunc(s.deleteAccount)))
	mux.Handle("GET /api/movements", s.authenticate(http.HandlerFunc(s.listMovements)))
	mux.Handle("POST /api/movements", s.authenticate(http.HandlerFunc(s.saveMovement)))
	mux.Handle("PUT /api/movements/{id}", s.authenticate(http.HandlerFunc(s.saveMovement)))
	mux.Handle("DELETE /api/movements/{id}", s.authenticate(http.HandlerFunc(s.deleteMovement)))
	mux.Handle("GET /api/overview", s.authenticate(http.HandlerFunc(s.overview)))
	s.importRoutes(mux)
	return s.security(mux)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		fail(w, 400, "Solicitud inválida")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		fail(w, 400, "Solicitud inválida")
		return false
	}
	return true
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		fail(w, 503, "No disponible")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) cookieName() string {
	if s.config.SecureCookies {
		return "__Host-orden_session"
	}
	return "orden_dev_session"
}

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if s.config.SecureCookies {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		// Only the private web proxy may assert a client IP in production.
		// Public callers cannot bypass this by spoofing X-Forwarded-For.
		if s.config.ProxySecret != "" && r.URL.Path != "/api/health" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Orden-Proxy-Secret")), []byte(s.config.ProxySecret)) != 1 || net.ParseIP(r.Header.Get("X-Orden-Client-IP")) == nil {
				fail(w, 403, "Proxy no autorizado")
				return
			}
		}
		defer func() {
			if recover() != nil {
				slog.Error("http panic")
				fail(w, 500, "Error interno")
			}
		}()
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("Origin") != s.config.Origin {
				fail(w, 403, "Origen no permitido")
				return
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				fail(w, 403, "Solicitud no permitida")
				return
			}
			mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if mediaErr != nil || mediaType != "application/json" {
				fail(w, 415, "Se requiere JSON")
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		timeout := 10 * time.Second
		if r.URL.Path == "/api/gmail/callback" {
			timeout = 25 * time.Second
		} else if r.URL.Path == "/api/gmail/sync" {
			timeout = 75 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cookieName())
		if err != nil || len(cookie.Value) != 43 {
			fail(w, 401, "Inicia sesión para continuar")
			return
		}
		auth := session{Hash: tokenHash(cookie.Value)}
		err = s.db.QueryRow(r.Context(), "SELECT s.csrf_token,s.user_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.email=$2", auth.Hash, s.config.OwnerEmail).Scan(&auth.CSRF, &auth.UserID)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				fail(w, 503, "No se pudo verificar la sesión")
				return
			}
			fail(w, 401, "La sesión no está disponible; vuelve a iniciar sesión")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(auth.CSRF)) != 1 {
				fail(w, 403, "Solicitud no permitida")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, auth)))
	})
}

// Per-client PostgreSQL counters survive restarts. Client headers are accepted
// only from the authenticated private proxy, never from arbitrary callers.
func (s *Server) limit(key string, maximum int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Orden-Client-IP")
		if s.config.ProxySecret == "" {
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		}
		clientKey := key + ":" + fmt.Sprintf("%x", tokenHash(ip))
		var attempts int
		err := s.db.QueryRow(r.Context(), `INSERT INTO rate_limits(key,window_start,attempts) VALUES($1,now(),1)
            ON CONFLICT(key) DO UPDATE SET attempts=CASE WHEN rate_limits.window_start < now()-interval '15 minutes' THEN 1 ELSE rate_limits.attempts+1 END,
            window_start=CASE WHEN rate_limits.window_start < now()-interval '15 minutes' THEN now() ELSE rate_limits.window_start END RETURNING attempts`, clientKey).Scan(&attempts)
		if err != nil {
			fail(w, 503, "Servicio no disponible")
			return
		}
		if attempts > maximum {
			w.Header().Set("Retry-After", "900")
			fail(w, 429, "Demasiados intentos. Espera 15 minutos.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) passwordWork(w http.ResponseWriter, r *http.Request, fn func()) {
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
		fn()
	case <-r.Context().Done():
		fail(w, 503, "Intenta nuevamente")
	}
}
