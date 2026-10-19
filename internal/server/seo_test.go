package server

import (
	"strings"
	"testing"
	"testing/fstest"
)

const testIndex = `<!doctype html>
<html lang="tr">
  <head>
    <title>Nabız</title>
  </head>
  <body><div id="app"></div></body>
</html>`

func TestSEOHead(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.Web = fstest.MapFS{"index.html": {Data: []byte(testIndex)}}
	s.cfg.Mode, s.cfg.BaseURL = ModeCloud, "https://nabiz.example.com/"

	body := do(s, "GET", "/", "", nil, nil).Body.String()
	for _, want := range []string{
		`<html lang="tr">`,
		"<title>Nabız: çerezsiz web analitiği, Google Analytics alternatifi</title>",
		`<link rel="canonical" href="https://nabiz.example.com/" />`,
		`hreflang="en" href="https://nabiz.example.com/en"`,
		`<meta property="og:image" content="https://nabiz.example.com/og-tr.png" />`,
		`"@type":"WebApplication"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/ içinde yok: %s", want)
		}
	}
	if strings.Contains(body, "noindex") {
		t.Error("tanıtım sayfası noindex olmamalı")
	}

	body = do(s, "GET", "/en/privacy", "", nil, nil).Body.String()
	for _, want := range []string{
		`<html lang="en">`,
		"<title>Privacy and data protection notice | Nabız</title>",
		`<link rel="canonical" href="https://nabiz.example.com/en/privacy" />`,
		`hreflang="tr" href="https://nabiz.example.com/privacy"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/en/privacy içinde yok: %s", want)
		}
	}
	if strings.Contains(body, "ld+json") {
		t.Error("JSON-LD sadece tanıtım sayfasında")
	}

	// Panel sayfaları dizine girmez.
	for _, p := range []string{"/login", "/miracdeprem.com", "/settings/miracdeprem.com"} {
		if b := do(s, "GET", p, "", nil, nil).Body.String(); !strings.Contains(b, `content="noindex"`) || strings.Contains(b, "canonical") {
			t.Errorf("%s noindex değil", p)
		}
	}
}

func TestSEOSelfhost(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.Web = fstest.MapFS{"index.html": {Data: []byte(testIndex)}}
	s.cfg.Mode, s.cfg.BaseURL = ModeSelfhost, "https://nabiz.example.com"

	if b := do(s, "GET", "/", "", nil, nil).Body.String(); !strings.Contains(b, `content="noindex"`) {
		t.Error("kendi sunucusunda tanıtım sayfası dizine girmemeli")
	}
	if b := do(s, "GET", "/robots.txt", "", nil, nil).Body.String(); b != "User-agent: *\nDisallow: /\n" {
		t.Errorf("robots = %q", b)
	}
	if rec := do(s, "GET", "/sitemap.xml", "", nil, nil); rec.Code != 404 {
		t.Errorf("sitemap = %d", rec.Code)
	}
}

func TestRobotsAndSitemap(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.Web = fstest.MapFS{"index.html": {Data: []byte(testIndex)}}
	s.cfg.Mode, s.cfg.BaseURL = ModeCloud, "https://nabiz.example.com"

	rec := do(s, "GET", "/robots.txt", "", nil, nil)
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") || !strings.Contains(rec.Body.String(), "Sitemap: https://nabiz.example.com/sitemap.xml") {
		t.Fatalf("robots: %s %q", rec.Header().Get("Content-Type"), rec.Body.String())
	}
	rec = do(s, "GET", "/sitemap.xml", "", nil, nil)
	body := rec.Body.String()
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/xml") || strings.Count(body, "<loc>") != 8 {
		t.Fatalf("sitemap: %s\n%s", rec.Header().Get("Content-Type"), body)
	}
	for _, want := range []string{"<loc>https://nabiz.example.com/en/how-it-works</loc>", "<loc>https://nabiz.example.com/terms</loc>"} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap içinde yok: %s", want)
		}
	}
}
