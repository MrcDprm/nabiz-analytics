package site

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

// Step, huninin bir adımı: hedeflerle aynı biçim (sayfa ya da olay, sayfa "*" ile bitebilir).
type Step struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Funnel: ziyaretçinin aynı ziyarette sırayla geçmesi beklenen 2-8 adım.
type Funnel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

const (
	MinSteps   = 2
	MaxSteps   = 8
	MaxFunnels = 10
)

var (
	ErrInvalidFunnel = errors.New("huni geçersiz: ad 1-64 karakter, 2-8 adım; her adım sayfa (/ ile başlar) ya da olay adı olmalı")
	ErrTooManyFunnel = errors.New("bir sitede en fazla 10 huni olabilir")
)

func Funnels(ctx context.Context, db *sql.DB, siteID string) ([]Funnel, error) {
	rows, err := db.QueryContext(ctx, `SELECT f.id, f.name, s.kind, s.value FROM funnels f
		JOIN funnel_steps s ON s.funnel_id = f.id WHERE f.site_id = ? ORDER BY f.name, f.id, s.position`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Funnel{}
	for rows.Next() {
		var id, name string
		var st Step
		if err := rows.Scan(&id, &name, &st.Kind, &st.Value); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].ID != id {
			out = append(out, Funnel{ID: id, Name: name})
		}
		out[len(out)-1].Steps = append(out[len(out)-1].Steps, st)
	}
	return out, rows.Err()
}

func FunnelByID(ctx context.Context, db *sql.DB, siteID, id string) (*Funnel, error) {
	all, err := Funnels(ctx, db, siteID)
	if err != nil {
		return nil, err
	}
	for _, f := range all {
		if f.ID == id {
			return &f, nil
		}
	}
	return nil, ErrNotFound
}

func AddFunnel(ctx context.Context, db *sql.DB, siteID, name string, steps []Step) (*Funnel, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 || len(steps) < MinSteps || len(steps) > MaxSteps {
		return nil, ErrInvalidFunnel
	}
	for i := range steps {
		steps[i].Value = strings.TrimSpace(steps[i].Value)
		if !validGoal(steps[i].Kind, steps[i].Value) {
			return nil, ErrInvalidFunnel
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM funnels WHERE site_id = ?`, siteID).Scan(&n); err != nil {
		return nil, err
	}
	if n >= MaxFunnels {
		return nil, ErrTooManyFunnel
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO funnels (id, site_id, name) VALUES (?, ?, ?)`, id, siteID, name); err != nil {
		return nil, err
	}
	for i, st := range steps {
		if _, err := tx.ExecContext(ctx, `INSERT INTO funnel_steps (funnel_id, position, kind, value) VALUES (?, ?, ?, ?)`,
			id, i, st.Kind, st.Value); err != nil {
			return nil, err
		}
	}
	return &Funnel{ID: id, Name: name, Steps: steps}, tx.Commit()
}

func DeleteFunnel(ctx context.Context, db *sql.DB, siteID, id string) (bool, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM funnels WHERE id = ? AND site_id = ?`, id, siteID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
