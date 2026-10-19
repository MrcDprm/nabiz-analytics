// Package server, Nabız'ın HTTP uçlarını bir araya getirir.
package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/ingest"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

// Mode sunucunun nasıl çalıştığını seçer: kendi sunucunda (paket yok) ya da
// barındırılan hizmet (kayıt + ücretli paketler).
type Mode string

const (
	ModeSelfhost Mode = "selfhost"
	ModeCloud    Mode = "cloud"
)

type Config struct {
	Mode       Mode
	TrustProxy bool // Caddy arkasında: istemci IP'si X-Forwarded-For'dan okunur
	Version    string
	Tracker    []byte // derlenmiş nabiz.js; nil ise /js/nabiz.js 404 döner
	Web        fs.FS  // derlenmiş panel (web/dist); nil ise sadece API çalışır
}

type Server struct {
	cfg      Config
	store    *store.Store
	recorder *ingest.Recorder
	auth     *auth.Service
	mux      *http.ServeMux
	csrf     *http.CrossOriginProtection

	// IP başına: saniyede 20 olay, art arda en fazla 60. Aynı ağdaki (ör. bir
	// ofis) birçok kişi tek IP'den gelebilir, o yüzden cömert.
	eventLimit *limiter
	// IP başına: dakikada 5 giriş denemesi.
	loginLimit *limiter
	// IP başına: saniyede 10 panel sorgusu, art arda 40 (bir panel ~10 istek atar).
	statsLimit *limiter
}

func New(cfg Config, st *store.Store, rec *ingest.Recorder, a *auth.Service) *Server {
	s := &Server{
		cfg: cfg, store: st, recorder: rec, auth: a, mux: http.NewServeMux(),
		csrf:       http.NewCrossOriginProtection(),
		eventLimit: newLimiter(20, 60, nil),
		loginLimit: newLimiter(5.0/60, 5, nil),
		statsLimit: newLimiter(10, 40, nil),
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/event", s.event)
	s.mux.HandleFunc("GET /js/nabiz.js", s.tracker)

	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("POST /api/logout", s.logout)
	s.mux.HandleFunc("GET /api/me", s.requireUser(s.me))
	s.mux.HandleFunc("GET /api/sites", s.requireUser(s.listSites))
	s.mux.HandleFunc("POST /api/sites", s.requireUser(s.createSite))
	s.mux.HandleFunc("PATCH /api/sites/{id}", s.requireUser(s.updateSite))
	s.mux.HandleFunc("DELETE /api/sites/{id}", s.requireUser(s.deleteSite))
	s.mux.HandleFunc("GET /api/sites/{id}/goals", s.requireUser(s.listGoals))
	s.mux.HandleFunc("POST /api/sites/{id}/goals", s.requireUser(s.addGoal))
	s.mux.HandleFunc("DELETE /api/sites/{id}/goals/{goal}", s.requireUser(s.deleteGoal))

	s.mux.HandleFunc("GET /api/stats/{domain}/summary", s.statsSummary)
	s.mux.HandleFunc("GET /api/stats/{domain}/timeseries", s.statsTimeseries)
	s.mux.HandleFunc("GET /api/stats/{domain}/breakdown", s.statsBreakdown)
	s.mux.HandleFunc("GET /api/stats/{domain}/live", s.statsLive)
	s.mux.HandleFunc("GET /api/stats/{domain}/goals", s.statsGoals)
	s.webRoutes()
	return s
}

// Run arka plan temizliklerini dakikada bir çalıştırır; ctx bitince durur.
func (s *Server) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.eventLimit.sweep()
			s.loginLimit.sweep()
			s.statsLimit.sweep()
			if err := s.auth.Sweep(ctx); err != nil {
				log.Printf("oturum temizliği: %v", err)
			}
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	// CSRF: başka bir siteden gelen POST/PATCH/DELETE isteklerini reddet
	// (tarayıcının Sec-Fetch-Site ve Origin başlıklarına bakar). Takip ucu
	// bilerek başka sitelerden çağrıldığı için hariç; o Origin'i kendisi denetler.
	if r.URL.Path != "/api/event" {
		if err := s.csrf.Check(r); err != nil {
			writeError(w, http.StatusForbidden, "başka bir siteden gelen istek reddedildi")
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		log.Printf("healthz: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.cfg.Version})
}

// tracker, takip betiğini sunar. Bir saatlik önbellek: betik güncellenince
// siteler en geç bir saat içinde yenisini alır.
func (s *Server) tracker(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Tracker == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	// Betik başka sitelerin sayfalarında çalışır; tarayıcının bunu engellememesi için.
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	_, _ = w.Write(s.cfg.Tracker)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
