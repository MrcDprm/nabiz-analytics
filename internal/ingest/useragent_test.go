package ingest

import "testing"

const (
	uaChromeWin  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
	uaEdgeWin    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0"
	uaSafariIOS  = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1"
	uaFirefoxLin = "Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0"
	uaSamsung    = "Mozilla/5.0 (Linux; Android 14; SM-S921B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/28.0 Chrome/130.0.0.0 Mobile Safari/537.36"
	uaSafariMac  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Safari/605.1.15"
	uaOperaWin   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 OPR/124.0.0.0"
	uaChromeOS   = "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
)

func TestParseUA(t *testing.T) {
	cases := []struct{ ua, browser, os string }{
		{uaChromeWin, "Chrome", "Windows"},
		{uaEdgeWin, "Edge", "Windows"},
		{uaSafariIOS, "Safari", "iOS"},
		{uaFirefoxLin, "Firefox", "Linux"},
		{uaSamsung, "Samsung Internet", "Android"},
		{uaSafariMac, "Safari", "macOS"},
		{uaOperaWin, "Opera", "Windows"},
		{uaChromeOS, "Chrome", "ChromeOS"},
	}
	for _, c := range cases {
		b, o := ParseUA(c.ua)
		if b != c.browser || o != c.os {
			t.Errorf("%s\n→ %s / %s, beklenen %s / %s", c.ua, b, o, c.browser, c.os)
		}
	}
}

func TestIsBot(t *testing.T) {
	bots := []string{
		"",
		"curl/8.5.0",
		"python-requests/2.32",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Chrome-Lighthouse",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"WhatsApp/2.23.20.0",
	}
	for _, ua := range bots {
		if !IsBot(ua) {
			t.Errorf("bot sayılmadı: %q", ua)
		}
	}
	for _, ua := range []string{uaChromeWin, uaSafariIOS, uaFirefoxLin, uaSamsung} {
		if IsBot(ua) {
			t.Errorf("gerçek tarayıcı bot sayıldı: %q", ua)
		}
	}
}
