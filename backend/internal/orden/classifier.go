package orden

import (
	"context"
	"math"
	"net/http"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
)

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type Example struct{ Description, Category string }
type Prediction struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Automatic  bool    `json:"automatic"`
	Examples   int     `json:"examples"`
}

var defaults = []string{
	"Vida diaria > Comida", "Vida diaria > Agua", "Vida diaria > Casa",
	"Transporte > Metro", "Transporte > Micro", "Transporte > Bencina",
	"Entretenimiento > Juntas", "Entretenimiento > Juegos", "Entretenimiento > Gimnasio",
	"Vacaciones", "Ocio > Cine", "Ocio > Cosas", "Ocio > Familia",
	"Personal > Corte de pelo", "Personal > Caprichos", "Personal > Remedios",
	"Préstamos > Gastos a fondo de emergencia", "Préstamos > Cuotas",
	"Pagos varios > Cumple", "Pagos varios > Regalos", "Pagos varios > Cosas",
}

func currentUser(r *http.Request) int64 { return r.Context().Value(sessionKey{}).(session).UserID }

var accentNormalizer = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u")

func words(text string) []string {
	text = accentNormalizer.Replace(strings.ToLower(text))
	tokens := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) })
	out := []string{}
	seen := map[string]bool{}
	for _, w := range tokens {
		if len(w) < 2 || strings.Contains(" de la el los las un una para por con y en al del ", " "+w+" ") || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) == 32 {
			break
		}
	}
	return out
}

// Multinomial Naive Bayes with Laplace smoothing, bounded vocabulary and no
// external models. Confidence is a heuristic, not a calibrated probability.
func classify(text string, examples []Example) Prediction {
	p := Prediction{Examples: len(examples)}
	tokens := words(text)
	if len(tokens) == 0 {
		return p
	}
	counts := map[string]map[string]int{}
	totals := map[string]int{}
	docs := map[string]int{}
	vocabulary := map[string]bool{}
	for _, e := range examples {
		if counts[e.Category] == nil {
			counts[e.Category] = map[string]int{}
		}
		docs[e.Category]++
		for _, w := range words(e.Description) {
			counts[e.Category][w]++
			totals[e.Category]++
			vocabulary[w] = true
		}
	}
	if len(counts) == 0 {
		return p
	}
	known := 0
	for _, w := range tokens {
		if vocabulary[w] {
			known++
		}
	}
	if known == 0 {
		return p
	}
	scores := map[string]float64{}
	max := math.Inf(-1)
	for cat, c := range counts {
		score := 0.0
		for _, w := range tokens {
			if vocabulary[w] {
				score += math.Log(float64(c[w]+1) / float64(totals[cat]+len(vocabulary)))
			}
		}
		scores[cat] = score
		if score > max {
			max = score
		}
	}
	sum := 0.0
	for _, v := range scores {
		sum += math.Exp(v - max)
	}
	second := 0.0
	for cat, v := range scores {
		prob := math.Exp(v-max) / sum
		if prob > p.Confidence || (prob == p.Confidence && (p.Category == "" || cat < p.Category)) {
			second = p.Confidence
			p.Confidence = prob
			p.Category = cat
		} else if prob > second {
			second = prob
		}
	}
	coverage := float64(known) / float64(len(tokens))
	p.Automatic = len(examples) >= 8 && docs[p.Category] >= 3 && coverage >= 0.75 && p.Confidence >= 0.9 && p.Confidence-second >= 0.4
	// A single trained category cannot justify confident generalization.
	if len(counts) < 2 {
		p.Automatic = false
		p.Confidence = 0.5
	}
	return p
}
func (s *Server) recordTraining(ctx context.Context, tx pgx.Tx, user, id int64, title, category, kind string, confirmed bool) error {
	if _, err := tx.Exec(ctx, "DELETE FROM training_examples WHERE movement_id=$1 AND user_id=$2", id, user); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE movements SET pending=$4,category_confirmed=$1 WHERE id=$2 AND user_id=$3", confirmed && kind == "expense" && category != "Sin categoría", id, user, kind == "expense" && category == "Sin categoría"); err != nil {
		return err
	}
	if !confirmed || kind != "expense" || category == "Sin categoría" {
		return nil
	}
	var categoryID int64
	if err := tx.QueryRow(ctx, `INSERT INTO expense_categories(user_id,name) VALUES($1,$2) ON CONFLICT(user_id,name) DO UPDATE SET name=excluded.name RETURNING id`, user, category).Scan(&categoryID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "INSERT INTO training_examples(movement_id,user_id,description,category_id) VALUES($1,$2,$3,$4)", id, user, title, categoryID)
	return err
}
func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var initialized bool
	if err = tx.QueryRow(r.Context(), "SELECT categories_initialized FROM users WHERE id=$1 FOR UPDATE", user).Scan(&initialized); err != nil {
		databaseError(w, err)
		return
	}
	if !initialized {
		for _, name := range defaults {
			if _, err = tx.Exec(r.Context(), "INSERT INTO expense_categories(user_id,name) VALUES($1,$2) ON CONFLICT DO NOTHING", user, name); err != nil {
				databaseError(w, err)
				return
			}
		}
		if _, err = tx.Exec(r.Context(), "UPDATE users SET categories_initialized=true WHERE id=$1", user); err != nil {
			databaseError(w, err)
			return
		}
	}
	rows, err := tx.Query(r.Context(), "SELECT id,name FROM expense_categories WHERE user_id=$1 ORDER BY name", user)
	if err != nil {
		databaseError(w, err)
		return
	}
	items := []Category{}
	for rows.Next() {
		var c Category
		if err = rows.Scan(&c.ID, &c.Name); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		items = append(items, c)
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
	writeJSON(w, 200, items)
}
func (s *Server) saveCategory(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if !validText(b.Name, 100) || b.Name == "Sin categoría" {
		fail(w, 400, "Nombre de categoría inválido")
		return
	}
	user := currentUser(r)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var id int64
	if r.Method == "POST" {
		err = tx.QueryRow(r.Context(), "INSERT INTO expense_categories(user_id,name) VALUES($1,$2) RETURNING id", user, b.Name).Scan(&id)
	} else {
		var ok bool
		id, ok = pathID(w, r)
		if !ok {
			return
		}
		var old string
		if err = tx.QueryRow(r.Context(), "SELECT name FROM expense_categories WHERE id=$1 AND user_id=$2 FOR UPDATE", id, user).Scan(&old); err != nil {
			fail(w, 404, "Categoría no encontrada")
			return
		}
		_, err = tx.Exec(r.Context(), "UPDATE expense_categories SET name=$1 WHERE id=$2 AND user_id=$3", b.Name, id, user)
		if err == nil {
			_, err = tx.Exec(r.Context(), "UPDATE movements SET category=$1 WHERE category=$2 AND user_id=$3 AND kind='expense'", b.Name, old, user)
		}
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]int64{"id": id})
}
func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tag, err := s.db.Exec(r.Context(), `DELETE FROM expense_categories c WHERE c.id=$1 AND c.user_id=$2 AND NOT EXISTS(SELECT 1 FROM movements m WHERE m.user_id=c.user_id AND m.category=c.name)`, id, currentUser(r))
	if err != nil {
		databaseError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "Categoría inexistente o utilizada por movimientos")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Server) prediction(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Description string `json:"description"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !validText(strings.TrimSpace(b.Description), 150) {
		fail(w, 400, "Escribe qué compraste")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT t.description,c.name FROM training_examples t JOIN expense_categories c ON c.id=t.category_id AND c.user_id=t.user_id WHERE t.user_id=$1 ORDER BY t.movement_id DESC LIMIT 10000`, currentUser(r))
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	examples := []Example{}
	for rows.Next() {
		var e Example
		if err = rows.Scan(&e.Description, &e.Category); err != nil {
			databaseError(w, err)
			return
		}
		examples = append(examples, e)
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, classify(b.Description, examples))
}
