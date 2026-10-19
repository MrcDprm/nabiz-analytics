package mail

import (
	"mime"
	"strings"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	msg, err := Build("Nabız <bildirim@example.com>", "ali@example.com", "Şifreni sıfırla", "Merhaba,\nbağlantı: https://x.com/reset?token=abc", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	s := string(msg)
	head, body, _ := strings.Cut(s, "\r\n\r\n")
	if !strings.Contains(head, "To: ali@example.com\r\n") || !strings.Contains(head, "charset=utf-8") || !strings.Contains(head, "@example.com>") {
		t.Fatalf("başlıklar:\n%s", head)
	}
	for _, line := range strings.Split(head, "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			dec, _ := new(mime.WordDecoder).DecodeHeader(strings.TrimPrefix(line, "Subject: "))
			if dec != "Şifreni sıfırla" {
				t.Fatalf("konu = %q", dec)
			}
		}
	}
	if !strings.Contains(body, "ba=C4=9Flant=C4=B1") {
		t.Fatalf("gövde quoted-printable değil:\n%s", body)
	}
}

func TestBuildRejectsHeaderInjection(t *testing.T) {
	for _, to := range []string{"ali@example.com\r\nBcc: herkes@example.com", "a@b.com\nX: y"} {
		if _, err := Build("n@example.com", to, "Konu", "gövde", time.Now()); err == nil {
			t.Errorf("%q kabul edildi", to)
		}
	}
	if _, err := Build("n@example.com", "a@b.com", "Konu\r\nBcc: x@y.com", "gövde", time.Now()); err == nil {
		t.Error("konuda satır sonu kabul edildi")
	}
}

func TestRender(t *testing.T) {
	sub, body := Render("tr", Reset, "https://nabiz.example.com/reset?token=abc")
	if sub != "Nabız şifreni sıfırla" || !strings.Contains(body, "https://nabiz.example.com/reset?token=abc") || !strings.Contains(body, "1 saat") {
		t.Fatalf("%q\n%s", sub, body)
	}
	if sub, _ := Render("de", Verify, "x"); sub != "Verify your Nabız account" {
		t.Fatalf("bilinmeyen dil: %q", sub)
	}
	for lang, kinds := range templates {
		for _, k := range []Kind{Verify, Reset, EmailChange, AlreadyTaken} {
			if kinds[k].subject == "" || !strings.Contains(kinds[k].body, "%[1]s") {
				t.Errorf("%s/%s şablonu eksik", lang, k)
			}
		}
	}
}
