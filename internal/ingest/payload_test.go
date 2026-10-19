package ingest

import (
	"strings"
	"testing"
)

func TestParseValidPageview(t *testing.T) {
	e, err := Parse([]byte(`{"n":"pageview","u":"https://www.miracdeprem.com/projects/hakk%C4%B1nda?utm_source=linkedin&utm_campaign=ekim#x","r":"https://www.google.com/search?q=gizli","s":"miracdeprem.com","w":1280,"extra":"yok sayılır"}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Event{
		Name: "pageview", Site: "miracdeprem.com", Path: "/projects/hakkında",
		Referrer: "google.com", UTMSource: "linkedin", UTMCampaign: "ekim", Device: "laptop",
	}
	if e.Name != want.Name || e.Site != want.Site || e.Path != want.Path || e.Referrer != want.Referrer ||
		e.UTMSource != want.UTMSource || e.UTMCampaign != want.UTMCampaign || e.Device != want.Device {
		t.Fatalf("olay = %+v\nbeklenen %+v", *e, want)
	}
}

func TestParseRejects(t *testing.T) {
	long := strings.Repeat("a", 600)
	cases := map[string]string{
		"bozuk json":           `{"n":`,
		"ad yok":               `{"u":"https://a.com/","s":"a.com"}`,
		"ad çok uzun":          `{"n":"` + strings.Repeat("x", 65) + `","u":"https://a.com/","s":"a.com"}`,
		"adda kontrol kar.":    `{"n":"a\u0000b","u":"https://a.com/","s":"a.com"}`,
		"site yok":             `{"n":"pageview","u":"https://a.com/"}`,
		"site geçersiz":        `{"n":"pageview","u":"https://a.com/","s":"a_b.com"}`,
		"başka sitenin adresi": `{"n":"pageview","u":"https://kotu.com/","s":"a.com"}`,
		"benzer alan adı":      `{"n":"pageview","u":"https://evila.com/","s":"a.com"}`,
		"javascript adresi":    `{"n":"pageview","u":"javascript:alert(1)","s":"a.com"}`,
		"adres çok uzun":       `{"n":"pageview","u":"https://a.com/` + long + `","s":"a.com"}`,
	}
	for name, body := range cases {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: kabul edildi", name)
		}
	}
}

func TestParseSubdomainAllowed(t *testing.T) {
	e, err := Parse([]byte(`{"n":"pageview","u":"https://quiz.miracdeprem.com/","s":"miracdeprem.com"}`))
	if err != nil || e.Path != "/" {
		t.Fatalf("alt alan adı reddedildi: %v", err)
	}
}

func TestReferrer(t *testing.T) {
	cases := []struct{ ref, want string }{
		{"", ""},
		{"https://www.linkedin.com/feed/", "linkedin.com"},
		{"https://a.com/baska-sayfa", ""}, // site içi
		{"https://blog.a.com/", ""},       // aynı sitenin alt alanı
		{"android-app://com.google/", ""}, // http(s) değil
		{"https://t.co/abc", "t.co"},
	}
	for _, c := range cases {
		if got := referrerHost(c.ref, "a.com", "a.com"); got != c.want {
			t.Errorf("%q → %q, beklenen %q", c.ref, got, c.want)
		}
	}
}

func TestRefParamAsSource(t *testing.T) {
	e, err := Parse([]byte(`{"n":"pageview","u":"https://a.com/?ref=producthunt","s":"a.com"}`))
	if err != nil || e.UTMSource != "producthunt" {
		t.Fatalf("ref parametresi kaynak olmadı: %+v %v", e, err)
	}
}

func TestDevice(t *testing.T) {
	cases := map[int]string{0: "", -5: "", 375: "mobile", 800: "tablet", 1366: "laptop", 1920: "desktop", 99999: ""}
	for w, want := range cases {
		if got := device(w); got != want {
			t.Errorf("device(%d) = %q, beklenen %q", w, got, want)
		}
	}
}

func TestProps(t *testing.T) {
	e, err := Parse([]byte(`{"n":"Kayıt","u":"https://a.com/","s":"a.com","p":{"paket":"pro","eposta":"ali@example.com","":"boş","uzun":"` + strings.Repeat("x", 200) + `"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Props) != 1 || e.Props["paket"] != "pro" {
		t.Fatalf("props = %v, sadece paket=pro kalmalı", e.Props)
	}
}

func TestPropsLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"n":"x","u":"https://a.com/","s":"a.com","p":{`)
	for i := range 15 {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"k` + string(rune('a'+i)) + `":"v"`)
	}
	b.WriteString("}}")
	e, err := Parse([]byte(b.String()))
	if err != nil || len(e.Props) != maxProps {
		t.Fatalf("props sayısı = %d (err=%v), %d olmalı", len(e.Props), err, maxProps)
	}
}
