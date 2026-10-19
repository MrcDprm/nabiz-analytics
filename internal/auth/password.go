// Package auth, panel hesaplarını yönetir: şifreler, oturumlar ve giriş.
package auth

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Argon2id ayarları (OWASP önerisinin üstünde): 64 MB bellek, 2 tur, 1 iş parçacığı.
// Bellek ağırlığı, ekran kartıyla toplu şifre denemeyi pahalı yapar.
const (
	argonMemory  = 64 * 1024 // KB
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword şifreyi "$argon2id$v=19$m=65536,t=2,p=1$tuz$özet" biçiminde saklanacak
// metne çevirir. Ayarlar metnin içinde durur; ileride ayarları artırırsak eski
// şifreler kendi ayarlarıyla doğrulanmaya devam eder.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

var errBadHash = errors.New("geçersiz şifre özeti")

// VerifyPassword şifrenin saklanan özetle eşleşip eşleşmediğini söyler.
// Karşılaştırma sabit sürede yapılır: kaç baytın tuttuğu süreden anlaşılamaz.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errBadHash
	}
	// Bozuk ya da kötü niyetli bir özet sunucuya 4 GB bellek ayırtamasın.
	if memory == 0 || memory > 256*1024 || time == 0 || time > 10 || threads == 0 {
		return false, errBadHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

//go:embed common-passwords.txt
var commonList []byte

// common, sızıntılarda en sık görülen şifreler (küçük harfle).
var common = func() map[string]bool {
	m := make(map[string]bool)
	sc := bufio.NewScanner(bytes.NewReader(commonList))
	for sc.Scan() {
		if w := strings.TrimSpace(sc.Text()); w != "" && !strings.HasPrefix(w, "#") {
			m[strings.ToLower(w)] = true
		}
	}
	return m
}()

const (
	MinPasswordLen = 10
	MaxPasswordLen = 128 // Argon2'ye megabaytlık şifre verilip işlemci yorulmasın
)

// Hata mesajları kullanıcıya olduğu gibi gösterilir.
var (
	ErrPasswordShort  = fmt.Errorf("şifre en az %d karakter olmalı", MinPasswordLen)
	ErrPasswordLong   = fmt.Errorf("şifre en fazla %d karakter olabilir", MaxPasswordLen)
	ErrPasswordSimple = errors.New("şifrede en az bir harf ve bir rakam olmalı")
	ErrPasswordCommon = errors.New("bu şifre çok yaygın, başka bir şifre seçin")
	ErrPasswordEmail  = errors.New("şifre e-posta adresinizi içermemeli")
)

// CheckPassword zayıf şifreleri reddeder.
func CheckPassword(password, email string) error {
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordLen:
		return ErrPasswordShort
	case n > MaxPasswordLen:
		return ErrPasswordLong
	}
	var letter, digit bool
	for _, r := range password {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	if !letter || !digit {
		return ErrPasswordSimple
	}
	lower := strings.ToLower(password)
	if common[lower] {
		return ErrPasswordCommon
	}
	if local, _, ok := strings.Cut(strings.ToLower(email), "@"); ok && len(local) >= 4 && strings.Contains(lower, local) {
		return ErrPasswordEmail
	}
	return nil
}
