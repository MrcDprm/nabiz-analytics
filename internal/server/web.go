package server

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/site"
)

// pageCSP: panel sadece bu sunucunun kendi betiklerini, stillerini ve yazı
// tiplerini yükler ve sadece bu sunucuyla konuşur. Trusted Types, metinden HTML
// üretilmesini tarayıcı düzeyinde yasaklar (sayfa kodu zaten hep textContent kullanır).
const pageCSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; " +
	"connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; " +
	"require-trusted-types-for 'script'; trusted-types 'none'"

var contentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".svg":   "image/svg+xml",
	".woff2": "font/woff2",
	".png":   "image/png",
	".txt":   "text/plain; charset=utf-8",
}

// webRoutes paneli sunar. Panel tek sayfalık bir uygulama: "/", "/login" ve
// "/{alan-adı}" aynı index.html'i açar, gerisini tarayıcıdaki kod çözer.
func (s *Server) webRoutes() {
	index := s.serveFile("index.html", "no-cache")
	s.mux.HandleFunc("GET /{$}", index)
	for _, p := range []string{"/login", "/signup", "/verify", "/forgot", "/reset", "/account", "/invite", "/billing"} {
		s.mux.HandleFunc("GET "+p, index)
	}
	sitePage := func(w http.ResponseWriter, r *http.Request) {
		// Alan adına benzemeyen adresler (ör. /wp-admin) için 404.
		if !site.ValidDomain(strings.ToLower(r.PathValue("domain"))) {
			http.NotFound(w, r)
			return
		}
		index(w, r)
	}
	s.mux.HandleFunc("GET /{domain}", sitePage)
	s.mux.HandleFunc("GET /settings/{domain}", sitePage) // /{domain}/settings, /assets/{name} ile çakışırdı
	s.mux.HandleFunc("GET /favicon.svg", s.serveFile("favicon.svg", "public, max-age=86400"))
	s.mux.HandleFunc("GET /select.svg", s.serveFile("select.svg", "public, max-age=86400"))
	s.mux.HandleFunc("GET /assets/{name}", func(w http.ResponseWriter, r *http.Request) {
		// Dosya adlarında içeriğin özeti var; tarayıcı süresiz saklayabilir.
		s.serveFile("assets/"+r.PathValue("name"), "public, max-age=31536000, immutable")(w, r)
	})
}

func (s *Server) serveFile(name, cacheControl string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Web == nil || strings.HasPrefix(path.Base(name), ".") || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(s.cfg.Web, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ct, ok := contentTypes[path.Ext(name)]
		if !ok {
			ct = "application/octet-stream"
		}
		h := w.Header()
		h.Set("Content-Type", ct)
		h.Set("Cache-Control", cacheControl)
		h.Set("Content-Security-Policy", pageCSP)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}
}
