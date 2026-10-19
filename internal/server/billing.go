package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/MrcDprm/nabiz-analytics/internal/auth"
	"github.com/MrcDprm/nabiz-analytics/internal/billing"
)

func billingError(w http.ResponseWriter, where string, err error) {
	var apiErr *billing.APIError
	switch {
	case errors.Is(err, billing.ErrUnknownPlan), errors.Is(err, billing.ErrAlreadyActive), errors.Is(err, billing.ErrNoActive),
		errors.Is(err, billing.ErrSameInterval), errors.Is(err, billing.ErrBadCheckout):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, billing.ErrSiteLimit), errors.Is(err, billing.ErrMemberLimit), errors.Is(err, billing.ErrOverUsage):
		writeError(w, http.StatusPaymentRequired, err.Error())
	case errors.As(err, &apiErr):
		// iyzico'nun hata metni kullanıcıya gösterilmez (iç ayrıntı olabilir), log'a gider.
		log.Printf("iyzico (%s): %v", where, err)
		writeError(w, http.StatusBadGateway, "ödeme sağlayıcısı isteği reddetti; bilgilerini kontrol edip tekrar dene")
	default:
		internalError(w, where, err)
	}
}

// billingStatus, plan sayfası için: paketler, kullanıcının durumu ve kullanımı.
// Kendi sunucunda kurulumda ödeme kapalıdır.
func (s *Server) billingStatus(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if s.cfg.Billing == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	st, err := s.cfg.Billing.StatusOf(r.Context(), u.ID)
	if err != nil {
		internalError(w, "paket", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "plans": billing.Plans, "status": st})
}

var (
	digitsOnly = regexp.MustCompile(`^[0-9]+$`)
	phoneRe    = regexp.MustCompile(`^\+?[0-9 ]{10,16}$`)
)

func clean(s string, max int) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && utf8.RuneCountInString(s) <= max && !strings.ContainsAny(s, "<>\r\n")
}

// validTCKN, TC kimlik numarasının algoritma kontrolü (11 hane, ilk hane 0 değil,
// 10. ve 11. hane sağlama). iyzico yabancılar için 11111111111 kabul ediyor.
func validTCKN(s string) bool {
	if len(s) != 11 || !digitsOnly.MatchString(s) || s[0] == '0' {
		return false
	}
	if s == "11111111111" {
		return true
	}
	d := make([]int, 11)
	for i := range s {
		d[i] = int(s[i] - '0')
	}
	odd := d[0] + d[2] + d[4] + d[6] + d[8]
	even := d[1] + d[3] + d[5] + d[7]
	if ((odd*7-even)%10+10)%10 != d[9] {
		return false
	}
	sum := 0
	for _, v := range d[:10] {
		sum += v
	}
	return sum%10 == d[10]
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if s.cfg.Billing == nil {
		writeError(w, http.StatusNotFound, "ödeme kapalı")
		return
	}
	if !u.Verified {
		writeError(w, http.StatusForbidden, "ödemeden önce e-posta adresinizi doğrulayın")
		return
	}
	if s.tooMany(w, r, s.accountLimit) {
		return
	}
	var in struct {
		Plan        string `json:"plan"`
		Interval    string `json:"interval"`
		AcceptTerms bool   `json:"accept_terms"`
		Name        string `json:"name"`
		Surname     string `json:"surname"`
		Phone       string `json:"phone"`
		Identity    string `json:"identity"`
		Address     string `json:"address"`
		City        string `json:"city"`
		Country     string `json:"country"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if !in.AcceptTerms {
		writeError(w, http.StatusBadRequest, "devam etmek için sözleşmeleri onaylaman gerekiyor")
		return
	}
	name, ok1 := clean(in.Name, 50)
	surname, ok2 := clean(in.Surname, 50)
	address, ok3 := clean(in.Address, 300)
	city, ok4 := clean(in.City, 50)
	country, ok5 := clean(in.Country, 50)
	phone := strings.ReplaceAll(strings.TrimSpace(in.Phone), " ", "")
	if !(ok1 && ok2 && ok3 && ok4 && ok5) {
		writeError(w, http.StatusBadRequest, "fatura bilgilerini eksiksiz doldur")
		return
	}
	if !phoneRe.MatchString(phone) {
		writeError(w, http.StatusBadRequest, "telefon numarası geçersiz (ör. +905551112233)")
		return
	}
	if !strings.HasPrefix(phone, "+") {
		phone = "+90" + strings.TrimPrefix(phone, "0")
	}
	if !validTCKN(in.Identity) {
		writeError(w, http.StatusBadRequest, "TC kimlik numarası geçersiz")
		return
	}
	cust := billing.Customer{
		Name: name, Surname: surname, Email: u.Email, GsmNumber: phone, IdentityNumber: in.Identity,
		BillingAddress: billing.Address{ContactName: name + " " + surname, City: city, Country: country, Address: address},
	}
	locale := "tr"
	if u.Locale == "en" {
		locale = "en"
	}
	token, err := s.cfg.Billing.StartCheckout(r.Context(), u.ID, in.Plan, in.Interval, s.cfg.BaseURL+"/api/billing/callback", locale, cust)
	if err != nil {
		billingError(w, "ödeme başlat", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": "/billing/pay/" + token})
}

// payPageCSP, sadece ödeme sayfası için gevşetilmiş CSP: iyzico'nun ödeme formu
// betiği (nonce ile işaretlenmiş) ve onun yüklediği betikler ('strict-dynamic'),
// iyzico'nun çerçeveleri ve istekleri çalışabilir. Panelin diğer sayfaları sıkı kalır.
// form-action yok: 3D Secure doğrulaması bankanın kendi sayfasına form gönderir.
func payPageCSP(nonce string) string {
	return "default-src 'none'; script-src 'nonce-" + nonce + "' 'strict-dynamic' https:; " +
		"style-src 'self' 'unsafe-inline' https://*.iyzipay.com; img-src 'self' data: https:; font-src 'self' data: https:; " +
		"connect-src https://*.iyzipay.com; frame-src https://*.iyzipay.com https:; base-uri 'none'; frame-ancestors 'none'"
}

// payPage, GET /billing/pay/{token}: iyzico ödeme formunun çalıştığı sayfa.
func (s *Server) payPage(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r)
	if err != nil || u == nil || s.cfg.Billing == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	content, err := s.cfg.Billing.CheckoutContent(r.Context(), r.PathValue("token"), u.ID)
	if err != nil {
		http.Redirect(w, r, "/billing?status=expired", http.StatusSeeOther)
		return
	}
	b := make([]byte, 16)
	rand.Read(b)
	nonce := base64.RawStdEncoding.EncodeToString(b)
	// iyzico'nun betiğine nonce ekle; başka hiçbir satır içi betik çalışamaz.
	content = strings.ReplaceAll(content, "<script", `<script nonce="`+nonce+`"`)
	lang := "tr"
	title, back := "Ödeme", "Vazgeç ve plan sayfasına dön"
	if u.Locale == "en" {
		lang, title, back = "en", "Payment", "Cancel and go back to plans"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", payPageCSP(nonce))
	io.WriteString(w, `<!doctype html><html lang="`+lang+`"><head><meta charset="utf-8">`+
		`<meta name="viewport" content="width=device-width, initial-scale=1"><title>`+html.EscapeString(title)+` | Nabız</title>`+
		`<link rel="icon" href="/favicon.svg"><style nonce="`+nonce+`">body{margin:0;font-family:system-ui,sans-serif;background:#fffaf8;color:#1d2b3f}`+
		`main{max-width:640px;margin:0 auto;padding:32px 16px}a{color:inherit}h1{font-size:24px}</style></head><body><main>`+
		`<h1>`+html.EscapeString(title)+`</h1><div id="iyzipay-checkout-form" class="responsive"></div>`+
		`<p><a href="/billing">`+html.EscapeString(back)+`</a></p></main>`+content+`</body></html>`)
}

// billingCallback, POST /api/billing/callback: iyzico ödeme formu bitince tarayıcıyı
// buraya gönderir. Gelen isteğe güvenilmez; sonuç iyzico'ya sorulur.
func (s *Server) billingCallback(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Billing == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	token := r.FormValue("token")
	if _, err := s.cfg.Billing.Complete(r.Context(), token); err != nil {
		log.Printf("ödeme sonucu: %v", err)
		http.Redirect(w, r, "/billing?status=failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/billing?status=ok", http.StatusSeeOther)
}

// billingWebhook, POST /api/billing/webhook: iyzico'nun aylık çekim bildirimi.
func (s *Server) billingWebhook(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Billing == nil {
		http.NotFound(w, r)
		return
	}
	var ev billing.WebhookEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&ev); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err := s.cfg.Billing.HandleWebhook(r.Context(), ev, r.Header.Get("X-IYZ-SIGNATURE-V3"))
	if errors.Is(err, billing.ErrBadSignature) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if err != nil {
		// 500: iyzico bildirimi daha sonra tekrar dener.
		log.Printf("webhook: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) cancelSubscription(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if s.cfg.Billing == nil {
		writeError(w, http.StatusNotFound, "ödeme kapalı")
		return
	}
	if err := s.cfg.Billing.Cancel(r.Context(), u.ID); err != nil {
		billingError(w, "iptal", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) changePlan(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if s.cfg.Billing == nil {
		writeError(w, http.StatusNotFound, "ödeme kapalı")
		return
	}
	var in struct {
		Plan     string `json:"plan"`
		Interval string `json:"interval"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.cfg.Billing.ChangePlan(r.Context(), u.ID, in.Plan, in.Interval); err != nil {
		billingError(w, "paket değiştir", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
