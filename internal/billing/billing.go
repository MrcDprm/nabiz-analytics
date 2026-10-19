package billing

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

// GracePeriod: ödeme alınamadığında (UNPAID) paketin bu kadar daha açık kalması;
// kullanıcı kartını güncelleyebilsin.
const GracePeriod = 7 * 24 * time.Hour

var (
	ErrUnknownPlan   = errors.New("geçersiz paket ya da dönem")
	ErrSiteLimit     = errors.New("paketinin site sınırına ulaştın; daha fazla site için paketini yükselt")
	ErrMemberLimit   = errors.New("paketinin takım sınırına ulaştın; daha fazla üye için paketini yükselt")
	ErrOverUsage     = errors.New("iki aydır görüntüleme sınırını aşıyorsun; yeni site eklemek için paketini yükselt")
	ErrNoActive      = errors.New("aktif bir aboneliğin yok")
	ErrAlreadyActive = errors.New("zaten aktif bir aboneliğin var; paket değiştirmeyi kullan")
	ErrSameInterval  = errors.New("dönem değiştirmek için aboneliği iptal edip yeniden başlat")
	ErrBadCheckout   = errors.New("ödeme bulunamadı ya da süresi dolmuş")
	ErrBadSignature  = errors.New("webhook imzası geçersiz")
)

// PlanCodes, iyzico panelinde oluşturulan fiyatlandırma planlarının referans kodları.
// Anahtar: "starter/monthly", "growth/yearly" gibi.
type PlanCodes map[string]string

func (pc PlanCodes) lookup(code string) (plan, interval string, ok bool) {
	for k, v := range pc {
		if v == code {
			plan, interval, _ = strings.Cut(k, "/")
			return plan, interval, true
		}
	}
	return "", "", false
}

// Service ödeme ve limitleri yönetir.
type Service struct {
	st    *store.Store
	iyz   *Iyzico
	codes PlanCodes
	now   func() time.Time

	mu    sync.Mutex
	cache map[int64]usageEntry // kullanıcı → kullanım (10 dakika)
}

type usageEntry struct {
	u   Usage
	exp time.Time
}

func New(st *store.Store, iyz *Iyzico, codes PlanCodes, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{st: st, iyz: iyz, codes: codes, now: now, cache: make(map[int64]usageEntry)}
}

// Status, kullanıcının paket durumu (plan sayfası gösterir).
type Status struct {
	Plan      Plan       `json:"plan"`
	Interval  string     `json:"interval,omitempty"`
	State     string     `json:"state"` // free | active | canceled | unpaid
	PeriodEnd *time.Time `json:"period_end,omitempty"`
	Usage     Usage      `json:"usage"`
}

// Usage: bu ay sahibi olunan sitelerdeki görüntüleme, site sayısı ve geçen ay aşım.
type Usage struct {
	Pageviews     int64 `json:"pageviews"`
	Sites         int   `json:"sites"`
	LastMonthOver bool  `json:"last_month_over"`
}

type subRow struct {
	plan, interval, status, ref, customer string
	periodEnd                             time.Time
}

func (s *Service) sub(ctx context.Context, userID int64) (*subRow, error) {
	var r subRow
	var end int64
	err := s.st.R.QueryRowContext(ctx, `SELECT plan, interval, status, iyzico_ref, customer_ref, current_period_end
		FROM subscriptions WHERE user_id = ?`, userID).Scan(&r.plan, &r.interval, &r.status, &r.ref, &r.customer, &end)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	r.periodEnd = time.Unix(end, 0).UTC()
	return &r, err
}

// effective: aboneliğe göre bugün geçerli paket ve durum.
//   - ACTIVE: ücretli paket.
//   - CANCELED: ödenmiş dönem bitene kadar ücretli paket, sonra ücretsiz.
//   - UNPAID: dönem sonu + 7 gün ödemesiz devam, sonra ücretsiz.
func (s *Service) effective(r *subRow) (Plan, string) {
	free, _ := PlanByID(Free)
	if r == nil {
		return free, "free"
	}
	p, ok := PlanByID(r.plan)
	if !ok {
		return free, "free"
	}
	now := s.now()
	switch r.status {
	case "ACTIVE":
		return p, "active"
	case "CANCELED":
		if now.Before(r.periodEnd) {
			return p, "canceled"
		}
	case "UNPAID":
		if now.Before(r.periodEnd.Add(GracePeriod)) {
			return p, "unpaid"
		}
	}
	return free, "free"
}

func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (s *Service) pageviews(ctx context.Context, userID int64, from, to time.Time) (int64, error) {
	var n int64
	err := s.st.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM events e
		WHERE e.site_id IN (SELECT site_id FROM memberships WHERE user_id = ? AND role = 'owner')
		AND e.ts >= ? AND e.ts < ? AND e.name = 'pageview'`, userID, from.Unix(), to.Unix()).Scan(&n)
	return n, err
}

// UsageOf kullanımı hesaplar; sorgu ağır olabileceği için 10 dakika önbellekte tutar.
func (s *Service) UsageOf(ctx context.Context, userID int64, plan Plan) (Usage, error) {
	now := s.now()
	s.mu.Lock()
	if e, ok := s.cache[userID]; ok && now.Before(e.exp) {
		s.mu.Unlock()
		return e.u, nil
	}
	s.mu.Unlock()

	var u Usage
	this := monthStart(now)
	var err error
	if u.Pageviews, err = s.pageviews(ctx, userID, this, now.Add(time.Second)); err != nil {
		return u, err
	}
	last, err := s.pageviews(ctx, userID, this.AddDate(0, -1, 0), this)
	if err != nil {
		return u, err
	}
	u.LastMonthOver = last > plan.Pageviews
	if err := s.st.R.QueryRowContext(ctx, `SELECT count(*) FROM memberships WHERE user_id = ? AND role = 'owner'`, userID).Scan(&u.Sites); err != nil {
		return u, err
	}
	s.mu.Lock()
	s.cache[userID] = usageEntry{u, now.Add(10 * time.Minute)}
	s.mu.Unlock()
	return u, nil
}

func (s *Service) forget(userID int64) {
	s.mu.Lock()
	delete(s.cache, userID)
	s.mu.Unlock()
}

func (s *Service) StatusOf(ctx context.Context, userID int64) (*Status, error) {
	r, err := s.sub(ctx, userID)
	if err != nil {
		return nil, err
	}
	p, state := s.effective(r)
	st := &Status{Plan: p, State: state}
	if r != nil && state != "free" {
		st.Interval = r.interval
		end := r.periodEnd
		st.PeriodEnd = &end
	}
	if st.Usage, err = s.UsageOf(ctx, userID, p); err != nil {
		return nil, err
	}
	return st, nil
}

// CanAddSite: paketin site sınırı ve iki aylık görüntüleme aşımı.
func (s *Service) CanAddSite(ctx context.Context, userID int64) error {
	st, err := s.StatusOf(ctx, userID)
	if err != nil {
		return err
	}
	s.forget(userID) // site sayısı değişecek
	if st.Usage.Sites >= st.Plan.Sites {
		return ErrSiteLimit
	}
	if st.Usage.LastMonthOver && st.Usage.Pageviews > st.Plan.Pageviews {
		return ErrOverUsage
	}
	return nil
}

// CanInvite: sitenin sahibinin paketine göre üye + bekleyen davet sayısı.
func (s *Service) CanInvite(ctx context.Context, siteID string) error {
	var owner int64
	if err := s.st.R.QueryRowContext(ctx, `SELECT user_id FROM memberships WHERE site_id = ? AND role = 'owner'`, siteID).Scan(&owner); err != nil {
		return err
	}
	r, err := s.sub(ctx, owner)
	if err != nil {
		return err
	}
	p, _ := s.effective(r)
	var n int
	if err := s.st.R.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM memberships WHERE site_id = ?)
		+ (SELECT count(*) FROM invites WHERE site_id = ? AND expires_at > ?)`, siteID, siteID, s.now().Unix()).Scan(&n); err != nil {
		return err
	}
	if n >= p.Members {
		return ErrMemberLimit
	}
	return nil
}
