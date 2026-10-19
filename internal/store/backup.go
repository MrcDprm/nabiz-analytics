package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupName, günün yedek dosyasının adı: nabiz-2026-10-19.db.
func BackupName(day time.Time) string {
	return "nabiz-" + day.UTC().Format("2006-01-02") + ".db"
}

// Backup veritabanının tutarlı bir kopyasını dir klasörüne yazar ve en yeni keep
// yedek dışındakileri siler. VACUUM INTO, sunucu çalışırken bile tutarlı bir
// anlık görüntü alır (yazmalar bu sırada kısa süre bekler). O günün yedeği zaten
// varsa hiçbir şey yapmaz.
func (s *Store) Backup(ctx context.Context, dir string, day time.Time, keep int) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, BackupName(day))
	if _, err := os.Stat(path); err == nil {
		return "", nil
	}
	// Önce geçici ada yaz, bitince taşı: yarım kalan bir yedek "tamam" görünmesin.
	tmp := path + ".tmp"
	os.Remove(tmp)
	if _, err := s.W.ExecContext(ctx, `VACUUM INTO ?`, tmp); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("yedek: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, prune(dir, keep)
}

// prune, en yeni keep yedek dışındakileri siler (adlar tarih içerdiği için sıralama tarih sırasıdır).
func prune(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if n := e.Name(); strings.HasPrefix(n, "nabiz-") && strings.HasSuffix(n, ".db") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for len(names) > keep {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
