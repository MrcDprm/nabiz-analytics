package server

import (
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/MrcDprm/nabiz-analytics/internal/ingest"
)

// event, POST /api/event: takip betiğinin gönderdiği tek olayı alır.
//
// Betik "text/plain" gönderir; bu yüzden tarayıcı CORS ön isteği (OPTIONS)
// atmaz ve bizim de ona cevap veren bir uç açmamıza gerek kalmaz.
//
// Cevaplar: 202 kabul (bot ve kayıtlı olmayan site de sessizce 202 alır, ne
// olduğunu öğrenmesinler), 400 bozuk olay, 403 Origin uyuşmuyor, 413 gövde
// çok büyük, 429 çok sık istek.
func (s *Server) event(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.eventLimit.allow(ip) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, ingest.MaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	}
	e, err := ingest.Parse(body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Tarayıcı Origin başlığını kendisi koyar, sayfadaki kod değiştiremez.
	// Bu yüzden başka bir sitenin sayfası bizim adımıza olay gönderemez.
	origin := r.Header.Get("Origin")
	if !originMatches(origin, e.Site) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	// CORS: "*" değil, sadece isteği atan sitenin kendi adresi.
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")

	ua := r.UserAgent()
	if !ingest.IsBot(ua) {
		err := s.recorder.Record(r.Context(), e, ip, ua)
		if err != nil && !errors.Is(err, ingest.ErrUnknownSite) {
			log.Printf("event: %v", err) // IP ve UA log'a yazılmaz
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

// originMatches: Origin, olaydaki sitenin kendisi ya da alt alan adı mı?
func originMatches(origin, site string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == site || strings.HasSuffix(h, "."+site)
}

// clientIP: Caddy arkasındaysak (-trust-proxy) X-Forwarded-For'un son değeri,
// yani Caddy'nin kendi eklediği adres; değilse doğrudan bağlantının adresi.
// İstemcinin kendi yazdığı sahte değerler listenin başında kalır, onlara bakmayız.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
