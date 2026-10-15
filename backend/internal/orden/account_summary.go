package orden

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// This preference never changes opening balances, movements or investment flags.
func (s *Server) setAccountSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		ShowInSummary *bool `json:"show_in_summary"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.ShowInSummary == nil {
		fail(w, 400, "Indica si la cuenta se muestra en el Resumen")
		return
	}
	ctx := r.Context()
	user := currentUser(r)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// Serialize selections per owner, before locking accounts, including concurrent requests.
	if _, err = tx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", user); err != nil {
		databaseError(w, err)
		return
	}
	var currency string
	err = tx.QueryRow(ctx, "SELECT currency FROM accounts WHERE id=$1 AND user_id=$2 FOR UPDATE", id, user).Scan(&currency)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Cuenta no encontrada")
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if *body.ShowInSummary {
		if _, err = tx.Exec(ctx, "UPDATE accounts SET show_in_summary=false WHERE user_id=$1 AND currency=$2 AND show_in_summary AND id<>$3", user, currency, id); err != nil {
			databaseError(w, err)
			return
		}
	}
	if _, err = tx.Exec(ctx, "UPDATE accounts SET show_in_summary=$1 WHERE id=$2 AND user_id=$3", *body.ShowInSummary, id, user); err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
