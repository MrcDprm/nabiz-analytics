package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
)

// Arama motorları ve paylaşım kartları için: herkese açık sayfaların başlığı,
// açıklaması, dil karşılıkları; robots.txt ve sitemap.xml.

type seoText struct{ Title, Desc string }

// seoPages: dizine girecek sayfalar. [0] Türkçe (/...), [1] İngilizce (/en/...).
// Başlıklar panelin document.title değeriyle aynı (web/src/i18n.ts, content/pages.ts).
var seoPages = map[string][2]seoText{
	"/": {
		{"Nabız: çerezsiz web analitiği, Google Analytics alternatifi",
			"Ziyaretçilerini tanımadan, çerez bandı göstermeden sitenin istatistiklerini gör. 1 KB'lık betik, KVKK dostu, kişisel veri yok. Ücretsiz ya da kendi sunucunda sınırsız."},
		{"Nabız: cookie-free web analytics, a Google Analytics alternative",
			"See your site's stats without identifying visitors or showing a cookie banner. A 1 KB script, GDPR and KVKK friendly, no personal data. Free, or self-host with no limits."},
	},
	"/how-it-works": {
		{"Nabız nasıl çalışır | Nabız",
			"Nabız ziyaretçileri çerez kullanmadan nasıl sayar: neyi toplar, neyi toplamaz, IP adresi ve günlük tuz ne işe yarar. Sitene eklenecek hazır gizlilik metni."},
		{"How Nabız works | Nabız",
			"How Nabız counts visitors without cookies: what it collects and what it doesn't, how the IP address and the daily salt are used, plus a ready-made privacy notice."},
	},
	"/privacy": {
		{"Gizlilik ve KVKK aydınlatma metni | Nabız",
			"Nabız'da hesap açanların kişisel verilerinin KVKK kapsamında nasıl işlendiği. Ölçülen sitelerin ziyaretçileri hakkında kişisel veri saklanmaz."},
		{"Privacy and data protection notice | Nabız",
			"How the personal data of Nabız account holders is processed under KVKK and GDPR. No personal data is stored about the visitors of measured sites."},
	},
	"/terms": {
		{"Kullanım koşulları | Nabız",
			"Nabız çerezsiz web analitiği hizmetinin kısa ve açık kullanım koşulları."},
		{"Terms of use | Nabız",
			"The short and plain terms of use of the Nabız cookie-free web analytics service."},
	},
}

var seoOrder = []string{"/", "/how-it-works", "/privacy", "/terms"}

// pubPath: sayfanın o dildeki adresi ("/privacy" → "/en/privacy").
func pubPath(path string, en bool) string {
	switch {
	case !en:
		return path
	case path == "/":
		return "/en"
	default:
		return "/en" + path
	}
}

// indexable: tanıtım sayfaları sadece barındırılan hizmette (cloud) gösterilir;
// kendi sunucusunda çalışan bir Nabız'ın arama motorlarında çıkması istenmez.
func (s *Server) indexable() bool {
	return s.cfg.Mode == ModeCloud && s.cfg.BaseURL != ""
}

func (s *Server) baseURL() string {
	return strings.TrimSuffix(s.cfg.BaseURL, "/")
}

// seoHead, index.html'deki <title> yerine yazılır. path seoPages'te yoksa
// (panel, giriş, ayarlar...) sayfa dizine girmez.
func (s *Server) seoHead(path string, en bool) string {
	p, ok := seoPages[path]
	if !ok || !s.indexable() {
		return `<title>Nabız</title>` + "\n    " + `<meta name="robots" content="noindex" />`
	}
	lang, locale, altLocale, i := "tr", "tr_TR", "en_GB", 0
	if en {
		lang, locale, altLocale, i = "en", "en_GB", "tr_TR", 1
	}
	t, e := p[i], html.EscapeString
	base := s.baseURL()
	self := base + pubPath(path, en)
	lines := []string{
		"<title>" + e(t.Title) + "</title>",
		`<meta name="description" content="` + e(t.Desc) + `" />`,
		`<link rel="canonical" href="` + e(self) + `" />`,
		`<link rel="alternate" hreflang="tr" href="` + e(base+pubPath(path, false)) + `" />`,
		`<link rel="alternate" hreflang="en" href="` + e(base+pubPath(path, true)) + `" />`,
		`<link rel="alternate" hreflang="x-default" href="` + e(base+pubPath(path, false)) + `" />`,
		`<meta property="og:type" content="website" />`,
		`<meta property="og:site_name" content="Nabız" />`,
		`<meta property="og:title" content="` + e(t.Title) + `" />`,
		`<meta property="og:description" content="` + e(t.Desc) + `" />`,
		`<meta property="og:url" content="` + e(self) + `" />`,
		`<meta property="og:image" content="` + e(base+"/og-"+lang+".png") + `" />`,
		`<meta property="og:image:width" content="1200" />`,
		`<meta property="og:image:height" content="630" />`,
		`<meta property="og:image:alt" content="` + e(p[i].Title) + `" />`,
		`<meta property="og:locale" content="` + locale + `" />`,
		`<meta property="og:locale:alternate" content="` + altLocale + `" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
	}
	if path == "/" {
		// json.Marshal "<" ve ">" işaretlerini kaçışlar; metin script etiketini kapatamaz.
		ld, _ := json.Marshal(map[string]any{
			"@context":            "https://schema.org",
			"@type":               "WebApplication",
			"name":                "Nabız",
			"url":                 self,
			"description":         t.Desc,
			"inLanguage":          lang,
			"applicationCategory": "BusinessApplication",
			"operatingSystem":     "Any",
			"isAccessibleForFree": true,
			"offers":              map[string]string{"@type": "Offer", "price": "0", "priceCurrency": "TRY"},
			"license":             "https://www.gnu.org/licenses/agpl-3.0.html",
			"sameAs":              "https://github.com/MrcDprm/nabiz-analytics",
			"author":              map[string]string{"@type": "Person", "name": "Miraç Deprem", "url": "https://www.miracdeprem.com"},
		})
		lines = append(lines, `<script type="application/ld+json">`+string(ld)+`</script>`)
	}
	return strings.Join(lines, "\n    ")
}

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if !s.indexable() {
		fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
		return
	}
	fmt.Fprintf(w, "User-agent: *\nDisallow: /api/\n\nSitemap: %s/sitemap.xml\n", s.baseURL())
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	if !s.indexable() {
		http.NotFound(w, r)
		return
	}
	base := html.EscapeString(s.baseURL())
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, p := range seoOrder {
		for _, en := range []bool{false, true} {
			fmt.Fprintf(&b, "  <url>\n    <loc>%s%s</loc>\n", base, pubPath(p, en))
			fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"tr\" href=\"%s%s\" />\n", base, pubPath(p, false))
			fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"en\" href=\"%s%s\" />\n", base, pubPath(p, true))
			fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"x-default\" href=\"%s%s\" />\n  </url>\n", base, pubPath(p, false))
		}
	}
	b.WriteString("</urlset>\n")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprint(w, b.String())
}
