// Package geo, IP adresinden ülkeyi yerel bir veritabanıyla bulur. IP hiçbir
// üçüncü tarafa gönderilmez ve hiçbir yere yazılmaz.
//
// Veritabanı: DB-IP "IP to Country Lite" (CC BY 4.0, https://db-ip.com).
// Lisans gereği arayüzde ve README'de DB-IP'ye atıf yapılır.
package geo

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// Locator, açık veritabanını tutar. Yeniden yükleme (Reload) çalışan isteklere
// dokunmadan yapılır: yeni okuyucu hazır olunca işaretçi tek hamlede değişir.
type Locator struct {
	path string
	db   atomic.Pointer[maxminddb.Reader]
}

// Open veritabanını açar. Dosya yoksa hata vermez: ülke boş kalır, sayım sürer.
func Open(path string) (*Locator, error) {
	l := &Locator{path: path}
	if err := l.Reload(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return l, nil
}

// Reload dosyayı yeniden açar (ör. "nabiz geoip update" sonrası SIGHUP ile).
func (l *Locator) Reload() error {
	r, err := maxminddb.Open(l.path)
	if err != nil {
		return err
	}
	if old := l.db.Swap(r); old != nil {
		// Eski okuyucuyu hemen kapatmıyoruz: o anda onu kullanan bir istek olabilir.
		// Ayda bir yapılan bir iş için kısa bir bekleme yeterli.
		time.AfterFunc(time.Minute, func() { old.Close() })
	}
	return nil
}

func (l *Locator) Loaded() bool { return l.db.Load() != nil }

type record struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// Country, iki harfli ülke kodunu döndürür (ör. "TR"); bulamazsa boş döner.
func (l *Locator) Country(ip string) string {
	r := l.db.Load()
	if r == nil {
		return ""
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.IsPrivate() || addr.IsLoopback() || addr.IsUnspecified() {
		return ""
	}
	var rec record
	if err := r.Lookup(addr.Unmap()).Decode(&rec); err != nil {
		return ""
	}
	if len(rec.Country.ISOCode) != 2 {
		return ""
	}
	return rec.Country.ISOCode
}

// DefaultBaseURL, DB-IP'nin ücretsiz indirme adresi. Dosya adı ay ile değişir.
const DefaultBaseURL = "https://download.db-ip.com/free"

const maxDownload = 64 << 20 // açılmış dosya en fazla 64 MB (bugün ~8 MB)

// Update bu ayın dosyasını indirir; ay yeni başladıysa ve henüz yayımlanmadıysa
// geçen ayınkini dener. Dosya önce geçici ada yazılır, açılıp doğrulanır,
// sonra tek hamlede asıl adın yerine geçer: yarım indirme çalışan dosyayı bozmaz.
func Update(ctx context.Context, client *http.Client, baseURL, path string, now time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var lastErr error
	for _, month := range []time.Time{now, now.AddDate(0, -1, 0)} {
		url := fmt.Sprintf("%s/dbip-country-lite-%s.mmdb.gz", baseURL, month.UTC().Format("2006-01"))
		err := download(ctx, client, url, path)
		if err == nil {
			log.Printf("geoip: %s indirildi", url)
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func download(ctx context.Context, client *http.Client, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: gzip değil: %w", url, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".geoip-*.mmdb")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // başarılı rename'den sonra zaten yok
	n, err := io.Copy(tmp, io.LimitReader(gz, maxDownload+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > maxDownload {
		return fmt.Errorf("%s: dosya beklenenden büyük", url)
	}

	// İndirilen dosya gerçekten bir ülke veritabanı mı?
	r, err := maxminddb.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("%s: geçersiz veritabanı: %w", url, err)
	}
	r.Close()
	return os.Rename(tmp.Name(), path)
}
