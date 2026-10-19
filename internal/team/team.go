// Package team, bir sitenin üyelerini yönetir: davet, rol değiştirme, çıkarma, sahiplik devri.
//
// Kurallar (sunucuda, her istekte):
//   - Sahip ve yönetici davet gönderebilir (yönetici ya da izleyici olarak).
//   - Sahip ve yönetici, sahip dışındaki üyelerin rolünü değiştirebilir ve onları çıkarabilir.
//   - Herkes kendi isteğiyle ayrılabilir; sahip ise önce sahipliği devretmeli.
//   - Sahipliği sadece sahip, mevcut bir üyeye devredebilir; eski sahip yönetici olur.
package team

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

const InviteLifetime = 7 * 24 * time.Hour

var (
	ErrBadRole      = errors.New("rol yönetici ya da izleyici olmalı")
	ErrAlreadyIn    = errors.New("bu kişi zaten sitenin üyesi")
	ErrNotMember    = errors.New("bu kişi sitenin üyesi değil")
	ErrOwner        = errors.New("sahibin rolü değiştirilemez; önce sahipliği devret")
	ErrBadInvite    = errors.New("davet geçersiz ya da süresi dolmuş")
	ErrWrongAccount = errors.New("bu davet başka bir e-posta adresine gönderildi")
	ErrSelf         = errors.New("sahiplik zaten sende")
)

type Member struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type Invite struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

// InviteInfo, davet bağlantısını açan kişiye gösterilen bilgi.
type InviteInfo struct {
	SiteID     string `json:"-"`
	Domain     string `json:"domain"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	HasAccount bool   `json:"has_account"`
}

type Team struct {
	st  *store.Store
	now func() time.Time
}

func New(st *store.Store, now func() time.Time) *Team {
	if now == nil {
		now = time.Now
	}
	return &Team{st: st, now: now}
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func (t *Team) Members(ctx context.Context, siteID string) ([]Member, []Invite, error) {
	rows, err := t.st.R.QueryContext(ctx, `SELECT u.email, m.role FROM memberships m JOIN users u ON u.id = m.user_id
		WHERE m.site_id = ? ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.email`, siteID)
	if err != nil {
		return nil, nil, err
	}
	members := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.Email, &m.Role); err != nil {
			rows.Close()
			return nil, nil, err
		}
		members = append(members, m)
	}
	rows.Close()
	rows, err = t.st.R.QueryContext(ctx, `SELECT email, role, expires_at FROM invites
		WHERE site_id = ? AND expires_at > ? ORDER BY email`, siteID, t.now().Unix())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	invites := []Invite{}
	for rows.Next() {
		var inv Invite
		var exp int64
		if err := rows.Scan(&inv.Email, &inv.Role, &exp); err != nil {
			return nil, nil, err
		}
		inv.ExpiresAt = time.Unix(exp, 0).UTC()
		invites = append(invites, inv)
	}
	return members, invites, rows.Err()
}

// CreateInvite davet token'ı üretir. Aynı adrese eski bir davet varsa yenisiyle değişir.
// email önceden NormalizeEmail'den geçmiş olmalı.
func (t *Team) CreateInvite(ctx context.Context, siteID, email, role string, invitedBy int64) (string, error) {
	if role != "admin" && role != "viewer" {
		return "", ErrBadRole
	}
	var n int
	if err := t.st.R.QueryRowContext(ctx, `SELECT count(*) FROM memberships m JOIN users u ON u.id = m.user_id
		WHERE m.site_id = ? AND u.email = ?`, siteID, email).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "", ErrAlreadyIn
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	tx, err := t.st.W.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM invites WHERE site_id = ? AND email = ?`, siteID, email); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO invites (token_hash, site_id, email, role, invited_by, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		hashToken(token), siteID, email, role, invitedBy, t.now().Add(InviteLifetime).Unix()); err != nil {
		return "", err
	}
	return token, tx.Commit()
}

func (t *Team) RevokeInvite(ctx context.Context, siteID, email string) (bool, error) {
	res, err := t.st.W.ExecContext(ctx, `DELETE FROM invites WHERE site_id = ? AND email = ?`, siteID, email)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Info, davet token'ının neye ait olduğunu söyler (bağlantıyı açan kişiye gösterilir).
func (t *Team) Info(ctx context.Context, token string) (*InviteInfo, error) {
	if token == "" || len(token) > 100 {
		return nil, ErrBadInvite
	}
	inv := &InviteInfo{}
	var exp int64
	err := t.st.R.QueryRowContext(ctx, `SELECT i.site_id, s.domain, i.email, i.role, i.expires_at FROM invites i
		JOIN sites s ON s.id = i.site_id WHERE i.token_hash = ?`, hashToken(token)).
		Scan(&inv.SiteID, &inv.Domain, &inv.Email, &inv.Role, &exp)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && exp <= t.now().Unix()) {
		return nil, ErrBadInvite
	}
	if err != nil {
		return nil, err
	}
	var n int
	if err := t.st.R.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE email = ?`, inv.Email).Scan(&n); err != nil {
		return nil, err
	}
	inv.HasAccount = n > 0
	return inv, nil
}

// Accept daveti kabul eder: kullanıcının e-postası davetin adresiyle aynı olmalı.
// Davet tek kullanımlık; kabul edilince silinir. Bağlantı e-postaya geldiği için
// adres doğrulanmış sayılır.
func (t *Team) Accept(ctx context.Context, token string, userID int64, userEmail string) (*InviteInfo, error) {
	info, err := t.Info(ctx, token)
	if err != nil {
		return nil, err
	}
	if info.Email != userEmail {
		return nil, ErrWrongAccount
	}
	tx, err := t.st.W.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM invites WHERE token_hash = ?`, hashToken(token))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrBadInvite // aynı anda iki kez kabul
	}
	// Zaten üyeyse (ör. başka davetle) rolünü düşürmeyiz.
	if _, err := tx.ExecContext(ctx, `INSERT INTO memberships (site_id, user_id, role) VALUES (?, ?, ?)
		ON CONFLICT (site_id, user_id) DO NOTHING`, info.SiteID, userID, info.Role); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, ?) WHERE id = ?`, t.now().Unix(), userID); err != nil {
		return nil, err
	}
	return info, tx.Commit()
}

func (t *Team) memberRole(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, siteID, email string) (int64, string, error) {
	var uid int64
	var role string
	err := q.QueryRowContext(ctx, `SELECT u.id, m.role FROM memberships m JOIN users u ON u.id = m.user_id
		WHERE m.site_id = ? AND u.email = ?`, siteID, email).Scan(&uid, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", ErrNotMember
	}
	return uid, role, err
}

// SetRole, sahip dışındaki bir üyenin rolünü değiştirir.
func (t *Team) SetRole(ctx context.Context, siteID, email, role string) error {
	if role != "admin" && role != "viewer" {
		return ErrBadRole
	}
	uid, cur, err := t.memberRole(ctx, t.st.R, siteID, email)
	if err != nil {
		return err
	}
	if cur == "owner" {
		return ErrOwner
	}
	_, err = t.st.W.ExecContext(ctx, `UPDATE memberships SET role = ? WHERE site_id = ? AND user_id = ?`, role, siteID, uid)
	return err
}

// Remove bir üyeyi siteden çıkarır (kendi ayrılması da bu). Sahip çıkarılamaz.
func (t *Team) Remove(ctx context.Context, siteID, email string) error {
	uid, cur, err := t.memberRole(ctx, t.st.R, siteID, email)
	if err != nil {
		return err
	}
	if cur == "owner" {
		return ErrOwner
	}
	_, err = t.st.W.ExecContext(ctx, `DELETE FROM memberships WHERE site_id = ? AND user_id = ?`, siteID, uid)
	return err
}

// Transfer sahipliği mevcut bir üyeye devreder; eski sahip yönetici olur. İkisi tek işlemde.
func (t *Team) Transfer(ctx context.Context, siteID string, fromUser int64, toEmail string) error {
	tx, err := t.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	uid, role, err := t.memberRole(ctx, tx, siteID, toEmail)
	if err != nil {
		return err
	}
	if role == "owner" || uid == fromUser {
		return ErrSelf
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memberships SET role = 'admin' WHERE site_id = ? AND user_id = ? AND role = 'owner'`, siteID, fromUser); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memberships SET role = 'owner' WHERE site_id = ? AND user_id = ?`, siteID, uid); err != nil {
		return err
	}
	return tx.Commit()
}

// Sweep süresi dolmuş davetleri siler.
func (t *Team) Sweep(ctx context.Context) error {
	_, err := t.st.W.ExecContext(ctx, `DELETE FROM invites WHERE expires_at <= ?`, t.now().Unix())
	return err
}
