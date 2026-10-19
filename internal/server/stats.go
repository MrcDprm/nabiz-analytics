package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/site"
	"github.com/MrcDprm/nabiz-analytics/internal/stats"
)

// statsSite, adresteki alan adının sitesini bulur ve isteği yapanın görmeye
// yetkisi olup olmadığına bakar: site herkese açık ya da kullanıcı üyesi olmalı.
// Site yoksa da yetki yoksa da aynı 404 döner; böylece hangi sitelerin
// Nabız kullandığı dışarıdan öğrenilemez.
func (s *Server) statsSite(w http.ResponseWriter, r *http.Request) (*site.Site, bool) {
	if !s.statsLimit.allow(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "çok fazla istek, biraz bekleyin")
		return nil, false
	}
	st, err := site.ByDomain(r.Context(), s.store.R, r.PathValue("domain"))
	if errors.Is(err, site.ErrNotFound) {
		writeError(w, http.StatusNotFound, "site bulunamadı")
		return nil, false
	}
	if err != nil {
		internalError(w, "stats site", err)
		return nil, false
	}
	if st.Public {
		return st, true
	}
	u, err := s.currentUser(r)
	if err != nil {
		internalError(w, "stats oturum", err)
		return nil, false
	}
	if u != nil {
		role, err := site.Role(r.Context(), s.store.R, st.ID, u.ID)
		if err != nil {
			internalError(w, "stats rol", err)
			return nil, false
		}
		if role != "" {
			return st, true
		}
	}
	writeError(w, http.StatusNotFound, "site bulunamadı")
	return nil, false
}

// statsQuery dönem, saat dilimi ve filtreleri adresten okur.
func (s *Server) statsQuery(w http.ResponseWriter, r *http.Request, siteID string) (stats.Query, bool) {
	v := r.URL.Query()
	rng, err := stats.ParsePeriod(v.Get("period"), v.Get("from"), v.Get("to"), v.Get("tz"), time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "geçersiz dönem")
		return stats.Query{}, false
	}
	filters, err := stats.ParseFilters(v.Get("filters"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "geçersiz filtre")
		return stats.Query{}, false
	}
	return stats.Query{SiteID: siteID, Range: rng, Filters: filters}, true
}

type rangeJSON struct {
	From     time.Time      `json:"from"`
	To       time.Time      `json:"to"`
	PrevFrom time.Time      `json:"prev_from"`
	PrevTo   time.Time      `json:"prev_to"`
	Interval stats.Interval `json:"interval"`
}

func toRangeJSON(r stats.Range) rangeJSON {
	return rangeJSON{r.From, r.To, r.PrevFrom, r.PrevTo, r.Interval}
}

func (s *Server) statsSummary(w http.ResponseWriter, r *http.Request) {
	st, ok := s.statsSite(w, r)
	if !ok {
		return
	}
	q, ok := s.statsQuery(w, r, st.ID)
	if !ok {
		return
	}
	sum, err := stats.GetSummary(r.Context(), s.store.R, q)
	if err != nil {
		internalError(w, "stats özet", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"range": toRangeJSON(q.Range), "summary": sum})
}

func (s *Server) statsTimeseries(w http.ResponseWriter, r *http.Request) {
	st, ok := s.statsSite(w, r)
	if !ok {
		return
	}
	q, ok := s.statsQuery(w, r, st.ID)
	if !ok {
		return
	}
	points, err := stats.GetTimeseries(r.Context(), s.store.R, q)
	if err != nil {
		internalError(w, "stats seri", err)
		return
	}
	out := map[string]any{"range": toRangeJSON(q.Range), "points": points}
	// compare=1: önceki dönemin çubukları da (grafikteki kesikli çizgi).
	if r.URL.Query().Get("compare") == "1" {
		pq := q
		pq.Range.From, pq.Range.To = q.Range.PrevFrom, q.Range.PrevTo
		prev, err := stats.GetTimeseries(r.Context(), s.store.R, pq)
		if err != nil {
			internalError(w, "stats önceki seri", err)
			return
		}
		out["previous"] = prev
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) statsBreakdown(w http.ResponseWriter, r *http.Request) {
	st, ok := s.statsSite(w, r)
	if !ok {
		return
	}
	q, ok := s.statsQuery(w, r, st.ID)
	if !ok {
		return
	}
	prop := r.URL.Query().Get("property")
	if !stats.ValidBreakdown(prop) {
		writeError(w, http.StatusBadRequest, "geçersiz kırılım")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	rows, err := stats.GetBreakdown(r.Context(), s.store.R, q, prop, limit, offset)
	if err != nil {
		internalError(w, "stats kırılım", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"property": prop, "rows": rows})
}

func (s *Server) statsLive(w http.ResponseWriter, r *http.Request) {
	st, ok := s.statsSite(w, r)
	if !ok {
		return
	}
	live, err := stats.GetLive(r.Context(), s.store.R, st.ID, time.Now())
	if err != nil {
		internalError(w, "stats canlı", err)
		return
	}
	writeJSON(w, http.StatusOK, live)
}

func (s *Server) statsGoals(w http.ResponseWriter, r *http.Request) {
	st, ok := s.statsSite(w, r)
	if !ok {
		return
	}
	q, ok := s.statsQuery(w, r, st.ID)
	if !ok {
		return
	}
	goals, err := site.Goals(r.Context(), s.store.R, st.ID)
	if err != nil {
		internalError(w, "stats hedefler", err)
		return
	}
	defs := make([]stats.GoalDef, len(goals))
	for i, g := range goals {
		defs[i] = stats.GoalDef{ID: g.ID, Kind: g.Kind, Value: g.Value}
	}
	res, err := stats.GetGoals(r.Context(), s.store.R, q, defs)
	if err != nil {
		internalError(w, "stats hedefler", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"goals": res})
}
