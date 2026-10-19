// Package server, Nabız'ın HTTP uçlarını bir araya getirir.
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

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
}

type Server struct {
	cfg      Config
	store    *store.Store
	recorder *ingest.Recorder
	mux      *http.ServeMux

	// IP başına: saniyede 20 olay, art arda en fazla 60. Aynı ağdaki (ör. bir
	// ofis) birçok kişi tek IP'den gelebilir, o yüzden cömert.
	eventLimit *limiter
}

func New(cfg Config, st *store.Store, rec *ingest.Recorder) *Server {
	s := &Server{
		cfg: cfg, store: st, recorder: rec, mux: http.NewServeMux(),
		eventLimit: newLimiter(20, 60, nil),
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/event", s.event)
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
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
