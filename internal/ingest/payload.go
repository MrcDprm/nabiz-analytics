// Package ingest, takip betiğinden gelen olayları alır, doğrular ve kaydeder.
package ingest

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MrcDprm/nabiz-analytics/internal/site"
)

const (
	MaxBody      = 2 << 10 // gövde en fazla 2 KB
	maxName      = 64
	maxURL       = 512
	maxProps     = 10
	maxPropKey   = 32
	maxPropValue = 128
	maxUTM       = 128
	Pageview     = "pageview"
)

// payload, betiğin gönderdiği JSON. Alan adları kısa tutuldu ki betik küçük kalsın.
// Sadece bu alanlar okunur; gövdedeki başka her şey yok sayılır.
type payload struct {
	Name     string            `json:"n"`
	URL      string            `json:"u"`
	Referrer string            `json:"r"`
	Site     string            `json:"s"`
	Width    int               `json:"w"`
	Props    map[string]string `json:"p"`
}

// Event, doğrulanmış ve sadeleştirilmiş olay: veritabanına sadece bunlar gider.
type Event struct {
	Name        string
	Site        string // betikteki data-site, küçük harf
	Path        string // sorgu ve # olmadan
	Referrer    string // sadece alan adı, "www." olmadan; site içi geçişte boş
	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	Device      string
	Props       map[string]string
}

var (
	errInvalid = errors.New("geçersiz olay")
	// Props'a yanlışlıkla e-posta gönderilmesin: kişisel veri saklamıyoruz.
	emailLike = regexp.MustCompile(`[^@\s]+@[^@\s]+\.[^@\s]+`)
)

// Parse gövdeyi çözer ve doğrular. Hata ayrıntısı istemciye dönmez; betik
// cevaba zaten bakmaz, saldırgana da ipucu vermeyiz.
func Parse(body []byte) (*Event, error) {
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, errInvalid
	}

	name := strings.TrimSpace(p.Name)
	if !validText(name, maxName) {
		return nil, errInvalid
	}

	siteName := strings.ToLower(strings.TrimSpace(p.Site))
	if !site.ValidDomain(siteName) {
		return nil, errInvalid
	}

	if len(p.URL) > maxURL {
		return nil, errInvalid
	}
	u, err := url.Parse(p.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errInvalid
	}
	// Sayfanın alan adı, betikte yazan siteyle ya da onun alt alan adıyla aynı olmalı.
	host := strings.ToLower(u.Hostname())
	if host != siteName && !strings.HasSuffix(host, "."+siteName) {
		return nil, errInvalid
	}

	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if decoded, err := url.PathUnescape(path); err == nil && utf8.ValidString(decoded) {
		path = decoded // panelde "/hakk%C4%B1nda" yerine "/hakkında" görünsün
	}

	q := u.Query()
	e := &Event{
		Name:        name,
		Site:        siteName,
		Path:        path,
		Referrer:    referrerHost(p.Referrer, host, siteName),
		UTMSource:   clip(q.Get("utm_source"), maxUTM),
		UTMMedium:   clip(q.Get("utm_medium"), maxUTM),
		UTMCampaign: clip(q.Get("utm_campaign"), maxUTM),
		Device:      device(p.Width),
		Props:       cleanProps(p.Props),
	}
	// "?ref=producthunt" gibi kaynak parametreleri UTM olmasa da kaynak sayılır.
	if e.UTMSource == "" {
		e.UTMSource = clip(firstNonEmpty(q.Get("ref"), q.Get("source")), maxUTM)
	}
	return e, nil
}

// referrerHost, gelinen sayfanın sadece alan adını döndürür (yol ve sorgu
// kişisel bilgi taşıyabilir). Site içinden gelinmişse boş döner.
func referrerHost(raw, pageHost, site string) string {
	if raw == "" || len(raw) > maxURL {
		return ""
	}
	r, err := url.Parse(raw)
	if err != nil || (r.Scheme != "http" && r.Scheme != "https") {
		return ""
	}
	h := strings.TrimPrefix(strings.ToLower(r.Hostname()), "www.")
	if h == "" || h == strings.TrimPrefix(pageHost, "www.") || h == site || strings.HasSuffix(h, "."+site) {
		return ""
	}
	return h
}

// device, ekran genişliğinden cihaz türünü bulur (Plausible ile aynı sınırlar).
func device(w int) string {
	switch {
	case w <= 0 || w > 20000:
		return ""
	case w < 576:
		return "mobile"
	case w < 992:
		return "tablet"
	case w < 1440:
		return "laptop"
	default:
		return "desktop"
	}
}

func cleanProps(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, min(len(in), maxProps))
	for k, v := range in {
		if len(out) == maxProps {
			break
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !validText(k, maxPropKey) || !validText(v, maxPropValue) || emailLike.MatchString(v) {
			continue // tek kötü özellik yüzünden bütün olayı atmıyoruz
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// validText: boş değil, sınırı aşmıyor, geçerli UTF-8 ve kontrol karakteri yok.
func validText(s string, limit int) bool {
	if s == "" || utf8.RuneCountInString(s) > limit || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) {
		return ""
	}
	if utf8.RuneCountInString(s) > n {
		s = string([]rune(s)[:n])
	}
	return s
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
