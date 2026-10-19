// Package store, Nabız'ın SQLite veritabanını açar ve şemayı günceller.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"runtime"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store iki bağlantı havuzu tutar: SQLite aynı anda tek yazara izin verir,
// bu yüzden yazmalar tek bağlantıdan geçer ("database is locked" olmaz);
// panel sorguları ise WAL sayesinde yazmayı beklemeden ayrı havuzdan okur.
type Store struct {
	W *sql.DB // yazma: tek bağlantı
	R *sql.DB // okuma: salt okunur, birden çok bağlantı
}

func dsn(path string, readOnly bool) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	}
	return "file:" + path + "?" + q.Encode()
}

// Open veritabanını açar (yoksa oluşturur) ve bekleyen migration'ları uygular.
func Open(path string) (*Store, error) {
	w, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	if err := migrate(w); err != nil {
		w.Close()
		return nil, fmt.Errorf("migration: %w", err)
	}
	r, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(max(4, runtime.NumCPU()))
	return &Store{W: w, R: r}, nil
}

func (s *Store) Close() error {
	return errors.Join(s.R.Close(), s.W.Close())
}

// Ping iki havuzun da çalıştığını kontrol eder (/healthz kullanır).
func (s *Store) Ping(ctx context.Context) error {
	if err := s.W.PingContext(ctx); err != nil {
		return err
	}
	return s.R.PingContext(ctx)
}

// migrate, migrations/NNNN_ad.sql dosyalarını sırayla uygular. Hangi sürümde
// olduğumuz PRAGMA user_version'da durur; her dosya kendi işleminde (transaction)
// çalışır, yarıda kalan bir migration veritabanını yarım bırakmaz.
func migrate(db *sql.DB) error {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	var current int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return err
	}
	for _, name := range names {
		base := strings.TrimPrefix(name, "migrations/")
		n, err := strconv.Atoi(strings.SplitN(base, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("%s: dosya adı NNNN_ad.sql olmalı", base)
		}
		if n <= current {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", base, err)
		}
		// PRAGMA parametre almaz; n bir tamsayı olduğu için birleştirmek güvenli.
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, n)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		current = n
	}
	return nil
}
