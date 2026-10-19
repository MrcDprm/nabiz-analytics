// Package billingtest, testler için sahte bir iyzico sunucusu sağlar. Sadece
// testlerden içe aktarılır, programa girmez. billing paketine bağlı değildir;
// böylece billing'in kendi testleri de kullanabilir.
package billingtest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const (
	APIKey     = "api-anahtari"
	SecretKey  = "gizli-anahtar"
	MerchantID = "12345"
	EndDateMs  = 1_800_000_000_000 // ödemeden sonraki dönem sonu
	RenewedMs  = 1_900_000_000_000 // yenilemeden sonraki dönem sonu
)

// Fake, iyzico abonelik API'sinin küçük bir taklidi. Her isteğin IYZWSv2
// imzasını belgedeki formülle yeniden hesaplar; tutmazsa "failure" döner.
type Fake struct {
	URL    string
	Client *http.Client

	mu        sync.Mutex
	checkouts map[string]string // form token → plan kodu
	subs      map[string]map[string]any
	Calls     []string
	nextRef   int
}

func New(t *testing.T) *Fake {
	f := &Fake{checkouts: map[string]string{}, subs: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	f.URL, f.Client = srv.URL, srv.Client()
	return f
}

func (f *Fake) LastCall() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Calls) == 0 {
		return ""
	}
	return f.Calls[len(f.Calls)-1]
}

func checkAuth(r *http.Request, body []byte) bool {
	h := r.Header.Get("Authorization")
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "IYZWSv2 "))
	if !strings.HasPrefix(h, "IYZWSv2 ") || err != nil {
		return false
	}
	parts := map[string]string{}
	for _, kv := range strings.Split(string(raw), "&") {
		k, v, _ := strings.Cut(kv, ":")
		parts[k] = v
	}
	mac := hmac.New(sha256.New, []byte(SecretKey))
	mac.Write([]byte(parts["randomKey"] + r.URL.Path + string(body)))
	return parts["apiKey"] == APIKey && parts["randomKey"] == r.Header.Get("x-iyzi-rnd") &&
		hmac.Equal([]byte(parts["signature"]), []byte(hex.EncodeToString(mac.Sum(nil))))
}

func (f *Fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, r.Method+" "+r.URL.Path)
	fail := func(code, msg string) {
		json.NewEncoder(w).Encode(map[string]any{"status": "failure", "errorCode": code, "errorMessage": msg})
	}
	reply := func(v map[string]any) {
		v["status"] = "success"
		json.NewEncoder(w).Encode(v)
	}
	if !checkAuth(r, body) {
		fail("1000", "Geçersiz imza")
		return
	}
	newSub := func(code string) map[string]any {
		f.nextRef++
		sub := map[string]any{
			"referenceCode": "abonelik-" + string(rune('0'+f.nextRef)), "customerReferenceCode": "musteri-1",
			"pricingPlanReferenceCode": code, "subscriptionStatus": "ACTIVE", "endDate": int64(EndDateMs),
		}
		f.subs[sub["referenceCode"].(string)] = sub
		return sub
	}
	p := r.URL.Path
	switch {
	case p == "/v2/subscription/checkoutform/initialize":
		var in struct {
			PricingPlanReferenceCode string `json:"pricingPlanReferenceCode"`
			Customer                 struct {
				IdentityNumber string `json:"identityNumber"`
			} `json:"customer"`
		}
		json.Unmarshal(body, &in)
		if in.Customer.IdentityNumber == "" {
			fail("2", "kimlik no")
			return
		}
		tok := "form-" + in.PricingPlanReferenceCode
		f.checkouts[tok] = in.PricingPlanReferenceCode
		reply(map[string]any{"token": tok, "checkoutFormContent": `<script type="text/javascript">var iyziInit = {token:"` + tok + `"};</script>`})
	case strings.HasPrefix(p, "/v2/subscription/checkoutform/"):
		tok := strings.TrimPrefix(p, "/v2/subscription/checkoutform/")
		code, ok := f.checkouts[tok]
		if !ok {
			fail("3", "form yok")
			return
		}
		reply(map[string]any{"data": newSub(code)})
	case strings.HasSuffix(p, "/cancel"):
		reply(map[string]any{})
	case strings.HasSuffix(p, "/upgrade"):
		var in struct {
			NewPricingPlanReferenceCode string `json:"newPricingPlanReferenceCode"`
		}
		json.Unmarshal(body, &in)
		reply(map[string]any{"data": newSub(in.NewPricingPlanReferenceCode)})
	case strings.HasPrefix(p, "/v2/subscription/subscriptions/"):
		sub := f.subs[strings.TrimPrefix(p, "/v2/subscription/subscriptions/")]
		if sub == nil {
			reply(map[string]any{"data": map[string]any{"items": []any{}}})
			return
		}
		cp := map[string]any{}
		for k, v := range sub {
			cp[k] = v
		}
		cp["endDate"] = int64(RenewedMs)
		reply(map[string]any{"data": map[string]any{"items": []any{cp}}})
	default:
		http.NotFound(w, r)
	}
}
