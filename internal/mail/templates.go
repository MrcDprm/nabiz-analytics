package mail

import (
	"fmt"
	"strings"
)

// Kind, hesap e-postasının türü.
type Kind string

const (
	Verify       Kind = "verify"        // kayıttan sonra e-posta doğrulama
	Reset        Kind = "reset"         // şifre sıfırlama
	EmailChange  Kind = "email_change"  // yeni adresi doğrulama
	AlreadyTaken Kind = "already_taken" // bu adresle tekrar kayıt denendi
	Invite       Kind = "invite"        // bir siteye takım daveti (%[2]s: site, %[3]s: rol)
)

type template struct{ subject, body string }

// %[1]s: bağlantı. Metinler kısa ve düz; ne olduğu, ne yapılacağı, ne zaman biteceği.
var templates = map[string]map[Kind]template{
	"tr": {
		Verify: {"Nabız hesabını doğrula", `Merhaba,

Nabız hesabını açtın. E-posta adresini doğrulamak için bu bağlantıyı aç:

%[1]s

Bağlantı 24 saat geçerli. Bu hesabı sen açmadıysan bu e-postayı yok sayabilirsin.

Nabız`},
		Reset: {"Nabız şifreni sıfırla", `Merhaba,

Nabız hesabın için şifre sıfırlama istendi. Yeni şifre belirlemek için bu bağlantıyı aç:

%[1]s

Bağlantı 1 saat geçerli ve bir kez kullanılabilir. Bunu sen istemediysen bu e-postayı yok say; şifren değişmez.

Nabız`},
		EmailChange: {"Nabız için yeni e-posta adresini doğrula", `Merhaba,

Nabız hesabının e-posta adresini bu adresle değiştirmek istedin. Onaylamak için bu bağlantıyı aç:

%[1]s

Bağlantı 24 saat geçerli. Bunu sen istemediysen bu e-postayı yok say.

Nabız`},
		AlreadyTaken: {"Nabız hesabın zaten var", `Merhaba,

Bu e-posta adresiyle yeni bir Nabız hesabı açılmaya çalışıldı, ama bu adresle zaten bir hesabın var. Giriş yapmak için:

%[1]s

Şifreni hatırlamıyorsan giriş sayfasındaki "Şifremi unuttum" bağlantısını kullanabilirsin. Bunu sen yapmadıysan bu e-postayı yok say.

Nabız`},
		Invite: {"Nabız: %[2]s sitesine davet edildin", `Merhaba,

%[2]s sitesinin Nabız paneline %[3]s olarak davet edildin. Daveti kabul etmek için bu bağlantıyı aç:

%[1]s

Davet 7 gün geçerli. Hesabın yoksa bağlantıdan bir şifre belirleyerek hemen açabilirsin. Bu daveti beklemiyorsan e-postayı yok say.

Nabız`},
	},
	"en": {
		Verify: {"Verify your Nabız account", `Hi,

You created a Nabız account. Open this link to verify your email address:

%[1]s

The link works for 24 hours. If you didn't create this account, you can ignore this email.

Nabız`},
		Reset: {"Reset your Nabız password", `Hi,

Someone asked to reset the password of your Nabız account. Open this link to set a new password:

%[1]s

The link works for 1 hour and only once. If it wasn't you, ignore this email; your password stays the same.

Nabız`},
		EmailChange: {"Verify your new email for Nabız", `Hi,

You asked to change the email of your Nabız account to this address. Open this link to confirm:

%[1]s

The link works for 24 hours. If it wasn't you, ignore this email.

Nabız`},
		AlreadyTaken: {"You already have a Nabız account", `Hi,

Someone tried to create a new Nabız account with this email, but you already have one. To log in:

%[1]s

If you forgot your password, use "Forgot password" on the login page. If it wasn't you, ignore this email.

Nabız`},
		Invite: {"Nabız: you're invited to %[2]s", `Hi,

You've been invited to the Nabız dashboard of %[2]s as %[3]s. Open this link to accept:

%[1]s

The invite works for 7 days. If you don't have an account, you can create one from the link by setting a password. If you weren't expecting this, ignore this email.

Nabız`},
	},
}

// Render konu ve gövdeyi dile göre üretir; bilinmeyen dilde İngilizce.
// extra, şablondaki %[2]s, %[3]s... yerine geçer (ör. davette site ve rol).
func Render(lang string, kind Kind, link string, extra ...string) (subject, body string) {
	t, ok := templates[lang]
	if !ok {
		t = templates["en"]
	}
	tpl := t[kind]
	args := []any{link}
	for _, e := range extra {
		args = append(args, e)
	}
	subject = tpl.subject
	if strings.Contains(subject, "%") {
		subject = fmt.Sprintf(subject, args...)
	}
	return subject, fmt.Sprintf(tpl.body, args...)
}
