package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/billing"
	"github.com/MrcDprm/nabiz-analytics/internal/billing/billingtest"
	"github.com/MrcDprm/nabiz-analytics/internal/site"
)

func billingServer(t *testing.T) (*Server, *http.Cookie, *billing.Iyzico) {
	t.Helper()
	s, _ := mailServer(t, false)
	fake := billingtest.New(t)
	iyz := &billing.Iyzico{BaseURL: fake.URL, APIKey: billingtest.APIKey, SecretKey: billingtest.SecretKey, MerchantID: billingtest.MerchantID, HTTP: fake.Client}
	s.cfg.Billing = billing.New(s.store, iyz, billing.PlanCodes{
		"starter/monthly": "plan-s-ay", "starter/yearly": "plan-s-yil", "growth/monthly": "plan-g-ay", "growth/yearly": "plan-g-yil",
	}, nil)
	addUser(t, s)
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"kahve-fincanı-7"}`, nil, nil))
	return s, c, iyz
}

const checkoutBody = `{"plan":"starter","interval":"monthly","accept_terms":true,"name":"Ali","surname":"Yılmaz",
	"phone":"0555 111 22 33","identity":"10000000146","address":"Bir sokak 1","city":"İstanbul","country":"Türkiye"}`

func TestValidTCKN(t *testing.T) {
	for in, want := range map[string]bool{
		"10000000146": true, "11111111111": true, "10000000145": false, "01234567890": false, "1234567890": false, "1000000014a": false,
	} {
		if validTCKN(in) != want {
			t.Errorf("%s → %v", in, !want)
		}
	}
}

func TestBillingDisabledInSelfhost(t *testing.T) {
	s, _ := newTestServer(t)
	addUser(t, s)
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"kahve-fincanı-7"}`, nil, nil))
	if rec := do(s, "GET", "/api/billing", "", c, nil); !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("selfhost = %s", rec.Body)
	}
	// Kendi sunucunda site sınırı yok.
	u, _ := s.auth.UserByEmail(context.Background(), "ali@example.com")
	for _, d := range []string{"a.com", "b.com", "c.com"} {
		site.Create(context.Background(), s.store, d, u.ID)
	}
	if rec := do(s, "POST", "/api/sites", `{"domain":"d.com"}`, c, nil); rec.Code != http.StatusCreated {
		t.Fatalf("selfhost 4. site = %d", rec.Code)
	}
}

func TestCheckoutValidation(t *testing.T) {
	s, c, _ := billingServer(t)
	cases := map[string]string{
		"sözleşme onayı yok": strings.Replace(checkoutBody, `"accept_terms":true`, `"accept_terms":false`, 1),
		"geçersiz TCKN":      strings.Replace(checkoutBody, "10000000146", "12345678901", 1),
		"geçersiz telefon":   strings.Replace(checkoutBody, "0555 111 22 33", "abc", 1),
		"boş şehir":          strings.Replace(checkoutBody, `"city":"İstanbul"`, `"city":" "`, 1),
		"HTML adres":         strings.Replace(checkoutBody, "Bir sokak 1", "<script>", 1),
		"ücretsiz paket":     strings.Replace(checkoutBody, `"plan":"starter"`, `"plan":"free"`, 1),
	}
	for name, body := range cases {
		if rec := do(s, "POST", "/api/billing/checkout", body, c, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestCheckoutToActive(t *testing.T) {
	s, c, iyz := billingServer(t)
	rec := do(s, "POST", "/api/billing/checkout", checkoutBody, c, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("başlat = %d %s", rec.Code, rec.Body)
	}
	var out struct{ URL string }
	json.NewDecoder(rec.Body).Decode(&out)
	if !strings.HasPrefix(out.URL, "/billing/pay/") {
		t.Fatalf("url = %q", out.URL)
	}

	// Ödeme sayfası: iyzico betiği nonce ile; CSP sadece bu sayfada gevşek.
	rec = do(s, "GET", out.URL, "", c, nil)
	csp := rec.Header().Get("Content-Security-Policy")
	if rec.Code != 200 || !strings.Contains(csp, "'strict-dynamic'") || !strings.Contains(rec.Body.String(), `<script nonce="`) || !strings.Contains(rec.Body.String(), "iyzipay-checkout-form") {
		t.Fatalf("ödeme sayfası = %d %q\n%s", rec.Code, csp, rec.Body)
	}
	nonce := strings.Split(strings.Split(csp, "'nonce-")[1], "'")[0]
	if !strings.Contains(rec.Body.String(), `nonce="`+nonce+`"`) {
		t.Fatal("betikteki nonce CSP ile uyuşmuyor")
	}
	// Giriş yapmamış biri formu göremez.
	if rec := do(s, "GET", out.URL, "", nil, nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("girişsiz ödeme sayfası = %d", rec.Code)
	}

	// iyzico'dan dönüş: başka siteden gelen form gönderimi (CSRF korumasına takılmamalı).
	token := strings.TrimPrefix(out.URL, "/billing/pay/")
	rec = do(s, "POST", "/api/billing/callback", "token="+url.QueryEscape(token), nil, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded", "Sec-Fetch-Site": "cross-site", "Origin": "https://sandbox-api.iyzipay.com",
	})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/billing?status=ok" {
		t.Fatalf("dönüş = %d %s", rec.Code, rec.Header().Get("Location"))
	}
	rec = do(s, "GET", "/api/billing", "", c, nil)
	if !strings.Contains(rec.Body.String(), `"state":"active"`) || !strings.Contains(rec.Body.String(), `"id":"starter"`) {
		t.Fatalf("durum = %s", rec.Body)
	}

	// Webhook: imzasız 401, imzalı 200.
	ev := billing.WebhookEvent{OrderReferenceCode: "s1", CustomerReferenceCode: "musteri-1", SubscriptionReferenceCode: "abonelik-1", IyziReferenceCode: "olay-1", IyziEventType: "subscription.order.success"}
	body, _ := json.Marshal(ev)
	if rec := do(s, "POST", "/api/billing/webhook", string(body), nil, map[string]string{"X-IYZ-SIGNATURE-V3": "uydurma"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("imzasız webhook = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/billing/webhook", string(body), nil, map[string]string{"X-IYZ-SIGNATURE-V3": iyz.WebhookSignature(ev)}); rec.Code != http.StatusOK {
		t.Fatalf("imzalı webhook = %d", rec.Code)
	}

	// Paket değiştir ve iptal.
	if rec := do(s, "POST", "/api/billing/change", `{"plan":"growth","interval":"monthly"}`, c, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("değiştir = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "POST", "/api/billing/cancel", "", c, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("iptal = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "GET", "/api/billing", "", c, nil); !strings.Contains(rec.Body.String(), `"state":"canceled"`) {
		t.Fatalf("iptal sonrası = %s", rec.Body)
	}
}

func TestFailedCallbackRedirects(t *testing.T) {
	s, _, _ := billingServer(t)
	rec := do(s, "POST", "/api/billing/callback", "token=uydurma", nil, map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if rec.Header().Get("Location") != "/billing?status=failed" {
		t.Fatalf("uydurma dönüş = %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

func TestFreeHostingWithoutPayments(t *testing.T) {
	s, _ := mailServer(t, true)
	s.cfg.Billing = billing.New(s.store, nil, nil, nil)
	addUser(t, s)
	c := sessionFrom(t, do(s, "POST", "/api/login", `{"email":"ali@example.com","password":"kahve-fincanı-7"}`, nil, nil))
	if rec := do(s, "GET", "/api/billing", "", c, nil); !strings.Contains(rec.Body.String(), `"payments":false`) {
		t.Fatalf("durum = %s", rec.Body)
	}
	if rec := do(s, "POST", "/api/billing/checkout", checkoutBody, c, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("ödeme kapalıyken başlat = %d", rec.Code)
	}
	do(s, "POST", "/api/sites", `{"domain":"a.com"}`, c, nil)
	do(s, "POST", "/api/sites", `{"domain":"b.com"}`, c, nil)
	rec := do(s, "POST", "/api/sites", `{"domain":"c.com"}`, c, nil)
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), "kendi sunucuna") {
		t.Fatalf("3. site = %d %s", rec.Code, rec.Body)
	}
}

func TestFreePlanLimits(t *testing.T) {
	s, c, _ := billingServer(t)
	for _, d := range []string{"a.com", "b.com"} {
		if rec := do(s, "POST", "/api/sites", `{"domain":"`+d+`"}`, c, nil); rec.Code != http.StatusCreated {
			t.Fatalf("%s = %d", d, rec.Code)
		}
	}
	rec := do(s, "POST", "/api/sites", `{"domain":"c.com"}`, c, nil)
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), "site sınırı") {
		t.Fatalf("ücretsizde 3. site = %d %s", rec.Code, rec.Body)
	}
	id := siteID(t, s, "a.com")
	if rec := do(s, "POST", "/api/sites/"+id+"/invites", `{"email":"veli@example.com","role":"viewer"}`, c, nil); rec.Code != http.StatusPaymentRequired {
		t.Fatalf("ücretsizde davet = %d", rec.Code)
	}
}
