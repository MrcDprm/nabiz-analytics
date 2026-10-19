package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// E-posta token'larının ömrü.
const (
	VerifyLifetime = 24 * time.Hour
	ResetLifetime  = time.Hour
)

var (
	ErrBadToken      = errors.New("bağlantı geçersiz ya da süresi dolmuş")
	ErrWrongPassword = errors.New("mevcut şifre yanlış")
	ErrSameEmail     = errors.New("bu zaten hesabının e-posta adresi")
)

// Mailer, auth paketinin e-posta göndermek için ihtiyaç duyduğu tek şey.
// Gerçek gönderim (SMTP ya da log) dışarıdan verilir; testlerde sahtesi kullanılır.
type Mailer func(ctx context.Context, to, lang, kind, token string) error

// newEmailToken bir e-posta token'ı üretir ve özetini saklar. Aynı amaçla eski
// token'lar silinir: kullanıcı iki kez "şifremi unuttum" derse sadece sonuncusu çalışır.
func (s *Service) newEmailToken(ctx context.Context, tx *sql.Tx, userID int64, purpose, newEmail string, life time.Duration) (string, error) {
	token, h, err := newToken()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM email_tokens WHERE user_id = ? AND purpose = ?`, userID, purpose); err != nil {
		return "", err
	}
	var ne any
	if newEmail != "" {
		ne = newEmail
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO email_tokens (token_hash, user_id, purpose, new_email, expires_at) VALUES (?, ?, ?, ?, ?)`,
		h, userID, purpose, ne, s.now().Add(life).Unix())
	return token, err
}

// Signup hesap açar ve doğrulama e-postası gönderir. E-posta zaten kayıtlıysa
// hata vermez; o adrese "hesabın zaten var" e-postası gider. Böylece kayıt formu
// "bu adres kayıtlı mı" sorusunun cevabını sızdırmaz.
func (s *Service) Signup(ctx context.Context, email, password, lang string, send Mailer) error {
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	if err := CheckPassword(password, email); err != nil {
		return err
	}
	lang = normLang(lang)
	u, err := s.CreateUser(ctx, email, password, false)
	if errors.Is(err, ErrEmailTaken) {
		return send(ctx, email, lang, "already_taken", "")
	}
	if err != nil {
		return err
	}
	if _, err := s.st.W.ExecContext(ctx, `UPDATE users SET locale = ? WHERE id = ?`, lang, u.ID); err != nil {
		return err
	}
	return s.SendVerification(ctx, u.ID, email, lang, send)
}

func normLang(l string) string {
	if l == "en" {
		return "en"
	}
	return "tr"
}

// SendVerification yeni bir doğrulama bağlantısı gönderir (kayıtta ve "tekrar gönder"de).
func (s *Service) SendVerification(ctx context.Context, userID int64, email, lang string, send Mailer) error {
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	token, err := s.newEmailToken(ctx, tx, userID, "verify", "", VerifyLifetime)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return send(ctx, email, lang, "verify", token)
}

// useToken token'ı bulur ve siler (tek kullanımlık). Süresi geçmiş ya da yoksa ErrBadToken.
func (s *Service) useToken(ctx context.Context, tx *sql.Tx, token, purpose string) (userID int64, newEmail string, err error) {
	if token == "" || len(token) > 100 {
		return 0, "", ErrBadToken
	}
	h := hashToken(token)
	var ne sql.NullString
	var exp int64
	err = tx.QueryRowContext(ctx, `SELECT user_id, new_email, expires_at FROM email_tokens WHERE token_hash = ? AND purpose = ?`, h, purpose).
		Scan(&userID, &ne, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", ErrBadToken
	}
	if err != nil {
		return 0, "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM email_tokens WHERE token_hash = ?`, h); err != nil {
		return 0, "", err
	}
	if exp <= s.now().Unix() {
		return 0, "", ErrBadToken
	}
	return userID, ne.String, nil
}

// Verify, kayıt doğrulama bağlantısını ya da e-posta değişikliği bağlantısını işler.
func (s *Service) Verify(ctx context.Context, token string) error {
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now().Unix()
	if uid, _, err := s.useToken(ctx, tx, token, "verify"); err == nil {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, ?) WHERE id = ?`, now, uid); err != nil {
			return err
		}
		return tx.Commit()
	} else if !errors.Is(err, ErrBadToken) {
		return err
	}
	uid, newEmail, err := s.useToken(ctx, tx, token, "email_change")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE users SET email = ?, email_verified_at = ? WHERE id = ?`, newEmail, now, uid)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ErrEmailTaken // arada başka biri bu adresle kaydolmuş
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ForgotPassword, hesap varsa sıfırlama bağlantısı gönderir. Hesap yoksa da
// hata vermez (hangi adreslerin kayıtlı olduğu öğrenilemesin).
func (s *Service) ForgotPassword(ctx context.Context, email string, send Mailer) error {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil
	}
	var id int64
	var lang string
	err = s.st.R.QueryRowContext(ctx, `SELECT id, locale FROM users WHERE email = ?`, email).Scan(&id, &lang)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	token, err := s.newEmailToken(ctx, tx, id, "reset", "", ResetLifetime)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return send(ctx, email, lang, "reset", token)
}

// ResetPassword yeni şifreyi kaydeder ve bütün oturumları kapatır: şifresi
// çalınan biri sıfırladığında saldırganın açık oturumu da biter. Bağlantı
// e-postaya geldiği için adres de doğrulanmış sayılır.
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	// Şifre kuralını e-postayı bilmeden önce, uzunluk/karmaşıklık için kontrol et.
	if err := CheckPassword(password, ""); err != nil {
		return err
	}
	h, err := s.hash(ctx, password)
	if err != nil {
		return err
	}
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	uid, _, err := s.useToken(ctx, tx, token, "reset")
	if err != nil {
		return err
	}
	var email string
	if err := tx.QueryRowContext(ctx, `SELECT email FROM users WHERE id = ?`, uid).Scan(&email); err != nil {
		return err
	}
	if err := CheckPassword(password, email); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, email_verified_at = COALESCE(email_verified_at, ?) WHERE id = ?`,
		h, s.now().Unix(), uid); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ?`, uid); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) checkCurrent(ctx context.Context, userID int64, password string) error {
	var hash string
	if err := s.st.R.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		return err
	}
	ok, err := s.verify(ctx, password, hash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrWrongPassword
	}
	return nil
}

// ChangePassword mevcut şifreyi doğrular, yenisini kaydeder ve bu oturum dışındaki
// bütün oturumları kapatır.
func (s *Service) ChangePassword(ctx context.Context, u *User, currentToken, current, next string) error {
	if err := s.checkCurrent(ctx, u.ID, current); err != nil {
		return err
	}
	if err := CheckPassword(next, u.Email); err != nil {
		return err
	}
	h, err := s.hash(ctx, next)
	if err != nil {
		return err
	}
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, h, u.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ? AND token_hash <> ?`, u.ID, hashToken(currentToken)); err != nil {
		return err
	}
	return tx.Commit()
}

// RequestEmailChange yeni adrese doğrulama bağlantısı gönderir; adres ancak
// bağlantı açılınca değişir. Yeni adres başka bir hesaptaysa sessizce hiçbir şey
// gönderilmez (adres sızmasın), kullanıcıya yine "bağlantı gönderildi" denir.
func (s *Service) RequestEmailChange(ctx context.Context, u *User, password, newEmail string, send Mailer) error {
	if err := s.checkCurrent(ctx, u.ID, password); err != nil {
		return err
	}
	newEmail, err := NormalizeEmail(newEmail)
	if err != nil {
		return err
	}
	if newEmail == u.Email {
		return ErrSameEmail
	}
	var exists int
	if err := s.st.R.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE email = ?`, newEmail).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	token, err := s.newEmailToken(ctx, tx, u.ID, "email_change", newEmail, VerifyLifetime)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return send(ctx, newEmail, u.Locale, "email_change", token)
}

func (s *Service) SetLocale(ctx context.Context, userID int64, lang string) error {
	_, err := s.st.W.ExecContext(ctx, `UPDATE users SET locale = ? WHERE id = ?`, normLang(lang), userID)
	return err
}

// DeleteAccount şifreyi doğrular, kullanıcının sahibi olduğu siteleri (bütün
// istatistikleriyle) ve hesabı siler. Üyelikler, oturumlar ve token'lar yabancı
// anahtarlarla birlikte gider (KVKK: silme hakkı). onDelete, ödeme aboneliğini
// iptal etmek gibi dış işler için silmeden hemen önce çağrılır.
func (s *Service) DeleteAccount(ctx context.Context, u *User, password string, onDelete func(context.Context, int64) error) error {
	if err := s.checkCurrent(ctx, u.ID, password); err != nil {
		return err
	}
	if onDelete != nil {
		if err := onDelete(ctx, u.ID); err != nil {
			return err
		}
	}
	tx, err := s.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sites WHERE id IN (SELECT site_id FROM memberships WHERE user_id = ? AND role = 'owner')`, u.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, u.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// SweepTokens süresi dolmuş e-posta token'larını siler.
func (s *Service) SweepTokens(ctx context.Context) error {
	_, err := s.st.W.ExecContext(ctx, `DELETE FROM email_tokens WHERE expires_at <= ?`, s.now().Unix())
	return err
}

// ConfirmPassword, geri alınamaz işlemlerden önce kullanıcının şifresini doğrular
// (oturum açmaz). Yanlışsa ErrWrongPassword.
func (s *Service) ConfirmPassword(ctx context.Context, userID int64, password string) error {
	return s.checkCurrent(ctx, userID, password)
}
