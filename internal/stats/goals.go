package stats

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// GoalDef, hesaplanacak hedef (site.Goal ile aynı alanlar; döngüsel import olmasın diye ayrı).
type GoalDef struct {
	ID, Kind, Value string
}

// GoalResult: hedefe ulaşan tekil ziyaretçi, hedef olayının kaç kez olduğu ve
// dönüşüm oranı (ulaşan ziyaretçi / bütün ziyaretçi, yüzde).
type GoalResult struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Value    string  `json:"value"`
	Visitors int64   `json:"visitors"`
	Count    int64   `json:"count"`
	Rate     float64 `json:"rate"`
}

// goalCond, olaylar tablosunda hedefe uyan satırların koşulu. "/blog/*" gibi
// sayfa hedefleri LIKE ile önek olarak eşlenir; % ve _ karakterleri kaçışlanır.
func goalCond(alias string, g GoalDef) (string, []any) {
	if g.Kind == "event" {
		return alias + ".name = ?", []any{g.Value}
	}
	if prefix, ok := strings.CutSuffix(g.Value, "*"); ok {
		esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(prefix)
		return alias + ".name = 'pageview' AND " + alias + `.pathname LIKE ? ESCAPE '\'`, []any{esc + "%"}
	}
	return alias + ".name = 'pageview' AND " + alias + ".pathname = ?", []any{g.Value}
}

func GetGoals(ctx context.Context, db *sql.DB, q Query, goals []GoalDef) ([]GoalResult, error) {
	out := make([]GoalResult, 0, len(goals))
	if len(goals) == 0 {
		return out, nil
	}
	total, err := metrics(ctx, db, q, q.Range.From, q.Range.To)
	if err != nil {
		return nil, err
	}
	where, wargs := q.where(q.Range.From, q.Range.To)
	ew, eargs := q.eventWhere(q.Range.From, q.Range.To)
	for _, g := range goals {
		r := GoalResult{ID: g.ID, Kind: g.Kind, Value: g.Value}

		// Ulaşan ziyaretçi: filtrelere uyan ve içinde hedef olayı geçen ziyaretlerin sahipleri.
		cond, cargs := goalCond("fe", g)
		args := append(append([]any{}, wargs...), q.SiteID)
		args = append(append(args, cargs...), q.Range.From.Unix(), q.Range.To.Add(24*time.Hour).Unix())
		err := db.QueryRowContext(ctx, `SELECT uniq(s.visitor) FROM sessions s WHERE `+where+`
			AND s.id IN (SELECT fe.session_id FROM events fe WHERE fe.site_id = ? AND `+cond+` AND fe.ts >= ? AND fe.ts < ?)`,
			args...).Scan(&r.Visitors)
		if err != nil {
			return nil, err
		}

		// Kaç kez: aralıktaki hedef olayları.
		cond, cargs = goalCond("e", g)
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events e WHERE `+ew+` AND `+cond,
			append(append([]any{}, eargs...), cargs...)...).Scan(&r.Count)
		if err != nil {
			return nil, err
		}
		if total.Visitors > 0 {
			r.Rate = 100 * float64(r.Visitors) / float64(total.Visitors)
		}
		out = append(out, r)
	}
	return out, nil
}
