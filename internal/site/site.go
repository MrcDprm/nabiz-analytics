// Package site, izlenen siteleri ve kimlerin hangi siteye erişebildiğini yönetir.
package site

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

var (
	ErrInvalidDomain = errors.New("geçerli bir alan adı yazın (ör. example.com)")
	ErrDomainTaken   = errors.New("bu site zaten ekli")
)

type Site struct {
	ID     string
	Domain string
	Public bool
}

// NormalizeDomain kullanıcının yazdığını sadeleştirir: "https://www.Example.com/"
// → "example.com". Betikteki data-site ile aynı biçim olmalı.
func NormalizeDomain(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.TrimPrefix(s, "www.")
	s, _, _ = strings.Cut(s, "/")
	if !ValidDomain(s) {
		return "", ErrInvalidDomain
	}
	return s, nil
}

// ValidDomain: harf, rakam ve tire; en az bir nokta; her parça 1-63 karakter.
func ValidDomain(s string) bool {
	if s == "" || len(s) > 253 || !strings.Contains(s, ".") {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

// newID rastgele bir UUID (sürüm 4) üretir: adreslerde görünen kimlikler tahmin edilemesin.
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40 // sürüm 4
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 varyantı
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// Create siteyi ekler ve ekleyen kullanıcıyı sahibi yapar (tek işlemde).
func Create(ctx context.Context, st *store.Store, domain string, ownerID int64) (*Site, error) {
	domain, err := NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	tx, err := st.W.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO sites (id, domain, created_at) VALUES (?, ?, ?)`,
		id, domain, time.Now().Unix()); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrDomainTaken
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memberships (site_id, user_id, role) VALUES (?, ?, 'owner')`,
		id, ownerID); err != nil {
		return nil, err
	}
	return &Site{ID: id, Domain: domain}, tx.Commit()
}
