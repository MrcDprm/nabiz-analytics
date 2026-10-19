package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSafeCell(t *testing.T) {
	cases := map[string]string{
		"=HYPERLINK(\"http://kotu.com\")": "'=HYPERLINK(\"http://kotu.com\")",
		"+90 555":                         "'+90 555",
		"-1+1":                            "'-1+1",
		"@SUM(A1)":                        "'@SUM(A1)",
		"/projects":                       "/projects",
		"":                                "",
	}
	for in, want := range cases {
		if got := safeCell(in); got != want {
			t.Errorf("%q → %q, beklenen %q", in, got, want)
		}
	}
}

func readZip(t *testing.T, body []byte) map[string][][]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][][]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		if !bytes.HasPrefix(data, []byte(utf8BOM)) {
			t.Errorf("%s: BOM yok", f.Name)
		}
		rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte(utf8BOM)))).ReadAll()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		out[f.Name] = rows
	}
	return out
}

func TestExport(t *testing.T) {
	s, c := statsSetup(t)
	// Zararlı bir kampanya adı ve olay adı: CSV'de formül olarak çalışmamalı.
	post(s, `{"n":"pageview","u":"https://a.com/?utm_campaign=%3DHYPERLINK(1)","s":"a.com","w":1280}`, "https://a.com", chromeUA+" Ek") // ayrı ziyaretçi: kaynak bilgisi ziyaretin ilk sayfasından gelir
	post(s, `{"n":"=cmd|' /C calc'!A0","u":"https://a.com/","s":"a.com","w":1280}`, "https://a.com", chromeUA)

	rec := do(s, "GET", "/api/stats/a.com/export?period=day&lang=tr", "", c, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("durum = %d %s", rec.Code, rec.Body)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="nabiz-a.com-`) {
		t.Fatalf("dosya adı: %q", cd)
	}
	files := readZip(t, rec.Body.Bytes())
	for _, name := range []string{"visitors.csv", "pages.csv", "sources.csv", "countries.csv", "goals.csv", "event_properties.csv", "operating_systems.csv"} {
		if _, ok := files[name]; !ok {
			t.Errorf("%s yok", name)
		}
	}
	if len(files["visitors.csv"]) != 25 || files["visitors.csv"][0][0] != "tarih" {
		t.Fatalf("visitors.csv: %d satır, başlık %v", len(files["visitors.csv"]), files["visitors.csv"][0])
	}
	for file, want := range map[string]string{"utm_campaigns.csv": "'=HYPERLINK(1)", "events.csv": "'=cmd|' /C calc'!A0"} {
		found := false
		for _, r := range files[file] {
			if strings.HasPrefix(r[0], "=") {
				t.Fatalf("%s: formül kaçışlanmadı: %v", file, r)
			}
			found = found || r[0] == want
		}
		if !found {
			t.Errorf("%s: %q yok: %v", file, want, files[file])
		}
	}
}

func TestExportAccess(t *testing.T) {
	s, _ := statsSetup(t)
	// Herkese açık sitede bile girişsiz indirme yok.
	if rec := do(s, "GET", "/api/stats/b.com/export", "", nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("herkese açık, girişsiz = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/stats/a.com/export", "", nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("girişsiz = %d", rec.Code)
	}
	s.auth.CreateUser(context.Background(), "veli@example.com", "baska-sifre-77", true)
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"veli@example.com","password":"baska-sifre-77"}`, nil, nil))
	if rec := do(s, "GET", "/api/stats/a.com/export", "", c, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("başkasının sitesi = %d", rec.Code)
	}
}
