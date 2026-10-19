package stats

import (
	"context"
	"database/sql"
	"strings"
)

// StepResult: huninin bir adımına ulaşan tekil ziyaretçi.
// Rate ilk adıma göre, Drop bir önceki adıma göre kayıp (yüzde).
type StepResult struct {
	Kind     string  `json:"kind"`
	Value    string  `json:"value"`
	Visitors int64   `json:"visitors"`
	Rate     float64 `json:"rate"`
	Drop     float64 `json:"drop"`
}

// matches, bir olayın adıma uyup uymadığını söyler (goalCond'un Go'daki karşılığı).
func matches(step GoalDef, name, path string) bool {
	if step.Kind == "event" {
		return name == step.Value
	}
	if name != "pageview" {
		return false
	}
	if prefix, ok := strings.CutSuffix(step.Value, "*"); ok {
		return strings.HasPrefix(path, prefix)
	}
	return path == step.Value
}

// GetFunnel, ziyaretlerin adımlardan sırayla geçişini sayar. Bir ziyaret 2. adıma
// ancak 1. adımdan sonra geçerse sayılır; aradaki başka sayfalar sorun değil.
// Önce adımlardan herhangi birine uyan olaylar ziyaret ve zaman sırasıyla çekilir,
// sonra her ziyaret Go'da baştan sona yürünür.
func GetFunnel(ctx context.Context, db *sql.DB, q Query, steps []GoalDef) ([]StepResult, error) {
	ew, args := q.eventWhere(q.Range.From, q.Range.To)
	var conds []string
	for _, st := range steps {
		c, cargs := goalCond("e", st)
		conds = append(conds, "("+c+")")
		args = append(args, cargs...)
	}
	rows, err := db.QueryContext(ctx, `SELECT e.session_id, e.visitor, e.name, e.pathname FROM events e
		WHERE `+ew+` AND (`+strings.Join(conds, " OR ")+`) ORDER BY e.session_id, e.ts, e.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reached := make([]map[int64]struct{}, len(steps))
	for i := range reached {
		reached[i] = make(map[int64]struct{})
	}
	var cur int64 = -1
	next := 0
	for rows.Next() {
		var session, visitor int64
		var name, path string
		if err := rows.Scan(&session, &visitor, &name, &path); err != nil {
			return nil, err
		}
		if session != cur {
			cur, next = session, 0
		}
		if next < len(steps) && matches(steps[next], name, path) {
			reached[next][visitor] = struct{}{}
			next++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]StepResult, len(steps))
	for i, st := range steps {
		r := StepResult{Kind: st.Kind, Value: st.Value, Visitors: int64(len(reached[i]))}
		if first := len(reached[0]); first > 0 {
			r.Rate = 100 * float64(r.Visitors) / float64(first)
		}
		if i > 0 {
			if prev := len(reached[i-1]); prev > 0 {
				r.Drop = 100 * (1 - float64(r.Visitors)/float64(prev))
			}
		}
		out[i] = r
	}
	return out, nil
}

// PropKeys, bir olayın özellik adları (ör. "CV indir" → "dosya", "dil").
func PropKeys(ctx context.Context, db *sql.DB, q Query, event string) ([]Row, error) {
	ew, args := q.eventWhere(q.Range.From, q.Range.To)
	return rowsOf(ctx, db, `SELECT p.key, uniq(e.visitor), COUNT(*) FROM events e
		JOIN event_props p ON p.event_id = e.id
		WHERE e.name = ? AND `+ew+` GROUP BY 1 ORDER BY 2 DESC, 3 DESC, 1 LIMIT 20`,
		append([]any{event}, args...)...)
}

// PropValues, bir özelliğin değerleri (ör. "dosya" → "cv-tr.pdf", "cv-en.pdf").
func PropValues(ctx context.Context, db *sql.DB, q Query, event, key string, limit int) ([]Row, error) {
	limit = min(max(limit, 1), MaxBreakdownLimit)
	ew, args := q.eventWhere(q.Range.From, q.Range.To)
	args = append([]any{event, key}, args...)
	return rowsOf(ctx, db, `SELECT p.value, uniq(e.visitor), COUNT(*) FROM events e
		JOIN event_props p ON p.event_id = e.id
		WHERE e.name = ? AND p.key = ? AND `+ew+` GROUP BY 1 ORDER BY 2 DESC, 3 DESC, 1 LIMIT ?`,
		append(args, limit)...)
}

func rowsOf(ctx context.Context, db *sql.DB, query string, args ...any) ([]Row, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.Value, &r.Visitors, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
