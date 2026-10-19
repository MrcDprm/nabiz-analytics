package billing

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

// fakeIyzico, iyzico abonelik API'sinin küçük bir taklidi. Her isteğin IYZWSv2
// imzasını belgedeki formülle yeniden hesaplar; tutmazsa "failure" döner.
type fakeIyzico struct {
	t      *testing.T
	apiKey string
	secret string

	mu        sync.Mutex
	checkouts map[string]string // form token → plan kodu
	subs      map[string]*Subscription
	calls     []string
	failNext  bool
}

func newFakeIyzico(t *testing.T) (*fakeIyzico, *Iyzico) {
	f := &fakeIyzico{t: t, apiKey: "api-anahtari", secret: "gizli-anahtar", checkouts: map[string]string{}, subs: map[string]*Subscription{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, &Iyzico{BaseURL: srv.URL, APIKey: f.apiKey, SecretKey: f.secret, MerchantID: "12345", HTTP: srv.Client()}
}

func (f *fakeIyzico) checkAuth(r *http.Request, body []byte) bool {
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
	mac := hmac.New(sha256.New, []byte(f.secret))
	mac.Write([]byte(parts["randomKey"] + r.URL.Path + string(body)))
	return parts["apiKey"] == f.apiKey && parts["randomKey"] == r.Header.Get("x-iyzi-rnd") &&
		hmac.Equal([]byte(parts["signature"]), []byte(hex.EncodeToString(mac.Sum(nil))))
}

func (f *fakeIyzico) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	reply := func(v map[string]any) {
		v["status"] = "success"
		json.NewEncoder(w).Encode(v)
	}
	if !f.checkAuth(r, body) {
		json.NewEncoder(w).Encode(map[string]any{"status": "failure", "errorCode": "1000", "errorMessage": "Geçersiz imza"})
		return
	}
	if f.failNext {
		f.failNext = false
		json.NewEncoder(w).Encode(map[string]any{"status": "failure", "errorCode": "5", "errorMessage": "hata"})
		return
	}
	p := r.URL.Path
	switch {
	case p == "/v2/subscription/checkoutform/initialize":
		var in struct {
			PricingPlanReferenceCode string   `json:"pricingPlanReferenceCode"`
			Customer                 Customer `json:"customer"`
		}
		json.Unmarshal(body, &in)
		if in.Customer.IdentityNumber == "" {
			json.NewEncoder(w).Encode(map[string]any{"status": "failure", "errorCode": "2", "errorMessage": "kimlik no"})
			return
		}
		tok := "form-" + in.PricingPlanReferenceCode
		f.checkouts[tok] = in.PricingPlanReferenceCode
		reply(map[string]any{"token": tok, "checkoutFormContent": `<script type="text/javascript">var iyziInit = {token:"` + tok + `"};</script>`})
	case strings.HasPrefix(p, "/v2/subscription/checkoutform/"):
		tok := strings.TrimPrefix(p, "/v2/subscription/checkoutform/")
		code := f.checkouts[tok]
		sub := &Subscription{ReferenceCode: "abonelik-1", CustomerReferenceCode: "musteri-1", PricingPlanReferenceCode: code, Status: "ACTIVE", EndDate: 1_800_000_000_000}
		f.subs[sub.ReferenceCode] = sub
		reply(map[string]any{"data": sub})
	case strings.HasSuffix(p, "/cancel"):
		reply(map[string]any{})
	case strings.HasSuffix(p, "/upgrade"):
		var in struct {
			NewPricingPlanReferenceCode string `json:"newPricingPlanReferenceCode"`
		}
		json.Unmarshal(body, &in)
		sub := &Subscription{ReferenceCode: "abonelik-2", CustomerReferenceCode: "musteri-1", PricingPlanReferenceCode: in.NewPricingPlanReferenceCode, Status: "ACTIVE", EndDate: 1_800_000_000_000}
		f.subs[sub.ReferenceCode] = sub
		reply(map[string]any{"data": sub})
	case strings.HasPrefix(p, "/v2/subscription/subscriptions/"):
		ref := strings.TrimPrefix(p, "/v2/subscription/subscriptions/")
		sub := f.subs[ref]
		if sub == nil {
			reply(map[string]any{"data": map[string]any{"items": []any{}}})
			return
		}
		cp := *sub
		cp.EndDate = 1_900_000_000_000 // yenilendi
		reply(map[string]any{"data": map[string]any{"items": []any{cp}}})
	default:
		http.NotFound(w, r)
	}
}
