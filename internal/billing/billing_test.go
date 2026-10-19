package billing

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/billing/billingtest"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

var codes = PlanCodes{
	"starter/monthly": "plan-s-ay", "starter/yearly": "plan-s-yil",
	"growth/monthly": "plan-g-ay", "growth/yearly": "plan-g-yil",
}

var customer = Customer{Name: "Ali", Surname: "Yılmaz", Email: "ali@example.com", GsmNumber: "+905551112233", IdentityNumber: "11111111111",
	BillingAddress: Address{ContactName: "Ali Yılmaz", City: "İstanbul", Country: "Türkiye", Address: "Bir sokak 1"}}

type env struct {
	s    *Service
	st   *store.Store
	fake *billingtest.Fake
	now  *time.Time
}

func setup(t *testing.T) env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.W.Exec(`INSERT INTO users (id, email, password_hash, created_at) VALUES (1, 'ali@example.com', 'x', 0)`)
	fake := billingtest.New(t)
	iyz := &Iyzico{BaseURL: fake.URL, APIKey: billingtest.APIKey, SecretKey: billingtest.SecretKey, MerchantID: billingtest.MerchantID, HTTP: fake.Client}
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	return env{New(st, iyz, codes, func() time.Time { return now }), st, fake, &now}
}

func (e env) subscribe(t *testing.T, plan, interval string) {
	t.Helper()
	ctx := context.Background()
	tok, err := e.s.StartCheckout(ctx, 1, plan, interval, "https://n.example.com/api/billing/callback", "tr", customer)
	if err != nil {
		t.Fatal(err)
	}
	if c, err := e.s.CheckoutContent(ctx, tok, 1); err != nil || !strings.Contains(c, "iyziInit") {
		t.Fatalf("form = %q %v", c, err)
	}
	if uid, err := e.s.Complete(ctx, tok); err != nil || uid != 1 {
		t.Fatalf("tamamla: %d %v", uid, err)
	}
}

func TestFreeByDefault(t *testing.T) {
	e := setup(t)
	st, err := e.s.StatusOf(context.Background(), 1)
	if err != nil || st.Plan.ID != Free || st.State != "free" {
		t.Fatalf("durum = %+v %v", st, err)
	}
}

func TestCheckoutFlow(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.s.StartCheckout(ctx, 1, "free", "monthly", "x", "tr", customer); err != ErrUnknownPlan {
		t.Fatalf("ücretsiz paket satın alındı: %v", err)
	}
	if _, err := e.s.StartCheckout(ctx, 1, "starter", "haftalik", "x", "tr", customer); err != ErrUnknownPlan {
		t.Fatalf("geçersiz dönem: %v", err)
	}
	e.subscribe(t, "starter", "monthly")
	st, _ := e.s.StatusOf(ctx, 1)
	if st.Plan.ID != Starter || st.State != "active" || st.Interval != "monthly" || st.PeriodEnd == nil {
		t.Fatalf("durum = %+v", st)
	}
	// Ödeme formu bir kez kullanılır; aktif abonelik varken yenisi açılamaz.
	if _, err := e.s.StartCheckout(ctx, 1, "growth", "monthly", "x", "tr", customer); err != ErrAlreadyActive {
		t.Fatalf("ikinci abonelik: %v", err)
	}
	if _, err := e.s.Complete(ctx, "form-plan-s-ay"); err != ErrBadCheckout {
		t.Fatalf("aynı form iki kez: %v", err)
	}
}

func TestCheckoutContentOnlyForOwner(t *testing.T) {
	e := setup(t)
	tok, _ := e.s.StartCheckout(context.Background(), 1, "starter", "monthly", "x", "tr", customer)
	if _, err := e.s.CheckoutContent(context.Background(), tok, 2); err != ErrBadCheckout {
		t.Fatalf("başkasının formu: %v", err)
	}
	*e.now = e.now.Add(CheckoutLifetime + time.Minute)
	if _, err := e.s.CheckoutContent(context.Background(), tok, 1); err != ErrBadCheckout {
		t.Fatalf("süresi dolan form: %v", err)
	}
}

func TestIyzicoErrorSurfaces(t *testing.T) {
	e := setup(t)
	bad := customer
	bad.IdentityNumber = ""
	_, err := e.s.StartCheckout(context.Background(), 1, "starter", "monthly", "x", "tr", bad)
	if apiErr, ok := err.(*APIError); !ok || apiErr.Code != "2" {
		t.Fatalf("iyzico hatası = %v", err)
	}
}

func TestWrongSignatureRejected(t *testing.T) {
	e := setup(t)
	e.s.iyz.SecretKey = "yanlis"
	if _, err := e.s.StartCheckout(context.Background(), 1, "starter", "monthly", "x", "tr", customer); err == nil {
		t.Fatal("yanlış anahtarla imzalanan istek kabul edildi")
	}
}

func webhook(e env, kind, ref string) WebhookEvent {
	ev := WebhookEvent{OrderReferenceCode: "siparis-" + ref, CustomerReferenceCode: "musteri-1", SubscriptionReferenceCode: "abonelik-1",
		IyziReferenceCode: "olay-" + ref, IyziEventType: kind}
	return ev
}

func TestWebhooks(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.subscribe(t, "starter", "monthly")

	ev := webhook(e, "subscription.order.failure", "1")
	if err := e.s.HandleWebhook(ctx, ev, "uydurma-imza"); err != ErrBadSignature {
		t.Fatalf("imzasız webhook: %v", err)
	}
	if err := e.s.HandleWebhook(ctx, ev, strings.ToUpper(e.s.iyz.WebhookSignature(ev))); err != nil {
		t.Fatal(err)
	}
	st, _ := e.s.StatusOf(ctx, 1)
	if st.State != "unpaid" || st.Plan.ID != Starter {
		t.Fatalf("ödeme alınamadı ama durum = %+v", st)
	}

	ok := webhook(e, "subscription.order.success", "2")
	if err := e.s.HandleWebhook(ctx, ok, e.s.iyz.WebhookSignature(ok)); err != nil {
		t.Fatal(err)
	}
	e.s.forget(1)
	st, _ = e.s.StatusOf(ctx, 1)
	if st.State != "active" || st.PeriodEnd.UnixMilli() != billingtest.RenewedMs {
		t.Fatalf("başarılı çekim sonrası = %+v", st)
	}

	// Aynı bildirim tekrar gelirse işlenmez (durum değişmez).
	if err := e.s.HandleWebhook(ctx, ev, e.s.iyz.WebhookSignature(ev)); err != nil {
		t.Fatal(err)
	}
	e.s.forget(1)
	if st, _ := e.s.StatusOf(ctx, 1); st.State != "active" {
		t.Fatalf("tekrarlanan eski bildirim durumu bozdu: %+v", st)
	}
}

func TestUnpaidGraceEnds(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.subscribe(t, "growth", "monthly")
	ev := webhook(e, "subscription.order.failure", "1")
	e.s.HandleWebhook(ctx, ev, e.s.iyz.WebhookSignature(ev))
	*e.now = time.UnixMilli(billingtest.EndDateMs).Add(GracePeriod + time.Hour)
	e.s.forget(1)
	if st, _ := e.s.StatusOf(ctx, 1); st.Plan.ID != Free {
		t.Fatalf("ödemesiz süre bitti ama paket = %+v", st.Plan)
	}
}

func TestCancelKeepsPaidPeriod(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if err := e.s.Cancel(ctx, 1); err != ErrNoActive {
		t.Fatalf("aboneliksiz iptal: %v", err)
	}
	e.subscribe(t, "starter", "yearly")
	if err := e.s.Cancel(ctx, 1); err != nil {
		t.Fatal(err)
	}
	st, _ := e.s.StatusOf(ctx, 1)
	if st.State != "canceled" || st.Plan.ID != Starter {
		t.Fatalf("iptal sonrası dönem içinde = %+v", st)
	}
	*e.now = time.UnixMilli(billingtest.EndDateMs).Add(time.Hour)
	e.s.forget(1)
	if st, _ := e.s.StatusOf(ctx, 1); st.Plan.ID != Free {
		t.Fatalf("dönem bitti ama paket = %+v", st.Plan)
	}
	// İptal edilmiş abonelikten sonra yeniden abone olunabilir.
	e.subscribe(t, "growth", "yearly")
}

func TestChangePlan(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.subscribe(t, "starter", "monthly")
	if err := e.s.ChangePlan(ctx, 1, "growth", "yearly"); err != ErrSameInterval {
		t.Fatalf("dönem değişikliği: %v", err)
	}
	if err := e.s.ChangePlan(ctx, 1, "growth", "monthly"); err != nil {
		t.Fatal(err)
	}
	st, _ := e.s.StatusOf(ctx, 1)
	if st.Plan.ID != Growth {
		t.Fatalf("yükseltme sonrası = %+v", st.Plan)
	}
	var ref string
	e.st.R.QueryRow(`SELECT iyzico_ref FROM subscriptions WHERE user_id = 1`).Scan(&ref)
	if ref != "abonelik-2" { // ödeme abonelik-1'i, yükseltme abonelik-2'yi üretti
		t.Fatalf("yeni abonelik referansı kaydedilmedi: %q", ref)
	}
}

func TestLimits(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := e.st.W.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0), ('s2', 'b.com', 0)`)
	exec(`INSERT INTO memberships (site_id, user_id, role) VALUES ('s1', 1, 'owner'), ('s2', 1, 'owner')`)

	// Ücretsiz: 2 site sınırı, takım yok.
	if err := e.s.CanAddSite(ctx, 1); err != ErrSiteLimit {
		t.Fatalf("3. site: %v", err)
	}
	if err := e.s.CanInvite(ctx, "s1"); err != ErrMemberLimit {
		t.Fatalf("ücretsizde davet: %v", err)
	}
	e.subscribe(t, "starter", "monthly")
	if err := e.s.CanAddSite(ctx, 1); err != nil {
		t.Fatalf("başlangıçta 3. site: %v", err)
	}
	if err := e.s.CanInvite(ctx, "s1"); err != nil {
		t.Fatalf("başlangıçta davet: %v", err)
	}

	// Görüntüleme: geçen ay ve bu ay sınırın üstündeyse yeni site yok.
	exec(`INSERT INTO sessions (id, site_id, visitor, started_at, ended_at) VALUES (1, 's1', 1, 0, 0)`)
	insert := func(n int, at time.Time) {
		tx, _ := e.st.W.Begin()
		stmt, _ := tx.Prepare(`INSERT INTO events (site_id, ts, session_id, visitor, name, pathname) VALUES ('s1', ?, 1, 1, 'pageview', '/')`)
		for range n {
			stmt.Exec(at.Unix())
		}
		tx.Commit()
	}
	insert(100_001, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	insert(100_001, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	e.s.forget(1)
	st, _ := e.s.StatusOf(ctx, 1)
	if st.Usage.Pageviews != 100_001 || !st.Usage.LastMonthOver {
		t.Fatalf("kullanım = %+v", st.Usage)
	}
	if err := e.s.CanAddSite(ctx, 1); err != ErrOverUsage {
		t.Fatalf("iki ay aşım: %v", err)
	}
}

func TestPaymentsOff(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.W.Exec(`INSERT INTO users (id, email, password_hash, created_at) VALUES (1, 'ali@example.com', 'x', 0)`)
	st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0), ('s2', 'b.com', 0)`)
	st.W.Exec(`INSERT INTO memberships (site_id, user_id, role) VALUES ('s1', 1, 'owner'), ('s2', 1, 'owner')`)
	s := New(st, nil, nil, nil)
	ctx := context.Background()

	if s.PaymentsEnabled() {
		t.Fatal("iyzico yokken ödeme açık")
	}
	if status, _ := s.StatusOf(ctx, 1); status.Plan.ID != Free {
		t.Fatalf("paket = %+v", status.Plan)
	}
	// Sınırlar yine geçerli, ama mesaj "kendi sunucuna kur" diyor.
	if err := s.CanAddSite(ctx, 1); err != ErrSiteLimitFree {
		t.Fatalf("site sınırı: %v", err)
	}
	if err := s.CanInvite(ctx, "s1"); err != ErrMemberLimitFree {
		t.Fatalf("takım sınırı: %v", err)
	}
	if _, err := s.StartCheckout(ctx, 1, "starter", "monthly", "x", "tr", customer); err != ErrPaymentsOff {
		t.Fatalf("ödeme başladı: %v", err)
	}
	if err := s.HandleWebhook(ctx, WebhookEvent{IyziReferenceCode: "x"}, "imza"); err != ErrBadSignature {
		t.Fatalf("webhook: %v", err)
	}
	if err := s.OnAccountDelete(ctx, 1); err != nil {
		t.Fatalf("hesap silme: %v", err)
	}
}

func TestAdminUnlimited(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0), ('s2', 'b.com', 0)`)
	e.st.W.Exec(`INSERT INTO memberships (site_id, user_id, role) VALUES ('s1', 1, 'owner'), ('s2', 1, 'owner')`)
	if err := e.s.CanAddSite(ctx, 1); err != ErrSiteLimit {
		t.Fatalf("yönetici değilken: %v", err)
	}
	e.s.SetAdmins([]string{" ALI@example.com ", ""})
	e.s.forget(1)
	st, _ := e.s.StatusOf(ctx, 1)
	if st.Plan.ID != "unlimited" || st.State != "admin" {
		t.Fatalf("yönetici durumu = %+v", st)
	}
	if err := e.s.CanAddSite(ctx, 1); err != nil {
		t.Fatalf("yönetici site ekleyemedi: %v", err)
	}
	if err := e.s.CanInvite(ctx, "s1"); err != nil {
		t.Fatalf("yönetici davet edemedi: %v", err)
	}
}

func TestAccountDeleteCancels(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if err := e.s.OnAccountDelete(ctx, 1); err != nil {
		t.Fatalf("aboneliksiz hesap silme: %v", err)
	}
	e.subscribe(t, "starter", "monthly")
	if err := e.s.OnAccountDelete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if last := e.fake.LastCall(); last != "POST /v2/subscription/subscriptions/abonelik-1/cancel" {
		t.Fatalf("iptal çağrılmadı: %v", last)
	}
}
