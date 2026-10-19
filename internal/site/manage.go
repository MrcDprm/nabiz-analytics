package site

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Roller güç sırasına göre: sahip > yönetici > izleyici.
var roleRank = map[string]int{"viewer": 1, "admin": 2, "owner": 3}

// AtLeast: role, en az want kadar yetkili mi?
func AtLeast(role, want string) bool { return roleRank[role] >= roleRank[want] && roleRank[role] > 0 }

// ByID siteyi kimliğiyle bulur.
func ByID(ctx context.Context, db *sql.DB, id string) (*Site, error) {
	s := &Site{}
	err := db.QueryRowContext(ctx, `SELECT id, domain, public FROM sites WHERE id = ?`, id).Scan(&s.ID, &s.Domain, &s.Public)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func SetPublic(ctx context.Context, db *sql.DB, id string, public bool) error {
	_, err := db.ExecContext(ctx, `UPDATE sites SET public = ? WHERE id = ?`, public, id)
	return err
}

// Delete siteyi siler; ziyaretler, olaylar, hedefler ve üyelikler yabancı
// anahtarlardaki ON DELETE CASCADE ile birlikte gider.
func Delete(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	return err
}

// Goal, bir dönüşüm hedefi: belirli bir sayfanın görüntülenmesi ya da bir özel olay.
// Sayfa hedefi "*" ile bitebilir: "/blog/*" bütün blog yazılarını kapsar.
type Goal struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"` // page | event
	Value string `json:"value"`
}

const MaxGoals = 20

var (
	ErrInvalidGoal = errors.New("hedef geçersiz: sayfa / ile başlamalı (en fazla 512 karakter), olay adı en fazla 64 karakter olabilir")
	ErrGoalExists  = errors.New("bu hedef zaten var")
	ErrTooMany     = errors.New("bir sitede en fazla 20 hedef olabilir")
)

func validGoal(kind, value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	switch kind {
	case "page":
		return strings.HasPrefix(value, "/") && utf8.RuneCountInString(value) <= 512 &&
			strings.Count(value, "*") <= 1 && (!strings.Contains(value, "*") || strings.HasSuffix(value, "*"))
	case "event":
		return utf8.RuneCountInString(value) <= 64
	}
	return false
}

func Goals(ctx context.Context, db *sql.DB, siteID string) ([]Goal, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, kind, value FROM goals WHERE site_id = ? ORDER BY kind, value`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Goal{}
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Kind, &g.Value); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func AddGoal(ctx context.Context, db *sql.DB, siteID, kind, value string) (*Goal, error) {
	value = strings.TrimSpace(value)
	if !validGoal(kind, value) {
		return nil, ErrInvalidGoal
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM goals WHERE site_id = ?`, siteID).Scan(&n); err != nil {
		return nil, err
	}
	if n >= MaxGoals {
		return nil, ErrTooMany
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO goals (id, site_id, kind, value) VALUES (?, ?, ?, ?)`, id, siteID, kind, value)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrGoalExists
		}
		return nil, err
	}
	return &Goal{ID: id, Kind: kind, Value: value}, nil
}

// DeleteGoal hedefi siler; başka sitenin hedefi silinemesin diye site de koşulda.
func DeleteGoal(ctx context.Context, db *sql.DB, siteID, goalID string) (bool, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM goals WHERE id = ? AND site_id = ?`, goalID, siteID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
