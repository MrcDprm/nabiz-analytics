package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

// SessionLifetime: panel oturumu bu süre sonra biter, yeniden giriş gerekir.
const SessionLifetime = 30 * 24 * time.Hour

var (
	ErrInvalidLogin = errors.New("e-posta ya da şifre yanlış")
	ErrEmailTaken   = errors.New("bu e-posta ile bir hesap zaten var")
	ErrInvalidEmail = errors.New("geçerli bir e-posta adresi yazın")
	ErrNoSession    = errors.New("oturum yok ya da süresi doldu")
)

type User struct {
	ID       int64
	Email    string
	Verified bool
	Locale   string
}

type Service struct {
	st  *store.Store
	now func() time.Time

	// Argon2 her denemede 64 MB bellek kullanır. Aynı anda en fazla 4 şifre
	// hesaplanır; yüzlerce eşzamanlı giriş denemesi sunucunun belleğini bitiremez.
	slots chan struct{}

	// Kullanıcı yoksa da aynı süre harcansın diye karşılaştırılan sahte özet:
	// cevap süresinden "bu e-posta kayıtlı mı" anlaşılamaz.
	dummyHash string
}

func NewService(st *store.Store, now func() time.Time) (*Service, error) {
	if now == nil {
		now = time.Now
	}
	dummy, err := HashPassword("nabiz-sahte-sifre-1")
	if err != nil {
		return nil, err
	}
	return &Service{st: st, now: now, slots: make(chan struct{}, 4), dummyHash: dummy}, nil
}

// NormalizeEmail adresi doğrular ve küçük harfe çevirir.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a, err := mail.ParseAddress(email)
	// ParseAddress "Ad <a@b.com>" biçimini de kabul eder; biz sadece çıplak adres istiyoruz.
	if err != nil || a.Address != email || len(email) > 254 || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func (s *Service) hash(ctx context.Context, password string) (string, error) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return HashPassword(password)
}

func (s *Service) verify(ctx context.Context, password, encoded string) (bool, error) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return false, ctx.Err()
	}
	return VerifyPassword(password, encoded)
}

// CreateUser yeni hesap açar. verified: komut satırından açılan hesaplar
// doğrulanmış sayılır (e-posta doğrulaması kayıt sayfasıyla gelecek).
func (s *Service) CreateUser(ctx context.Context, email, password string, verified bool) (*User, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if err := CheckPassword(password, email); err != nil {
		return nil, err
	}
	h, err := s.hash(ctx, password)
	if err != nil {
		return nil, err
	}
	now := s.now().Unix()
	var verifiedAt any
	if verified {
		verifiedAt = now
	}
	res, err := s.st.W.ExecContext(ctx, `INSERT INTO users (email, password_hash, email_verified_at, created_at) VALUES (?, ?, ?, ?)`,
		email, h, verifiedAt, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, Verified: verified, Locale: "tr"}, nil
}

// UserByEmail kullanıcıyı e-postasıyla bulur (komut satırı için).
func (s *Service) UserByEmail(ctx context.Context, email string) (*User, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	u := &User{}
	var verified sql.NullInt64
	err = s.st.R.QueryRowContext(ctx, `SELECT id, email, email_verified_at, locale FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &verified, &u.Locale)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidLogin
	}
	u.Verified = verified.Valid
	return u, err
}

// Login e-posta ve şifreyi kontrol eder, doğruysa yeni oturum açar.
// Yanlış e-posta da yanlış şifre de aynı hatayı ve aynı süreyi verir.
func (s *Service) Login(ctx context.Context, email, password string) (*User, string, error) {
	if len(password) > MaxPasswordLen*4 { // bayt sınırı; Argon2'ye dev girdi verilmesin
		return nil, "", ErrInvalidLogin
	}
	email, _ = NormalizeEmail(email) // geçersizse de sahte özetle aynı süre harcanır
	u := &User{}
	var hash string
	var verified sql.NullInt64
	err := s.st.R.QueryRowContext(ctx, `SELECT id, email, password_hash, email_verified_at, locale FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &hash, &verified, &u.Locale)
	if errors.Is(err, sql.ErrNoRows) {
		s.verify(ctx, password, s.dummyHash)
		return nil, "", ErrInvalidLogin
	}
	if err != nil {
		return nil, "", err
	}
	ok, err := s.verify(ctx, password, hash)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", ErrInvalidLogin
	}
	u.Verified = verified.Valid
	token, err := s.createSession(ctx, u.ID)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// newToken 32 rastgele bayt üretir; tarayıcıya giden hâli ve veritabanına
// giden SHA-256 özeti. Veritabanı sızsa bile oturumlar ele geçirilemez.
func newToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// StartSession kullanıcı için yeni bir oturum açar (ör. davetle hesap açınca).
func (s *Service) StartSession(ctx context.Context, userID int64) (string, error) {
	return s.createSession(ctx, userID)
}

func (s *Service) createSession(ctx context.Context, userID int64) (string, error) {
	token, h, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = s.st.W.ExecContext(ctx, `INSERT INTO auth_sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		h, userID, s.now().Add(SessionLifetime).Unix())
	return token, err
}

// UserBySession çerezdeki token'ın sahibini döndürür.
func (s *Service) UserBySession(ctx context.Context, token string) (*User, error) {
	if token == "" || len(token) > 100 {
		return nil, ErrNoSession
	}
	u := &User{}
	var verified sql.NullInt64
	err := s.st.R.QueryRowContext(ctx, `SELECT u.id, u.email, u.email_verified_at, u.locale
		FROM auth_sessions a JOIN users u ON u.id = a.user_id
		WHERE a.token_hash = ? AND a.expires_at > ?`, hashToken(token), s.now().Unix()).
		Scan(&u.ID, &u.Email, &verified, &u.Locale)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSession
	}
	u.Verified = verified.Valid
	return u, err
}

// Logout oturumu siler; aynı token bir daha kullanılamaz.
func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.st.W.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

// Sweep süresi dolmuş oturumları siler.
func (s *Service) Sweep(ctx context.Context) error {
	_, err := s.st.W.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at <= ?`, s.now().Unix())
	return err
}
