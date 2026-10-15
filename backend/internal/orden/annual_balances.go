package orden

import (
	"net/http"
	"strconv"
	"time"
)

type annualBalance struct {
	Year    int    `json:"year"`
	Balance string `json:"balance"`
}

func validAnnualYear(value string, currentYear int) (int, bool) {
	year, err := strconv.Atoi(value)
	return year, err == nil && len(value) == 4 && year >= 1900 && year < currentYear
}

func (s *Server) listAnnualBalances(w http.ResponseWriter, r *http.Request) {
	currency := r.URL.Query().Get("currency")
	if currency != "CLP" && currency != "USD" {
		fail(w, 400, "Moneda inválida")
		return
	}
	rows, err := s.db.Query(r.Context(), "SELECT year,balance::text FROM annual_balances WHERE user_id=$1 AND currency=$2 ORDER BY year", currentUser(r), currency)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	items := []annualBalance{}
	for rows.Next() {
		var item annualBalance
		if err = rows.Scan(&item.Year, &item.Balance); err != nil {
			databaseError(w, err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"currency": currency, "current_year": time.Now().UTC().Year(), "items": items})
}

func (s *Server) saveAnnualBalance(w http.ResponseWriter, r *http.Request) {
	year, ok := validAnnualYear(r.PathValue("year"), time.Now().UTC().Year())
	if !ok {
		fail(w, 400, "Elige un año cerrado desde 1900, anterior al año actual")
		return
	}
	var body struct {
		Currency string `json:"currency"`
		Balance  string `json:"balance"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !validMoney(body.Balance, body.Currency, false) {
		fail(w, 400, "Saldo o moneda inválidos; CLP sin fracciones y USD con hasta dos decimales")
		return
	}
	var saved annualBalance
	err := s.db.QueryRow(r.Context(), `INSERT INTO annual_balances(user_id,currency,year,balance) VALUES($1,$2,$3,$4::numeric)
 ON CONFLICT(user_id,currency,year) DO UPDATE SET balance=excluded.balance,updated_at=now() RETURNING year,balance::text`, currentUser(r), body.Currency, year, body.Balance).Scan(&saved.Year, &saved.Balance)
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, saved)
}

func (s *Server) deleteAnnualBalance(w http.ResponseWriter, r *http.Request) {
	year, ok := validAnnualYear(r.PathValue("year"), time.Now().UTC().Year())
	currency := r.URL.Query().Get("currency")
	if !ok || (currency != "CLP" && currency != "USD") {
		fail(w, 400, "Año o moneda inválidos")
		return
	}
	result, err := s.db.Exec(r.Context(), "DELETE FROM annual_balances WHERE user_id=$1 AND currency=$2 AND year=$3", currentUser(r), currency, year)
	if err != nil {
		databaseError(w, err)
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, 404, "Cierre no encontrado")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
