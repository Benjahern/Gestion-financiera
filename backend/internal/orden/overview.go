package orden

import (
	"net/http"
	"time"
)

type CategoryTotal struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}
type MonthTotal struct {
	Month      string `json:"month"`
	Income     string `json:"income"`
	Expense    string `json:"expense"`
	Investment string `json:"investment"`
}
type Overview struct {
	Currency   string          `json:"currency"`
	Month      string          `json:"month"`
	Balance    string          `json:"balance"`
	Income     string          `json:"income"`
	Expense    string          `json:"expense"`
	Investment string          `json:"investment"`
	Count      int             `json:"count"`
	Categories []CategoryTotal `json:"categories"`
	Flow       []MonthTotal    `json:"flow"`
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	currency, month := r.URL.Query().Get("currency"), r.URL.Query().Get("month")
	if currency == "" {
		currency = "CLP"
	}
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	from, to, ok := monthRange(month)
	if !ok || (currency != "CLP" && currency != "USD") {
		fail(w, 400, "Moneda o mes inválido")
		return
	}
	out := Overview{Currency: currency, Month: month, Categories: []CategoryTotal{}, Flow: []MonthTotal{}}
	// One consistent read snapshot for all dashboard totals.
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
		databaseError(w, err)
		return
	}
	err = tx.QueryRow(r.Context(), `SELECT (COALESCE((SELECT sum(opening_balance) FROM accounts WHERE user_id=1 AND currency=$1),0)
        +COALESCE((SELECT sum(CASE WHEN m.kind='income' THEN m.amount ELSE -m.amount END) FROM movements m JOIN accounts a ON a.id=m.account_id AND a.user_id=m.user_id WHERE m.user_id=1 AND a.currency=$1),0))::text`, currency).Scan(&out.Balance)
	if err != nil {
		databaseError(w, err)
		return
	}
	err = tx.QueryRow(r.Context(), `SELECT COALESCE(sum(m.amount) FILTER(WHERE m.kind='income'),0)::text,
        COALESCE(sum(m.amount) FILTER(WHERE m.kind='expense'),0)::text,COALESCE(sum(m.amount) FILTER(WHERE m.kind='investment'),0)::text,count(*)
        FROM movements m JOIN accounts a ON a.id=m.account_id AND a.user_id=m.user_id WHERE m.user_id=1 AND a.currency=$1 AND m.date >= $2::date AND m.date < $3::date`, currency, from, to).Scan(&out.Income, &out.Expense, &out.Investment, &out.Count)
	if err != nil {
		databaseError(w, err)
		return
	}
	rows, err := tx.Query(r.Context(), `SELECT m.category,sum(m.amount)::text FROM movements m JOIN accounts a ON a.id=m.account_id AND a.user_id=m.user_id
        WHERE m.user_id=1 AND a.currency=$1 AND m.kind='expense' AND m.date >= $2::date AND m.date < $3::date GROUP BY m.category ORDER BY sum(m.amount) DESC,m.category`, currency, from, to)
	if err != nil {
		databaseError(w, err)
		return
	}
	for rows.Next() {
		var item CategoryTotal
		if err = rows.Scan(&item.Name, &item.Amount); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		out.Categories = append(out.Categories, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		databaseError(w, err)
		return
	}
	rows, err = tx.Query(r.Context(), `SELECT to_char(months.month,'YYYY-MM'),
        COALESCE(sum(m.amount) FILTER(WHERE m.kind='income'),0)::text,
        COALESCE(sum(m.amount) FILTER(WHERE m.kind='expense'),0)::text,
        COALESCE(sum(m.amount) FILTER(WHERE m.kind='investment'),0)::text
        FROM generate_series($2::date-interval '5 months',$2::date,interval '1 month') months(month)
        LEFT JOIN (movements m JOIN accounts a ON a.id=m.account_id AND a.user_id=m.user_id AND a.currency=$1)
        ON m.user_id=1 AND m.date>=months.month AND m.date<months.month+interval '1 month'
        GROUP BY months.month ORDER BY months.month`, currency, from)
	if err != nil {
		databaseError(w, err)
		return
	}
	for rows.Next() {
		var item MonthTotal
		if err = rows.Scan(&item.Month, &item.Income, &item.Expense, &item.Investment); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		out.Flow = append(out.Flow, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
