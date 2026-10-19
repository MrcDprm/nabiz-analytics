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
	"github.com/MrcDprm/nabiz-analytics/internal/billing"
	"github.com/MrcDprm/nabiz-analytics/internal/ingest"
	"github.com/MrcDprm/nabiz-analytics/internal/mail"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
	"github.com/MrcDprm/nabiz-analytics/internal/team"
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
	BaseURL    string // e-postalardaki bağlantılar için (ör. https://nabiz.example.com)
	Signup     bool   // herkese açık kayıt açık mı
	Mail       mail.Sender
	Billing    *billing.Service // nil: ödeme ve limit yok (kendi sunucunda kurulum)
	DemoSite   string           // tanıtım sayfasında canlı nabzı gösterilen herkese açık site
	// Ödeme açıksa satış sözleşmelerinin adresleri (işletmecinin kendi metinleri).
	SalesContractURL, PreInfoURL string
}

type Server struct {
	cfg      Config
	store    *store.Store
	recorder *ingest.Recorder
	auth     *auth.Service
	team     *team.Team
	mux      *http.ServeMux
	csrf     *http.CrossOriginProtection

	// IP başına: saniyede 20 olay, art arda en fazla 60. Aynı ağdaki (ör. bir
	// ofis) birçok kişi tek IP'den gelebilir, o yüzden cömert.
	eventLimit *limiter
	// IP başına: dakikada 5 giriş denemesi.
	loginLimit *limiter
	// IP başına: saniyede 10 panel sorgusu, art arda 40 (bir panel ~10 istek atar).
	statsLimit *limiter
	// IP başına: saatte 5 e-posta tetikleyen istek (kayıt, şifremi unuttum, tekrar gönder).
	mailLimit *limiter
	// IP başına: dakikada 10 şifre gerektiren hesap işlemi (değiştir, sil, sıfırla, doğrula).
	accountLimit *limiter
}

func New(cfg Config, st *store.Store, rec *ingest.Recorder, a *auth.Service) *Server {
	s := &Server{
		cfg: cfg, store: st, recorder: rec, auth: a, team: team.New(st, nil), mux: http.NewServeMux(),
		csrf:         http.NewCrossOriginProtection(),
		eventLimit:   newLimiter(20, 60, nil),
		loginLimit:   newLimiter(5.0/60, 5, nil),
		statsLimit:   newLimiter(10, 40, nil),
		mailLimit:    newLimiter(5.0/3600, 5, nil),
		accountLimit: newLimiter(10.0/60, 10, nil),
	}
	if s.cfg.Mail == nil {
		s.cfg.Mail = mail.LogSender{}
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/event", s.event)
	s.mux.HandleFunc("GET /js/nabiz.js", s.tracker)

	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("POST /api/logout", s.logout)
	s.mux.HandleFunc("GET /api/me", s.requireUser(s.me))
	s.mux.HandleFunc("GET /api/config", s.config)
	s.mux.HandleFunc("POST /api/signup", s.signup)
	s.mux.HandleFunc("POST /api/verify", s.verifyEmail)
	s.mux.HandleFunc("POST /api/verify/resend", s.requireUser(s.resendVerification))
	s.mux.HandleFunc("POST /api/password/forgot", s.forgotPassword)
	s.mux.HandleFunc("POST /api/password/reset", s.resetPassword)
	s.mux.HandleFunc("PATCH /api/account", s.requireUser(s.updateAccount))
	s.mux.HandleFunc("POST /api/account/password", s.requireUser(s.changePassword))
	s.mux.HandleFunc("POST /api/account/email", s.requireUser(s.changeEmail))
	s.mux.HandleFunc("DELETE /api/account", s.requireUser(s.deleteAccount))
	s.mux.HandleFunc("GET /api/sites", s.requireUser(s.listSites))
	s.mux.HandleFunc("POST /api/sites", s.requireUser(s.createSite))
	s.mux.HandleFunc("PATCH /api/sites/{id}", s.requireUser(s.updateSite))
	s.mux.HandleFunc("DELETE /api/sites/{id}", s.requireUser(s.deleteSite))
	s.mux.HandleFunc("GET /api/sites/{id}/goals", s.requireUser(s.listGoals))
	s.mux.HandleFunc("POST /api/sites/{id}/goals", s.requireUser(s.addGoal))
	s.mux.HandleFunc("DELETE /api/sites/{id}/goals/{goal}", s.requireUser(s.deleteGoal))
	s.mux.HandleFunc("GET /api/sites/{id}/funnels", s.requireUser(s.listFunnels))
	s.mux.HandleFunc("POST /api/sites/{id}/funnels", s.requireUser(s.addFunnel))
	s.mux.HandleFunc("DELETE /api/sites/{id}/funnels/{funnel}", s.requireUser(s.deleteFunnel))
	s.mux.HandleFunc("GET /api/sites/{id}/members", s.requireUser(s.listMembers))
	s.mux.HandleFunc("POST /api/sites/{id}/invites", s.requireUser(s.invite))
	s.mux.HandleFunc("DELETE /api/sites/{id}/invites/{email}", s.requireUser(s.revokeInvite))
	s.mux.HandleFunc("PATCH /api/sites/{id}/members/{email}", s.requireUser(s.setMemberRole))
	s.mux.HandleFunc("DELETE /api/sites/{id}/members/{email}", s.requireUser(s.removeMember))
	s.mux.HandleFunc("POST /api/sites/{id}/transfer", s.requireUser(s.transferSite))
	s.mux.HandleFunc("POST /api/invites/info", s.inviteInfo)
	s.mux.HandleFunc("POST /api/invites/accept", s.acceptInvite)
	s.mux.HandleFunc("GET /api/billing", s.requireUser(s.billingStatus))
	s.mux.HandleFunc("POST /api/billing/checkout", s.requireUser(s.checkout))
	s.mux.HandleFunc("POST /api/billing/cancel", s.requireUser(s.cancelSubscription))
	s.mux.HandleFunc("POST /api/billing/change", s.requireUser(s.changePlan))
	s.mux.HandleFunc("POST /api/billing/callback", s.billingCallback)
	s.mux.HandleFunc("POST /api/billing/webhook", s.billingWebhook)
	s.mux.HandleFunc("GET /billing/pay/{token}", s.payPage)

	s.mux.HandleFunc("GET /api/stats/{domain}/summary", s.statsSummary)
	s.mux.HandleFunc("GET /api/stats/{domain}/timeseries", s.statsTimeseries)
	s.mux.HandleFunc("GET /api/stats/{domain}/breakdown", s.statsBreakdown)
	s.mux.HandleFunc("GET /api/stats/{domain}/live", s.statsLive)
	s.mux.HandleFunc("GET /api/stats/{domain}/goals", s.statsGoals)
	s.mux.HandleFunc("GET /api/stats/{domain}/funnels", s.statsFunnels)
	s.mux.HandleFunc("GET /api/stats/{domain}/props", s.statsProps)
	s.mux.HandleFunc("GET /api/stats/{domain}/export", s.export)
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
			s.mailLimit.sweep()
			s.accountLimit.sweep()
			if err := s.auth.SweepTokens(ctx); err != nil {
				log.Printf("token temizliği: %v", err)
			}
			if err := s.team.Sweep(ctx); err != nil {
				log.Printf("davet temizliği: %v", err)
			}
			if s.cfg.Billing != nil {
				if err := s.cfg.Billing.Sweep(ctx); err != nil {
					log.Printf("ödeme temizliği: %v", err)
				}
			}
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
	// Panel kamera, mikrofon ya da konum istemez; bir açık olsa bile istenemesin.
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	// CSRF: başka bir siteden gelen POST/PATCH/DELETE isteklerini reddet
	// (tarayıcının Sec-Fetch-Site ve Origin başlıklarına bakar). Hariç tutulanlar:
	// takip ucu (başka sitelerden çağrılır, Origin'i kendisi denetler), iyzico'nun
	// ödeme dönüşü (iyzico sayfasından gelir, sonucu iyzico'ya sorarız) ve webhook (imzalı).
	if p := r.URL.Path; p != "/api/event" && p != "/api/billing/callback" && p != "/api/billing/webhook" {
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
