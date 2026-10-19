package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Ziyaret (oturum) düzeyindeki boyutlar: panel adı → sütun. Sütun adları SQL'e
// sadece bu listeden girer; kullanıcının yazdığı değer her zaman parametredir.
var sessionDims = map[string]string{
	"source":       "s.referrer",
	"utm_source":   "s.utm_source",
	"utm_medium":   "s.utm_medium",
	"utm_campaign": "s.utm_campaign",
	"country":      "s.country",
	"device":       "s.device",
	"browser":      "s.browser",
	"os":           "s.os",
	"entry":        "s.entry_page",
	"exit":         "s.exit_page",
}

// Filter: "ülke = TR" gibi tek bir süzgeç. Page ve Event olay düzeyindedir.
type Filter struct {
	Dim   string
	Value string
}

var ErrBadFilter = errors.New("geçersiz filtre")

const maxFilters = 10

// ParseFilters, adresteki filters=[["country","TR"],["page","/x"]] değerini çözer.
// JSON kullanıyoruz çünkü sayfa yolları ":" ve ";" içerebilir.
func ParseFilters(raw string) ([]Filter, error) {
	if raw == "" {
		return nil, nil
	}
	var pairs [][2]string
	if err := json.Unmarshal([]byte(raw), &pairs); err != nil || len(pairs) > maxFilters {
		return nil, ErrBadFilter
	}
	out := make([]Filter, 0, len(pairs))
	for _, p := range pairs {
		dim, val := p[0], p[1]
		if _, ok := sessionDims[dim]; !ok && dim != "page" && dim != "event" {
			return nil, ErrBadFilter
		}
		if utf8.RuneCountInString(val) > 512 || ((dim == "page" || dim == "event") && val == "") {
			return nil, ErrBadFilter
		}
		out = append(out, Filter{dim, val})
	}
	return out, nil
}

// Query, bir panel isteğinin ortak kısmı.
type Query struct {
	SiteID  string
	Range   Range
	Filters []Filter
}

// where, oturum (s) tablosu için WHERE koşulunu ve parametrelerini üretir.
// Sayfa ve olay filtreleri "bu ziyarette o sayfa/olay var mı" olarak uygulanır.
func (q Query) where(from, to time.Time) (string, []any) {
	var b strings.Builder
	args := []any{q.SiteID, from.Unix(), to.Unix()}
	b.WriteString("s.site_id = ? AND s.started_at >= ? AND s.started_at < ?")
	for _, f := range q.Filters {
		switch f.Dim {
		case "page", "event":
			// "Bu ziyarette o sayfa/olay var mı": önce aralıktaki eşleşen olayların
			// ziyaretlerini (events_page indeksinden) bulup sonra ziyaretleri süzmek,
			// her ziyaret için ayrı ayrı bakmaktan (EXISTS) onlarca kat hızlı.
			// Ziyaret aralığın sonuna yakın başlayıp ertesi güne taşabilir: +1 gün.
			name, path := "?", "fe.pathname = ? AND "
			eargs := []any{q.SiteID, "pageview", f.Value}
			if f.Dim == "event" {
				path = ""
				eargs = []any{q.SiteID, f.Value}
			}
			b.WriteString(" AND s.id IN (SELECT fe.session_id FROM events fe WHERE fe.site_id = ? AND fe.name = " + name +
				" AND " + path + "fe.ts >= ? AND fe.ts < ?)")
			args = append(args, append(eargs, from.Unix(), to.Add(24*time.Hour).Unix())...)
		default:
			b.WriteString(" AND " + sessionDims[f.Dim] + " = ?")
			args = append(args, f.Value)
		}
	}
	return b.String(), args
}

func (q Query) hasPageFilter() bool {
	for _, f := range q.Filters {
		if f.Dim == "page" {
			return true
		}
	}
	return false
}

// eventWhere, olaylar (e) tablosu için koşul: olay zamanı aralıkta, sayfa
// filtresi olayın kendi yoluna uygulanır. Ziyaret düzeyinde (kaynak, ülke, olay...)
// filtre varsa olay, filtreye uyan bir ziyarete ait olmalı; yoksa sessions
// tablosuna hiç gidilmez (hızlı yol).
func (q Query) eventWhere(from, to time.Time) (string, []any) {
	var b strings.Builder
	args := []any{q.SiteID, from.Unix(), to.Unix()}
	b.WriteString("e.site_id = ? AND e.ts >= ? AND e.ts < ?")
	var visit []Filter
	for _, f := range q.Filters {
		if f.Dim == "page" {
			b.WriteString(" AND e.pathname = ?")
			args = append(args, f.Value)
		} else {
			visit = append(visit, f)
		}
	}
	if len(visit) > 0 {
		// Ziyaret aralıktan biraz önce başlamış olabilir; bir gün geriden bakıyoruz.
		sub := Query{SiteID: q.SiteID, Filters: visit}
		w, wargs := sub.where(from.Add(-24*time.Hour), to)
		b.WriteString(" AND e.session_id IN (SELECT s.id FROM sessions s WHERE " + w + ")")
		args = append(args, wargs...)
	}
	return b.String(), args
}

// Metrics bir dönemin özeti.
type Metrics struct {
	Visitors    int64   `json:"visitors"`
	Visits      int64   `json:"visits"`
	Pageviews   int64   `json:"pageviews"`
	BounceRate  float64 `json:"bounce_rate"`  // 0-100
	VisitLength float64 `json:"visit_length"` // saniye, ortalama
}

// sessionAggregates: ziyaretçi, ziyaret, görüntüleme, hemen çıkma, süre.
// Hemen çıkma = tek sayfa görüntülemeli ziyaret; hiç görüntülemesi olmayan
// (sadece özel olay) ziyaretler orana katılmaz. Görüntüleme sayısı ziyaret
// satırında hazır durduğu için olaylar tablosuna gitmeye gerek yok.
const sessionAggregates = `uniq(s.visitor), COUNT(*), COALESCE(SUM(s.pageviews), 0),
	COALESCE(100.0 * SUM(s.pageviews = 1) / NULLIF(SUM(s.pageviews >= 1), 0), 0),
	COALESCE(AVG(s.ended_at - s.started_at), 0)`

func metrics(ctx context.Context, db *sql.DB, q Query, from, to time.Time) (Metrics, error) {
	var m Metrics
	where, args := q.where(from, to)
	err := db.QueryRowContext(ctx, `SELECT `+sessionAggregates+` FROM sessions s WHERE `+where, args...).
		Scan(&m.Visitors, &m.Visits, &m.Pageviews, &m.BounceRate, &m.VisitLength)
	if err != nil || !q.hasPageFilter() {
		return m, err
	}
	// Sayfa süzülüyken görüntüleme = o sayfanın görüntülenmesi (ziyaretteki diğer sayfalar değil).
	ew, eargs := q.eventWhere(from, to)
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events e WHERE e.name = 'pageview' AND `+ew, eargs...).Scan(&m.Pageviews)
	return m, err
}

// Summary, seçili dönemin ve önceki dönemin özeti.
type Summary struct {
	Current  Metrics `json:"current"`
	Previous Metrics `json:"previous"`
}

func GetSummary(ctx context.Context, db *sql.DB, q Query) (Summary, error) {
	cur, err := metrics(ctx, db, q, q.Range.From, q.Range.To)
	if err != nil {
		return Summary{}, err
	}
	prev, err := metrics(ctx, db, q, q.Range.PrevFrom, q.Range.PrevTo)
	return Summary{Current: cur, Previous: prev}, err
}

// Point, zaman grafiğinde bir çubuk.
type Point struct {
	Start       time.Time `json:"start"`
	Visitors    int64     `json:"visitors"`
	Visits      int64     `json:"visits"`
	Pageviews   int64     `json:"pageviews"`
	BounceRate  float64   `json:"bounce_rate"`
	VisitLength float64   `json:"visit_length"`
}

// bucketsCTE, çubuk sınırlarını SQL'e küçük bir tablo olarak verir:
// WITH b(i, lo, hi) AS (VALUES (?, ?, ?), ...). Saat dilimi ve yaz saati
// hesabı Go'da yapıldığı için SQL tarafı sadece sayıları karşılaştırır.
func bucketsCTE(starts []time.Time, end time.Time) (string, []any) {
	var b strings.Builder
	args := make([]any, 0, len(starts)*3)
	b.WriteString("WITH b(i, lo, hi) AS (VALUES ")
	for i, t := range starts {
		hi := end
		if i+1 < len(starts) {
			hi = starts[i+1]
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("(?, ?, ?)")
		args = append(args, i, t.Unix(), hi.Unix())
	}
	b.WriteString(") ")
	return b.String(), args
}

func GetTimeseries(ctx context.Context, db *sql.DB, q Query) ([]Point, error) {
	starts := q.Range.Buckets()
	points := make([]Point, len(starts))
	for i, t := range starts {
		points[i].Start = t
	}
	cte, cargs := bucketsCTE(starts, q.Range.To)
	where, args := q.where(q.Range.From, q.Range.To)

	rows, err := db.QueryContext(ctx, cte+`SELECT b.i, `+sessionAggregates+`
		FROM sessions s JOIN b ON s.started_at >= b.lo AND s.started_at < b.hi
		WHERE `+where+` GROUP BY b.i`, append(cargs, args...)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var i int
		var p Point
		if err := rows.Scan(&i, &p.Visitors, &p.Visits, &p.Pageviews, &p.BounceRate, &p.VisitLength); err != nil {
			rows.Close()
			return nil, err
		}
		p.Start = points[i].Start
		points[i] = p
	}
	rows.Close()
	if err := rows.Err(); err != nil || !q.hasPageFilter() {
		return points, err
	}

	// Sayfa süzülüyken görüntüleme sayısı olaylardan, olayın kendi zamanına göre.
	for i := range points {
		points[i].Pageviews = 0
	}
	ew, eargs := q.eventWhere(q.Range.From, q.Range.To)
	rows, err = db.QueryContext(ctx, cte+`SELECT b.i, COUNT(*)
		FROM events e JOIN b ON e.ts >= b.lo AND e.ts < b.hi
		WHERE e.name = 'pageview' AND `+ew+` GROUP BY b.i`, append(cargs, eargs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var i int
		var n int64
		if err := rows.Scan(&i, &n); err != nil {
			return nil, err
		}
		points[i].Pageviews = n
	}
	return points, rows.Err()
}

// Row, bir kırılım listesindeki satır (ör. "google.com: 410 ziyaretçi").
type Row struct {
	Value    string `json:"value"`
	Visitors int64  `json:"visitors"`
	Count    int64  `json:"count"` // sayfalarda görüntüleme, olaylarda olay sayısı, diğerlerinde ziyaret
}

// Breakdown boyutları: oturum boyutlarının yanında "page" ve "event".
func ValidBreakdown(prop string) bool {
	_, ok := sessionDims[prop]
	return ok || prop == "page" || prop == "event"
}

const MaxBreakdownLimit = 100

// MaxExportRows: CSV dışa aktarmada bir listenin en fazla satırı.
const MaxExportRows = 1000

func GetBreakdown(ctx context.Context, db *sql.DB, q Query, prop string, limit, offset int) ([]Row, error) {
	return breakdown(ctx, db, q, prop, min(max(limit, 1), MaxBreakdownLimit), max(offset, 0))
}

// ExportBreakdown, panel sınırı yerine dışa aktarma sınırıyla aynı listeyi verir.
func ExportBreakdown(ctx context.Context, db *sql.DB, q Query, prop string) ([]Row, error) {
	return breakdown(ctx, db, q, prop, MaxExportRows, 0)
}

func breakdown(ctx context.Context, db *sql.DB, q Query, prop string, limit, offset int) ([]Row, error) {
	if !ValidBreakdown(prop) {
		return nil, ErrBadFilter
	}

	var query string
	var args []any
	switch prop {
	case "page", "event":
		col, cond := "e.pathname", "e.name = 'pageview'"
		if prop == "event" {
			col, cond = "e.name", "e.name <> 'pageview'"
		}
		ew, eargs := q.eventWhere(q.Range.From, q.Range.To)
		query = `SELECT ` + col + `, uniq(e.visitor), COUNT(*) FROM events e
			WHERE ` + cond + ` AND ` + ew + `
			GROUP BY 1 ORDER BY 2 DESC, 3 DESC, 1 LIMIT ? OFFSET ?`
		args = eargs
	default:
		where, wargs := q.where(q.Range.From, q.Range.To)
		query = fmt.Sprintf(`SELECT %s, uniq(s.visitor), COUNT(*)
			FROM sessions s WHERE %s
			GROUP BY 1 ORDER BY 2 DESC, 3 DESC, 1 LIMIT ? OFFSET ?`, sessionDims[prop], where)
		args = wargs
	}
	rows, err := db.QueryContext(ctx, query, append(args, limit, offset)...)
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

// LiveWindow: "şu an sitede" sayılan süre.
const LiveWindow = 5 * time.Minute

// Live, son 5 dakikadaki tekil ziyaretçi ve görüntüleme zamanları (nabız şeridi için).
type Live struct {
	Visitors int64   `json:"visitors"`
	Beats    []int64 `json:"beats"` // her görüntülemenin Unix zamanı (saniye)
}

func GetLive(ctx context.Context, db *sql.DB, siteID string, now time.Time) (Live, error) {
	since := now.Add(-LiveWindow).Unix()
	l := Live{Beats: []int64{}}
	// started_at koşulu indeksi kullanabilmek için: bir günden uzun süren ziyaret olmaz.
	if err := db.QueryRowContext(ctx, `SELECT uniq(visitor) FROM sessions
		WHERE site_id = ? AND started_at >= ? AND started_at <= ? AND ended_at >= ?`,
		siteID, now.Add(-24*time.Hour).Unix(), now.Unix(), since).Scan(&l.Visitors); err != nil {
		return l, err
	}
	rows, err := db.QueryContext(ctx, `SELECT ts FROM events
		WHERE site_id = ? AND ts >= ? AND ts <= ? AND name = 'pageview' ORDER BY ts LIMIT 500`, siteID, since, now.Unix())
	if err != nil {
		return l, err
	}
	defer rows.Close()
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			return l, err
		}
		l.Beats = append(l.Beats, ts)
	}
	return l, rows.Err()
}
