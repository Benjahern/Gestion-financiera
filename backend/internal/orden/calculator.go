package orden

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type calculatorSettings struct {
	Currency         string `json:"currency"`
	ReferenceIncome  string `json:"reference_income"`
	General          string `json:"general"`
	Wants            string `json:"wants"`
	Investment       string `json:"investment"`
	IndexFund        string `json:"index_fund"`
	Reserve          string `json:"reserve"`
	EmergencyGoal    string `json:"emergency_goal"`
	EmergencyBalance string `json:"emergency_balance"`
	OwnHomePercent   string `json:"own_home_percent"`
	RentalPercent    string `json:"rental_percent"`
	TravelPercent    string `json:"travel_percent"`
}

// Integer minor units; validMoney bounds values well below int64's limit.
func planningUnits(value, currency string) (int64, bool) {
	if strings.HasPrefix(value, "-") || !validMoney(value, currency, false) {
		return 0, false
	}
	whole, fraction, _ := strings.Cut(value, ".")
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, false
	}
	if currency == "CLP" {
		return w, true
	}
	fraction = (fraction + "00")[:2]
	f, err := strconv.ParseInt(fraction, 10, 64)
	return w*100 + f, err == nil
}

func validCalculator(c calculatorSettings) bool {
	values := []string{c.ReferenceIncome, c.General, c.Wants, c.Investment, c.IndexFund, c.Reserve, c.EmergencyGoal, c.EmergencyBalance}
	u := make([]int64, len(values))
	for i, value := range values {
		var ok bool
		u[i], ok = planningUnits(value, c.Currency)
		if !ok {
			return false
		}
	}
	if u[0] <= 0 || u[6] <= 0 || u[1]+u[2]+u[3] != u[0] || u[4]+u[5] != u[3] {
		return false
	}
	var percent int64
	for _, value := range []string{c.OwnHomePercent, c.RentalPercent, c.TravelPercent} {
		p, ok := planningUnits(value, "USD") // hundredths of a percent
		if !ok || p > 10000 {
			return false
		}
		percent += p
	}
	return percent == 10000
}

func (s *Server) getCalculator(w http.ResponseWriter, r *http.Request) {
	currency := r.URL.Query().Get("currency")
	if currency != "CLP" && currency != "USD" {
		fail(w, 400, "Moneda inválida")
		return
	}
	var data []byte
	err := s.db.QueryRow(r.Context(), "SELECT settings FROM calculator_settings WHERE user_id=$1 AND currency=$2", currentUser(r), currency).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, map[string]any{"settings": nil})
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"settings": json.RawMessage(data)})
}

func (s *Server) saveCalculator(w http.ResponseWriter, r *http.Request) {
	var c calculatorSettings
	if !decode(w, r, &c) {
		return
	}
	if !validCalculator(c) {
		fail(w, 400, "Configuración inválida: reparte todo el sueldo y la inversión; los destinos deben sumar 100% y la meta debe ser positiva")
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		fail(w, 500, "No se pudo guardar la configuración")
		return
	}
	_, err = s.db.Exec(r.Context(), `INSERT INTO calculator_settings(user_id,currency,settings) VALUES($1,$2,$3::jsonb)
        ON CONFLICT(user_id,currency) DO UPDATE SET settings=excluded.settings,updated_at=now()`, currentUser(r), c.Currency, data)
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"settings": c})
}
