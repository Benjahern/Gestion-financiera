package orden

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

type Account struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Currency       string `json:"currency"`
	OpeningBalance string `json:"opening_balance"`
	Balance        string `json:"balance"`
	IsInvestment   bool   `json:"is_investment"`
	ShowInSummary  bool   `json:"show_in_summary"`
}
type Movement struct {
	ID                int64  `json:"id"`
	AccountID         int64  `json:"account_id"`
	Account           string `json:"account"`
	SourceAccountID   *int64 `json:"source_account_id,omitempty"`
	SourceAccount     string `json:"source_account,omitempty"`
	InvestmentMode    string `json:"investment_mode"`
	Currency          string `json:"currency"`
	Title             string `json:"title"`
	Category          string `json:"category"`
	Date              string `json:"date"`
	Amount            string `json:"amount"`
	Kind              string `json:"kind"`
	Pending           bool   `json:"pending"`
	CategoryConfirmed bool   `json:"category_confirmed"`
}

var moneyPattern = regexp.MustCompile(`^-?[0-9]{1,12}(\.[0-9]{1,2})?$`)

func validMoney(value, currency string, positive bool) bool {
	if !moneyPattern.MatchString(value) {
		return false
	}
	parts := strings.Split(value, ".")
	if currency == "CLP" && len(parts) > 1 && strings.Trim(parts[1], "0") != "" {
		return false
	}
	if positive && (strings.HasPrefix(value, "-") || strings.Trim(strings.ReplaceAll(value, ".", ""), "0") == "") {
		return false
	}
	return currency == "CLP" || currency == "USD"
}
func validText(value string, max int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) > 0 && utf8.RuneCountInString(value) <= max
}
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, 400, "Identificador inválido")
		return 0, false
	}
	return id, true
}
func databaseError(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			if pgErr.ConstraintName == "accounts_summary_currency_key" {
				fail(w, 409, "Ya hay una cuenta destacada en esa moneda; desmarca Mostrar en Resumen antes de cambiar la moneda")
				return
			}
			fail(w, 409, "Ya existe un registro con ese nombre")
			return
		case "23503":
			fail(w, 409, "La cuenta no existe o tiene movimientos asociados")
			return
		case "23514", "22003", "22007", "22008":
			fail(w, 400, "Datos financieros inválidos")
			return
		}
	}
	fail(w, 503, "No se pudo completar la operación")
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `SELECT a.id,a.name,a.type,a.currency,a.opening_balance::text,
	        (a.opening_balance+COALESCE(sum(CASE WHEN m.kind='income' THEN m.amount WHEN m.kind='investment' AND m.investment_mode='contribution' THEN m.amount ELSE -m.amount END),0)
	        -COALESCE((SELECT sum(sm.amount) FROM movements sm WHERE sm.source_account_id=a.id AND sm.user_id=a.user_id),0))::text,a.is_investment,a.show_in_summary
        FROM accounts a LEFT JOIN movements m ON m.account_id=a.id AND m.user_id=a.user_id
        WHERE a.user_id=1 GROUP BY a.id ORDER BY a.created_at,a.id`)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	items := []Account{}
	for rows.Next() {
		var a Account
		if err = rows.Scan(&a.ID, &a.Name, &a.Type, &a.Currency, &a.OpeningBalance, &a.Balance, &a.IsInvestment, &a.ShowInSummary); err != nil {
			databaseError(w, err)
			return
		}
		items = append(items, a)
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) saveAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string `json:"name"`
		Type           string `json:"type"`
		Currency       string `json:"currency"`
		OpeningBalance string `json:"opening_balance"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Type = strings.TrimSpace(body.Type)
	if !validText(body.Name, 100) || !validText(body.Type, 100) || !validMoney(body.OpeningBalance, body.Currency, false) {
		fail(w, 400, "Nombre, tipo, moneda o saldo inicial inválido")
		return
	}
	var id int64
	var err error
	if r.Method == http.MethodPost {
		err = s.db.QueryRow(r.Context(), "INSERT INTO accounts(user_id,name,type,currency,opening_balance,is_investment) VALUES(1,$1,$2,$3,$4::numeric,$5) RETURNING id", body.Name, body.Type, body.Currency, body.OpeningBalance, investmentAccountType(body.Type)).Scan(&id)
	} else {
		var ok bool
		id, ok = pathID(w, r)
		if !ok {
			return
		}
		tag, e := s.db.Exec(r.Context(), `UPDATE accounts SET name=$1,type=$2,currency=$3,opening_balance=$4::numeric
	            WHERE id=$5 AND user_id=1 AND (currency=$3 OR (NOT EXISTS(SELECT 1 FROM movements WHERE account_id=$5 OR source_account_id=$5) AND NOT EXISTS(SELECT 1 FROM gmail_card_rules WHERE account_id=$5)))`, body.Name, body.Type, body.Currency, body.OpeningBalance, id)
		err = e
		if err == nil && tag.RowsAffected() == 0 {
			fail(w, 409, "Cuenta inexistente o moneda bloqueada por movimientos o tarjetas vinculadas")
			return
		}
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	writeJSON(w, status, map[string]int64{"id": id})
}

func investmentAccountType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "inversión", "inversion", "inversiones", "cuenta de inversión", "cuenta de inversion":
		return true
	}
	return false
}

func (s *Server) setAccountInvestment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		IsInvestment *bool `json:"is_investment"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.IsInvestment == nil {
		fail(w, 400, "Indica si la cuenta es de inversión")
		return
	}
	tag, err := s.db.Exec(r.Context(), "UPDATE accounts SET is_investment=$1 WHERE id=$2 AND user_id=$3", *body.IsInvestment, id, currentUser(r))
	if err != nil {
		databaseError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "Cuenta no encontrada")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tag, err := s.db.Exec(r.Context(), "DELETE FROM accounts WHERE id=$1 AND user_id=1", id)
	if err != nil {
		databaseError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "Cuenta no encontrada")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) saveMovement(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID         int64  `json:"account_id"`
		SourceAccountID   *int64 `json:"source_account_id"`
		InvestmentMode    string `json:"investment_mode"`
		Title             string `json:"title"`
		Category          string `json:"category"`
		Date              string `json:"date"`
		Amount            string `json:"amount"`
		Kind              string `json:"kind"`
		CategoryConfirmed bool   `json:"category_confirmed"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Category = strings.TrimSpace(body.Category)
	if body.Kind == "investment" && body.InvestmentMode == "" {
		body.InvestmentMode = "legacy_outflow"
	}
	if body.Kind != "investment" {
		body.InvestmentMode = "legacy_outflow"
	}
	_, dateErr := time.Parse("2006-01-02", body.Date)
	if body.AccountID <= 0 || !validText(body.Title, 150) || !validText(body.Category, 100) || dateErr != nil || (body.Kind != "expense" && body.Kind != "income" && body.Kind != "investment") || (body.Kind == "investment" && body.InvestmentMode != "legacy_outflow" && body.InvestmentMode != "contribution") || (body.Kind != "investment" && body.SourceAccountID != nil) {
		fail(w, 400, "Datos del movimiento inválidos")
		return
	}
	if body.InvestmentMode != "contribution" && body.SourceAccountID != nil {
		fail(w, 400, "El origen solo se admite en nuevos aportes de inversión")
		return
	}
	if body.SourceAccountID != nil && (*body.SourceAccountID <= 0 || *body.SourceAccountID == body.AccountID) {
		fail(w, 400, "Selecciona cuentas distintas para origen y destino")
		return
	}
	// Lock both accounts in ID order to avoid deadlocks and prevent currency changes.
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	accountIDs := []int64{body.AccountID}
	if body.SourceAccountID != nil {
		accountIDs = append(accountIDs, *body.SourceAccountID)
	}
	rows, err := tx.Query(r.Context(), "SELECT id,currency,is_investment FROM accounts WHERE user_id=$1 AND id=ANY($2) ORDER BY id FOR UPDATE", currentUser(r), accountIDs)
	if err != nil {
		databaseError(w, err)
		return
	}
	accountCurrencies := map[int64]string{}
	investmentDestination := false
	for rows.Next() {
		var accountID int64
		var accountCurrency string
		var isInvestment bool
		if err = rows.Scan(&accountID, &accountCurrency, &isInvestment); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		accountCurrencies[accountID] = accountCurrency
		if accountID == body.AccountID {
			investmentDestination = isInvestment
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		databaseError(w, err)
		return
	}
	rows.Close()
	currency, accountExists := accountCurrencies[body.AccountID]
	if !accountExists {
		fail(w, 400, "Selecciona una cuenta válida")
		return
	}
	if body.InvestmentMode == "contribution" {
		if !investmentDestination {
			fail(w, 400, "Selecciona una cuenta de inversión como destino")
			return
		}
		if body.SourceAccountID != nil && accountCurrencies[*body.SourceAccountID] != currency {
			fail(w, 400, "El origen y el destino deben usar la misma moneda")
			return
		}
	}
	if !validMoney(body.Amount, currency, true) {
		fail(w, 400, "Importe inválido: CLP sin decimales, USD con hasta dos decimales")
		return
	}
	var id int64
	if r.Method == http.MethodPost {
		err = tx.QueryRow(r.Context(), "INSERT INTO movements(user_id,account_id,source_account_id,investment_mode,title,category,date,amount,kind) VALUES(1,$1,$2,$3,$4,$5,$6::date,$7::numeric,$8) RETURNING id", body.AccountID, body.SourceAccountID, body.InvestmentMode, body.Title, body.Category, body.Date, body.Amount, body.Kind).Scan(&id)
	} else {
		var ok bool
		id, ok = pathID(w, r)
		if !ok {
			return
		}
		tag, e := tx.Exec(r.Context(), "UPDATE movements SET account_id=$1,source_account_id=$2,investment_mode=$3,title=$4,category=$5,date=$6::date,amount=$7::numeric,kind=$8 WHERE id=$9 AND user_id=1", body.AccountID, body.SourceAccountID, body.InvestmentMode, body.Title, body.Category, body.Date, body.Amount, body.Kind, id)
		err = e
		if err == nil && tag.RowsAffected() == 0 {
			fail(w, 404, "Movimiento no encontrado")
			return
		}
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = s.recordTraining(r.Context(), tx, 1, id, body.Title, body.Category, body.Kind, body.CategoryConfirmed); err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		databaseError(w, err)
		return
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	writeJSON(w, status, map[string]int64{"id": id})
}

func (s *Server) deleteMovement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tag, err := s.db.Exec(r.Context(), "DELETE FROM movements WHERE id=$1 AND user_id=1", id)
	if err != nil {
		databaseError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "Movimiento no encontrado")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func monthRange(month string) (string, string, bool) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", false
	}
	return t.Format("2006-01-02"), t.AddDate(0, 1, 0).Format("2006-01-02"), true
}

func (s *Server) listMovements(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("pending") != "" && q.Get("pending") != "true" {
		fail(w, 400, "Filtro de pendientes inválido")
		return
	}
	kind := q.Get("kind")
	currency := q.Get("currency")
	category := q.Get("category")
	search := q.Get("search")
	if (kind != "" && kind != "expense" && kind != "income" && kind != "investment") || (currency != "" && currency != "CLP" && currency != "USD") || (search != "" && !validText(search, 150)) || (category != "" && !validText(category, 100)) {
		fail(w, 400, "Filtros inválidos")
		return
	}
	from, to := "0001-01-01", "9999-12-31"
	if q.Get("month") != "" {
		var ok bool
		from, to, ok = monthRange(q.Get("month"))
		if !ok {
			fail(w, 400, "Mes inválido")
			return
		}
	}
	limit, offset := 50, 0
	if q.Get("limit") != "" {
		n, e := strconv.Atoi(q.Get("limit"))
		if e != nil || n < 1 || n > 100 {
			fail(w, 400, "Límite inválido")
			return
		}
		limit = n
	}
	if q.Get("offset") != "" {
		n, e := strconv.Atoi(q.Get("offset"))
		if e != nil || n < 0 || n > 1000000 {
			fail(w, 400, "Página inválida")
			return
		}
		offset = n
	}
	search = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(search)
	args := []any{kind, currency, category, from, to, search, q.Get("pending") == "true"}
	where := ` FROM movements m JOIN accounts a ON a.id=m.account_id AND a.user_id=m.user_id
        LEFT JOIN accounts sa ON sa.id=m.source_account_id AND sa.user_id=m.user_id WHERE m.user_id=1
        AND ($1='' OR m.kind=$1) AND ($2='' OR a.currency=$2) AND ($3='' OR m.category=$3)
        AND m.date >= $4::date AND m.date < $5::date AND ($6='' OR m.title ILIKE '%' || $6 || '%') AND (NOT $7::boolean OR m.pending)`
	var total int
	if err := s.db.QueryRow(r.Context(), "SELECT count(*)"+where, args...).Scan(&total); err != nil {
		databaseError(w, err)
		return
	}
	args = append(args, limit, offset)
	rows, err := s.db.Query(r.Context(), "SELECT m.id,m.account_id,a.name,a.currency,m.source_account_id,COALESCE(sa.name,''),m.investment_mode,m.title,m.category,m.date::text,m.amount::text,m.kind,m.pending,m.category_confirmed"+where+" ORDER BY m.date DESC,m.id DESC LIMIT $8 OFFSET $9", args...)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	items := []Movement{}
	for rows.Next() {
		var m Movement
		if err = rows.Scan(&m.ID, &m.AccountID, &m.Account, &m.Currency, &m.SourceAccountID, &m.SourceAccount, &m.InvestmentMode, &m.Title, &m.Category, &m.Date, &m.Amount, &m.Kind, &m.Pending, &m.CategoryConfirmed); err != nil {
			databaseError(w, err)
			return
		}
		items = append(items, m)
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}
