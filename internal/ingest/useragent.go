package ingest

import "strings"

// Bot imzaları: User-Agent'ta bunlardan biri geçiyorsa olay sayılmaz.
// Liste küçük harfle; sıralama önemli değil.
var botMarkers = []string{
	"bot", "crawl", "spider", "slurp", "headless", "lighthouse", "pagespeed",
	"preview", "monitor", "uptime", "pingdom", "phantomjs", "selenium",
	"puppeteer", "playwright", "curl/", "wget/", "python", "go-http-client",
	"java/", "okhttp", "axios", "node-fetch", "httpclient", "facebookexternalhit",
	"whatsapp", "telegram", "discord", "slack", "embedly", "quora link",
	"bitly", "feedfetcher", "scrapy", "ahrefs", "semrush", "mj12", "yandex.com/bots",
}

// IsBot, User-Agent'a bakıp isteğin bir insandan gelmediğini tahmin eder.
// Gerçek tarayıcıların UA'sı her zaman "Mozilla/" ile başlar ve uzundur.
func IsBot(ua string) bool {
	if len(ua) < 20 || !strings.HasPrefix(ua, "Mozilla/") {
		return true
	}
	l := strings.ToLower(ua)
	for _, m := range botMarkers {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

// ParseUA, User-Agent'tan sadece tarayıcı ve işletim sistemi adını çıkarır;
// sürüm kaydetmiyoruz (hem gereksiz hem parmak izine katkı). Sıra önemli:
// Edge ve Opera kendini "Chrome" diye de tanıttığı için önce onlara bakılır.
func ParseUA(ua string) (browser, os string) {
	switch {
	case strings.Contains(ua, "Edg/"), strings.Contains(ua, "EdgA/"), strings.Contains(ua, "EdgiOS/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "YaBrowser/"):
		browser = "Yandex"
	case strings.Contains(ua, "SamsungBrowser/"):
		browser = "Samsung Internet"
	case strings.Contains(ua, "Firefox/"), strings.Contains(ua, "FxiOS/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	default:
		browser = "Other"
	}

	switch {
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iPod"):
		os = "iOS"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "CrOS"):
		os = "ChromeOS"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		os = "macOS"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	default:
		os = "Other"
	}
	return browser, os
}
