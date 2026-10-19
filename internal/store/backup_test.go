package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0)`)
	backups := filepath.Join(dir, "backup")
	ctx := context.Background()
	day := time.Date(2026, 10, 19, 3, 0, 0, 0, time.UTC)

	path, err := s.Backup(ctx, backups, day, 3)
	if err != nil || filepath.Base(path) != "nabiz-2026-10-19.db" {
		t.Fatalf("yedek = %q %v", path, err)
	}
	// Aynı gün ikinci kez: dokunulmaz.
	if path, err := s.Backup(ctx, backups, day, 3); err != nil || path != "" {
		t.Fatalf("aynı gün = %q %v", path, err)
	}
	// Yedek açılabilir ve veriyi içeriyor.
	b, err := Open(filepath.Join(backups, "nabiz-2026-10-19.db"))
	if err != nil {
		t.Fatal(err)
	}
	var n int
	b.R.QueryRow(`SELECT count(*) FROM sites`).Scan(&n)
	b.Close()
	if n != 1 {
		t.Fatalf("yedekte %d site", n)
	}

	// 5 gün daha: en yeni 3 kalır.
	for i := 1; i <= 5; i++ {
		if _, err := s.Backup(ctx, backups, day.AddDate(0, 0, i), 3); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(backups)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 3 || names[0] != "nabiz-2026-10-22.db" || names[2] != "nabiz-2026-10-24.db" {
		t.Fatalf("kalan yedekler = %v", names)
	}
}
