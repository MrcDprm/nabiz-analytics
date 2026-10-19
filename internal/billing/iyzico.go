package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Iyzico, iyzico abonelik API'sinin bize gereken kısmı.
// Belgeler: docs.iyzico.com → Abonelik, HMACSHA256 Kimlik Doğrulama, Webhook.
type Iyzico struct {
	BaseURL    string // sandbox: https://sandbox-api.iyzipay.com, canlı: https://api.iyzipay.com
	APIKey     string
	SecretKey  string
	MerchantID string // webhook imzasında kullanılır
	HTTP       *http.Client
}

const SandboxURL = "https://sandbox-api.iyzipay.com"

// authHeader, IYZWSv2 imzasını üretir:
//
//	imza   = hex(HMAC-SHA256(gizli anahtar, rastgele + yol + gövde))
//	başlık = "IYZWSv2 " + base64("apiKey:" + anahtar + "&randomKey:" + rastgele + "&signature:" + imza)
func (c *Iyzico) authHeader(randomKey, path string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(randomKey + path))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	plain := "apiKey:" + c.APIKey + "&randomKey:" + randomKey + "&signature:" + sig
	return "IYZWSv2 " + base64.StdEncoding.EncodeToString([]byte(plain))
}

func randomKey() string {
	b := make([]byte, 8)
	rand.Read(b)
	return strconv.FormatInt(time.Now().UnixMilli(), 10) + hex.EncodeToString(b)
}

// APIError, iyzico'nun "failure" cevabı. Mesaj kullanıcıya gösterilmez, log'a gider.
type APIError struct {
	Code, Message string
}

func (e *APIError) Error() string { return "iyzico " + e.Code + ": " + e.Message }

type envelope struct {
	Status       string          `json:"status"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	Data         json.RawMessage `json:"data"`
	// Ödeme formu başlatma cevabında alanlar üst düzeyde gelir.
	Token               string `json:"token"`
	CheckoutFormContent string `json:"checkoutFormContent"`
}

func (c *Iyzico) do(ctx context.Context, method, path string, in any) (*envelope, error) {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	rnd := randomKey()
	req.Header.Set("Authorization", c.authHeader(rnd, path, body))
	req.Header.Set("x-iyzi-rnd", rnd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("iyzico: HTTP %d, okunamayan cevap", resp.StatusCode)
	}
	if env.Status != "success" {
		return nil, &APIError{Code: env.ErrorCode, Message: env.ErrorMessage}
	}
	return &env, nil
}

// Customer: iyzico'nun ödeme formu için istediği alıcı bilgisi. Biz saklamayız,
// sadece iyzico'ya iletiriz.
type Customer struct {
	Name           string  `json:"name"`
	Surname        string  `json:"surname"`
	Email          string  `json:"email"`
	GsmNumber      string  `json:"gsmNumber"`
	IdentityNumber string  `json:"identityNumber"`
	BillingAddress Address `json:"billingAddress"`
}

type Address struct {
	ContactName string `json:"contactName"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Address     string `json:"address"`
}

// InitCheckout ödeme formunu başlatır; form token'ı ve formu açan betiği döndürür.
func (c *Iyzico) InitCheckout(ctx context.Context, planCode, callbackURL, locale string, cust Customer) (token, content string, err error) {
	env, err := c.do(ctx, http.MethodPost, "/v2/subscription/checkoutform/initialize", map[string]any{
		"locale":                    locale,
		"callbackUrl":               callbackURL,
		"pricingPlanReferenceCode":  planCode,
		"subscriptionInitialStatus": "ACTIVE",
		"customer":                  cust,
	})
	if err != nil {
		return "", "", err
	}
	if env.Token == "" || env.CheckoutFormContent == "" {
		return "", "", errors.New("iyzico: ödeme formu boş döndü")
	}
	return env.Token, env.CheckoutFormContent, nil
}

// Subscription, iyzico'nun bir abonelik hakkında söyledikleri.
type Subscription struct {
	ReferenceCode            string `json:"referenceCode"`
	CustomerReferenceCode    string `json:"customerReferenceCode"`
	PricingPlanReferenceCode string `json:"pricingPlanReferenceCode"`
	Status                   string `json:"subscriptionStatus"`
	EndDate                  int64  `json:"endDate"` // Unix milisaniye
}

// RetrieveCheckout, ödeme formu tamamlandıktan sonra sonucu sorar.
func (c *Iyzico) RetrieveCheckout(ctx context.Context, token string) (*Subscription, error) {
	env, err := c.do(ctx, http.MethodGet, "/v2/subscription/checkoutform/"+token, nil)
	if err != nil {
		return nil, err
	}
	var s Subscription
	if err := json.Unmarshal(env.Data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSubscription aboneliğin güncel durumunu sorar (webhook'tan sonra dönem sonunu öğrenmek için).
func (c *Iyzico) GetSubscription(ctx context.Context, ref string) (*Subscription, error) {
	env, err := c.do(ctx, http.MethodGet, "/v2/subscription/subscriptions/"+ref, nil)
	if err != nil {
		return nil, err
	}
	var page struct {
		Items []Subscription `json:"items"`
	}
	if err := json.Unmarshal(env.Data, &page); err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, errors.New("iyzico: abonelik bulunamadı")
	}
	return &page.Items[0], nil
}

func (c *Iyzico) Cancel(ctx context.Context, ref string) error {
	_, err := c.do(ctx, http.MethodPost, "/v2/subscription/subscriptions/"+ref+"/cancel", map[string]string{})
	return err
}

// Upgrade aboneliği başka pakete taşır (yükseltme ya da düşürme; dönem aynı kalmalı).
// iyzico yeni bir abonelik referansı üretir.
func (c *Iyzico) Upgrade(ctx context.Context, ref, newPlanCode string) (*Subscription, error) {
	env, err := c.do(ctx, http.MethodPost, "/v2/subscription/subscriptions/"+ref+"/upgrade", map[string]any{
		"newPricingPlanReferenceCode": newPlanCode,
		"upgradePeriod":               "NOW",
		"useTrial":                    false,
		"resetRecurrenceCount":        false,
	})
	if err != nil {
		return nil, err
	}
	var s Subscription
	if err := json.Unmarshal(env.Data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// WebhookEvent, abonelik bildiriminin gövdesi.
type WebhookEvent struct {
	OrderReferenceCode        string `json:"orderReferenceCode"`
	CustomerReferenceCode     string `json:"customerReferenceCode"`
	SubscriptionReferenceCode string `json:"subscriptionReferenceCode"`
	IyziReferenceCode         string `json:"iyziReferenceCode"`
	IyziEventType             string `json:"iyziEventType"` // subscription.order.success | subscription.order.failure
	IyziEventTime             int64  `json:"iyziEventTime"`
}

// WebhookSignature, X-IYZ-SIGNATURE-V3 değerini hesaplar:
// hex(HMAC-SHA256(gizli anahtar, merchantId + gizliAnahtar + eventType + abonelikRef + siparişRef + müşteriRef)).
func (c *Iyzico) WebhookSignature(e WebhookEvent) string {
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(c.MerchantID + c.SecretKey + e.IyziEventType + e.SubscriptionReferenceCode + e.OrderReferenceCode + e.CustomerReferenceCode))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhook imzayı sabit sürede karşılaştırır.
func (c *Iyzico) VerifyWebhook(e WebhookEvent, header string) bool {
	want := c.WebhookSignature(e)
	return header != "" && hmac.Equal([]byte(want), []byte(strings.ToLower(header)))
}
