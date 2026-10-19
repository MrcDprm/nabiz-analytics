package geo

import (
	"bytes"
	"compress/gzip"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

// testDB, iki ağ içeren küçük bir ülke veritabanı üretir (gerçek dosya gerekmesin).
func testDB(t *testing.T) []byte {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "DBIP-Country-Lite", RecordSize: 24})
	if err != nil {
		t.Fatal(err)
	}
	for cidr, code := range map[string]string{"81.214.0.0/16": "TR", "2a02:ff0::/32": "DE"} {
		_, network, _ := net.ParseCIDR(cidr)
		rec := mmdbtype.Map{"country": mmdbtype.Map{"iso_code": mmdbtype.String(code)}}
		if err := tree.Insert(network, rec); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if _, err := tree.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCountry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.mmdb")
	os.WriteFile(path, testDB(t), 0o600)
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"81.214.10.20":        "TR",
		"::ffff:81.214.10.20": "TR", // IPv6 içinde IPv4
		"2a02:ff0::1":         "DE",
		"8.8.8.8":             "", // veritabanında yok
		"192.168.1.5":         "", // özel ağ
		"127.0.0.1":           "",
		"bozuk":               "",
	}
	for ip, want := range cases {
		if got := l.Country(ip); got != want {
			t.Errorf("Country(%q) = %q, beklenen %q", ip, got, want)
		}
	}
}

func TestMissingFileIsNotFatal(t *testing.T) {
	l, err := Open(filepath.Join(t.TempDir(), "yok.mmdb"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Loaded() || l.Country("81.214.10.20") != "" {
		t.Fatal("dosya yokken ülke döndü")
	}
}

func gzipped(t *testing.T, b []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(b)
	zw.Close()
	return buf.Bytes()
}

func TestUpdateFallsBackToLastMonth(t *testing.T) {
	db := gzipped(t, testDB(t))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "dbip-country-lite-2026-09.mmdb.gz") {
			w.Write(db)
			return
		}
		http.NotFound(w, r) // ekim dosyası henüz yayımlanmamış gibi
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "geoip.mmdb")
	now := time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)
	if err := Update(context.Background(), srv.Client(), srv.URL, path, now); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path)
	if err != nil || l.Country("81.214.1.1") != "TR" {
		t.Fatalf("indirilen dosya çalışmıyor: %v", err)
	}
}

func TestUpdateKeepsOldFileOnBadDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(gzipped(t, []byte("veritabanı değil")))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "geoip.mmdb")
	good := testDB(t)
	os.WriteFile(path, good, 0o600)
	if err := Update(context.Background(), srv.Client(), srv.URL, path, time.Now()); err == nil {
		t.Fatal("bozuk dosya kabul edildi")
	}
	if b, _ := os.ReadFile(path); !bytes.Equal(b, good) {
		t.Fatal("bozuk indirme eski dosyanın üstüne yazıldı")
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".geoip-*")); len(left) != 0 {
		t.Fatalf("geçici dosya kaldı: %v", left)
	}
}

func TestReloadSwapsDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.mmdb")
	l, _ := Open(path)
	if l.Loaded() {
		t.Fatal("dosya yokken yüklendi")
	}
	os.WriteFile(path, testDB(t), 0o600)
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if l.Country("81.214.1.1") != "TR" {
		t.Fatal("yeniden yükleme sonrası ülke bulunamadı")
	}
}
