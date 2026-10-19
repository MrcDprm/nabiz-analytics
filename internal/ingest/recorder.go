package ingest

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/geo"
	"github.com/MrcDprm/nabiz-analytics/internal/salt"
	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

// SessionTimeout: aynı ziyaretçi bu süreden uzun ara verirse yeni ziyaret sayılır.
const SessionTimeout = 30 * time.Minute

// siteTTL: kayıtlı site listesi bu kadar süre önbellekte durur. Yeni eklenen
// site en geç bu süre sonra sayılmaya başlar.
const siteTTL = 30 * time.Second

var ErrUnknownSite = errors.New("kayıtlı olmayan site")

type sessionKey struct {
	site    string
	visitor int64
}

type activeSession struct {
	id   int64
	last time.Time
}

type siteEntry struct {
	id      string // boşsa site kayıtlı değil (olumsuz sonuç da önbelleğe alınır)
	expires time.Time
}

// Recorder, doğrulanmış olayları veritabanına yazar ve açık ziyaretleri
// bellekte izler. Bellekteki harita sadece "bu ziyaretçinin şu anki ziyareti
// hangisi" sorusunu cevaplar; IP ya da tarayıcı bilgisi tutmaz.
type Recorder struct {
	st   *store.Store
	salt *salt.Keeper
	geo  *geo.Locator
	now  func() time.Time

	mu     sync.Mutex
	active map[sessionKey]activeSession
	sites  map[string]siteEntry
}

func NewRecorder(st *store.Store, k *salt.Keeper, g *geo.Locator, now func() time.Time) *Recorder {
	if now == nil {
		now = time.Now
	}
	return &Recorder{
		st: st, salt: k, geo: g, now: now,
		active: make(map[sessionKey]activeSession),
		sites:  make(map[string]siteEntry),
	}
}

// Record bir olayı kaydeder: ziyaretçi kimliği, ülke, tarayıcı ve OS burada
// bulunur; ip ve userAgent bu fonksiyondan dışarı çıkmaz.
func (r *Recorder) Record(ctx context.Context, e *Event, ip, userAgent string) error {
	siteID, err := r.siteID(ctx, e.Site)
	if err != nil {
		return err
	}
	visitor, err := r.salt.Visitor(ctx, siteID, ip, userAgent)
	if err != nil {
		return err
	}
	browser, osName := ParseUA(userAgent)
	country := r.geo.Country(ip)
	now := r.now()
	ts := now.Unix()
	pageview := e.Name == Pageview

	key := sessionKey{siteID, visitor}
	// Aynı ziyaretçinin iki isteği aynı anda gelirse iki ayrı ziyaret açılmasın
	// diye oturum seçimi ve yazma kilit altında. Yazmalar zaten tek bağlantıdan
	// geçtiği için bu kilit ek bir yavaşlık getirmiyor.
	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.st.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	sessionID := int64(0)
	if s, ok := r.active[key]; ok && now.Sub(s.last) < SessionTimeout {
		res, err := tx.ExecContext(ctx, `UPDATE sessions SET ended_at = ?, pageviews = pageviews + ?,
			exit_page = CASE WHEN ? THEN ? ELSE exit_page END WHERE id = ?`,
			ts, boolInt(pageview), pageview, e.Path, s.id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			sessionID = s.id
		}
		// n == 0: site bu arada silinmiş olabilir; aşağıda yeni ziyaret açılır.
	}
	if sessionID == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO sessions (site_id, visitor, started_at, ended_at, pageviews,
			entry_page, exit_page, referrer, utm_source, utm_medium, utm_campaign, country, device, browser, os)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			siteID, visitor, ts, ts, boolInt(pageview), e.Path, e.Path, e.Referrer,
			e.UTMSource, e.UTMMedium, e.UTMCampaign, country, e.Device, browser, osName)
		if err != nil {
			return err
		}
		if sessionID, err = res.LastInsertId(); err != nil {
			return err
		}
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO events (site_id, ts, session_id, name, pathname) VALUES (?, ?, ?, ?, ?)`,
		siteID, ts, sessionID, e.Name, e.Path)
	if err != nil {
		return err
	}
	eventID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for k, v := range e.Props {
		if _, err := tx.ExecContext(ctx, `INSERT INTO event_props (event_id, site_id, key, value) VALUES (?, ?, ?, ?)`,
			eventID, siteID, k, v); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.active[key] = activeSession{id: sessionID, last: now}
	return nil
}

// siteID, alan adının kayıtlı olup olmadığını önbellekten ya da veritabanından bulur.
func (r *Recorder) siteID(ctx context.Context, domain string) (string, error) {
	now := r.now()
	r.mu.Lock()
	entry, ok := r.sites[domain]
	r.mu.Unlock()
	if !ok || now.After(entry.expires) {
		var id string
		err := r.st.R.QueryRowContext(ctx, `SELECT id FROM sites WHERE domain = ?`, domain).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		entry = siteEntry{id: id, expires: now.Add(siteTTL)}
		r.mu.Lock()
		r.sites[domain] = entry
		r.mu.Unlock()
	}
	if entry.id == "" {
		return "", ErrUnknownSite
	}
	return entry.id, nil
}

// Sweep süresi dolmuş ziyaretleri ve site önbelleğini bellekten temizler.
func (r *Recorder) Sweep() {
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, s := range r.active {
		if now.Sub(s.last) >= SessionTimeout {
			delete(r.active, k)
		}
	}
	for d, e := range r.sites {
		if now.After(e.expires) {
			delete(r.sites, d)
		}
	}
}

// Run, Sweep'i dakikada bir çalıştırır; ctx bitince durur.
func (r *Recorder) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Sweep()
		}
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
