package server

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/site"
	"github.com/MrcDprm/nabiz-analytics/internal/stats"
)

// safeCell, Excel/LibreOffice'in formül olarak çalıştıracağı hücreleri etkisizleştirir
// (CSV enjeksiyonu). Ziyaretçiden gelen bir sayfa adı "=HYPERLINK(...)" olabilir;
// başına ' eklenince sadece metin olarak görünür.
func safeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// Sütun başlıkları; lang=tr ile Türkçe.
var exportHeads = map[string]map[string]string{
	"en": {
		"date": "date", "visitors": "visitors", "visits": "visits", "pageviews": "pageviews",
		"bounce_rate": "bounce_rate", "visit_duration": "visit_duration_seconds", "value": "value",
		"count": "count", "goal": "goal", "kind": "kind", "conversion_rate": "conversion_rate",
		"event": "event", "property": "property",
	},
	"tr": {
		"date": "tarih", "visitors": "ziyaretci", "visits": "ziyaret", "pageviews": "goruntuleme",
		"bounce_rate": "hemen_cikma_orani", "visit_duration": "ortalama_sure_saniye", "value": "deger",
		"count": "sayi", "goal": "hedef", "kind": "tur", "conversion_rate": "donusum_orani",
		"event": "olay", "property": "ozellik",
	},
}

// Dışa aktarılan listeler: dosya adı → kırılım.
var exportLists = []struct{ file, prop string }{
	{"pages.csv", "page"}, {"entry_pages.csv", "entry"}, {"exit_pages.csv", "exit"},
	{"sources.csv", "source"}, {"utm_sources.csv", "utm_source"}, {"utm_mediums.csv", "utm_medium"},
	{"utm_campaigns.csv", "utm_campaign"}, {"countries.csv", "country"}, {"devices.csv", "device"},
	{"browsers.csv", "browser"}, {"operating_systems.csv", "os"}, {"events.csv", "event"},
}

// utf8BOM: Excel bu üç baytı görünce dosyayı UTF-8 okur (yoksa "ş" bozuk görünür).
const utf8BOM = "\xef\xbb\xbf"

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
func i64(v int64) string  { return strconv.FormatInt(v, 10) }

// writeCSV, UTF-8 BOM ile başlayan bir CSV yazar.
func writeCSV(zw *zip.Writer, name string, head []string, rows [][]string) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, utf8BOM); err != nil {
		return err
	}
	cw := csv.NewWriter(f)
	cw.Write(head)
	for _, r := range rows {
		for i := range r {
			r[i] = safeCell(r[i])
		}
		cw.Write(r)
	}
	cw.Flush()
	return cw.Error()
}

// export, GET /api/stats/{domain}/export: seçili dönem ve filtrelerle bütün
// listeleri tek bir ZIP olarak verir. Herkese açık panelde bile sadece site üyeleri indirebilir.
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	if !s.statsLimit.allow(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "çok fazla istek, biraz bekleyin")
		return
	}
	u, err := s.currentUser(r)
	if err != nil {
		internalError(w, "export oturum", err)
		return
	}
	st, err := site.ByDomain(r.Context(), s.store.R, r.PathValue("domain"))
	if err != nil && !errors.Is(err, site.ErrNotFound) {
		internalError(w, "export site", err)
		return
	}
	role := ""
	if st != nil && u != nil {
		if role, err = site.Role(r.Context(), s.store.R, st.ID, u.ID); err != nil {
			internalError(w, "export rol", err)
			return
		}
	}
	if role == "" {
		writeError(w, http.StatusNotFound, "site bulunamadı")
		return
	}
	q, ok := s.statsQuery(w, r, st.ID)
	if !ok {
		return
	}
	h := exportHeads["en"]
	if r.URL.Query().Get("lang") == "tr" {
		h = exportHeads["tr"]
	}

	// Önce bellekte bir ZIP oluşturulur: bir sorgu yarıda hata verirse kullanıcı
	// bozuk dosya yerine düzgün bir hata mesajı alır.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	ctx := r.Context()

	points, err := stats.GetTimeseries(ctx, s.store.R, q)
	if err != nil {
		internalError(w, "export seri", err)
		return
	}
	layout := "2006-01-02"
	if q.Range.Interval == stats.Hour {
		layout = "2006-01-02 15:04"
	} else if q.Range.Interval == stats.Month {
		layout = "2006-01"
	}
	var rows [][]string
	for _, p := range points {
		rows = append(rows, []string{p.Start.In(q.Range.Loc).Format(layout), i64(p.Visitors), i64(p.Visits), i64(p.Pageviews), f1(p.BounceRate), f1(p.VisitLength)})
	}
	if err := writeCSV(zw, "visitors.csv", []string{h["date"], h["visitors"], h["visits"], h["pageviews"], h["bounce_rate"], h["visit_duration"]}, rows); err != nil {
		internalError(w, "export yaz", err)
		return
	}

	for _, l := range exportLists {
		list, err := stats.ExportBreakdown(ctx, s.store.R, q, l.prop)
		if err != nil {
			internalError(w, "export liste", err)
			return
		}
		rows = rows[:0]
		for _, row := range list {
			rows = append(rows, []string{row.Value, i64(row.Visitors), i64(row.Count)})
		}
		countHead := h["visits"]
		if l.prop == "page" || l.prop == "event" {
			countHead = h["count"]
		}
		if err := writeCSV(zw, l.file, []string{h["value"], h["visitors"], countHead}, rows); err != nil {
			internalError(w, "export yaz", err)
			return
		}
	}

	goals, err := site.Goals(ctx, s.store.R, st.ID)
	if err != nil {
		internalError(w, "export hedef", err)
		return
	}
	defs := make([]stats.GoalDef, len(goals))
	for i, g := range goals {
		defs[i] = stats.GoalDef{ID: g.ID, Kind: g.Kind, Value: g.Value}
	}
	gres, err := stats.GetGoals(ctx, s.store.R, q, defs)
	if err != nil {
		internalError(w, "export hedef", err)
		return
	}
	rows = rows[:0]
	for _, g := range gres {
		rows = append(rows, []string{g.Value, g.Kind, i64(g.Visitors), i64(g.Count), f1(g.Rate)})
	}
	if err := writeCSV(zw, "goals.csv", []string{h["goal"], h["kind"], h["visitors"], h["count"], h["conversion_rate"]}, rows); err != nil {
		internalError(w, "export yaz", err)
		return
	}

	props, err := stats.AllProps(ctx, s.store.R, q)
	if err != nil {
		internalError(w, "export özellik", err)
		return
	}
	rows = rows[:0]
	for _, p := range props {
		rows = append(rows, []string{p.Event, p.Key, p.Value, i64(p.Visitors), i64(p.Count)})
	}
	if err := writeCSV(zw, "event_properties.csv", []string{h["event"], h["property"], h["value"], h["visitors"], h["count"]}, rows); err != nil {
		internalError(w, "export yaz", err)
		return
	}
	if err := zw.Close(); err != nil {
		internalError(w, "export zip", err)
		return
	}

	// Dosya adı: nabiz-example.com-2026-10-13-2026-10-19.zip (alan adı zaten doğrulanmış).
	last := q.Range.To.Add(-time.Second)
	name := fmt.Sprintf("nabiz-%s-%s-%s.zip", st.Domain, q.Range.From.Format("2006-01-02"), last.Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(buf.Bytes())
}
