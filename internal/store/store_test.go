package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var version int
	if err := s.R.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("user_version = %d, 1 olmalı", version)
	}
	for _, table := range []string{"users", "sites", "memberships", "sessions", "events", "event_props", "subscriptions"} {
		var n int
		err := s.R.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n)
		if err != nil || n != 1 {
			t.Errorf("%s tablosu yok (err=%v)", table, err)
		}
	}
}

func TestOpenTwiceKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nabiz.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('a', 'example.com', 0)`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	// İkinci açılışta migration tekrar çalışmamalı, veri durmalı.
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	if err := s.R.QueryRow(`SELECT count(*) FROM sites`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("site sayısı = %d (err=%v), 1 olmalı", n, err)
	}
}

func TestReadPoolIsReadOnly(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.R.ExecContext(context.Background(), `INSERT INTO sites (id, domain, created_at) VALUES ('a', 'x.com', 0)`); err == nil {
		t.Fatal("okuma havuzu yazabildi")
	}
}

func TestForeignKeysOn(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.W.Exec(`INSERT INTO memberships (site_id, user_id, role) VALUES ('yok', 99, 'owner')`)
	if err == nil {
		t.Fatal("olmayan siteye üyelik eklenebildi; foreign_keys kapalı")
	}
}
