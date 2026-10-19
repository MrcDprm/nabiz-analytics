// Package salt, çerezsiz saymanın çekirdeğidir: günlük tuzu tutar ve
// ziyaretçi kimliğini (visitor) bu tuzla üretir.
//
// Aynı kişi aynı gün aynı siteye gelirse aynı visitor değeri çıkar; ertesi gün
// tuz değiştiği için tamamen farklı bir değer çıkar. Eski tuz silindiği için
// dünkü bir değerin kime ait olduğu sonradan hesaplanamaz.
package salt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

const size = 32

// Keeper bugünün tuzunu bellekte tutar. Gün değişince ilk istekte yeni tuz
// üretilir, veritabanına yazılır ve eski günlerin tuzu silinir; bunun için ayrı
// bir zamanlayıcıya gerek yok.
type Keeper struct {
	db  *sql.DB
	now func() time.Time

	mu    sync.Mutex
	day   string
	value []byte
}

// New bugünün tuzunu veritabanından yükler, yoksa üretir. Sunucu gün içinde
// yeniden başlarsa aynı tuz kullanılır, yani sayımlar bölünmez.
func New(ctx context.Context, db *sql.DB, now func() time.Time) (*Keeper, error) {
	if now == nil {
		now = time.Now
	}
	k := &Keeper{db: db, now: now}
	if _, err := k.current(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

func today(t time.Time) string { return t.UTC().Format("2006-01-02") }

// current bugünün tuzunu döndürür, gerekirse yeniler.
func (k *Keeper) current(ctx context.Context) ([]byte, error) {
	day := today(k.now())
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.day == day {
		return k.value, nil
	}
	value, err := k.rotate(ctx, day)
	if err != nil {
		return nil, err
	}
	clear(k.value) // eski tuzu bellekte de sil
	k.day, k.value = day, value
	return value, nil
}

// rotate, verilen günün tuzunu getirir ya da üretir ve diğer bütün günlerin
// tuzunu siler. Hepsi tek işlemde: yarıda kalırsa ne yeni tuz yazılır ne eskisi silinir.
func (k *Keeper) rotate(ctx context.Context, day string) ([]byte, error) {
	tx, err := k.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var value []byte
	err = tx.QueryRowContext(ctx, `SELECT value FROM salts WHERE day = ?`, day).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		value = make([]byte, size)
		if _, err := rand.Read(value); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO salts (day, value) VALUES (?, ?)`, day, value); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM salts WHERE day <> ?`, day); err != nil {
		return nil, err
	}
	return value, tx.Commit()
}

// Visitor, bir ziyaretçinin bugünkü kimliğini üretir:
// SHA-256(tuz ‖ site ‖ IP ‖ User-Agent) çıktısının ilk 8 baytı.
// IP ve User-Agent burada kullanılıp atılır, hiçbir yere yazılmaz.
func (k *Keeper) Visitor(ctx context.Context, siteID, ip, userAgent string) (int64, error) {
	s, err := k.current(ctx)
	if err != nil {
		return 0, err
	}
	h := sha256.New()
	h.Write(s)
	for _, field := range []string{siteID, ip, userAgent} {
		// Her alanın önüne uzunluğunu yazıyoruz; yoksa ("ab", "c") ile ("a", "bc")
		// aynı baytları verir ve iki farklı ziyaretçi aynı kimliği alabilirdi.
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(field)))
		h.Write(n[:])
		h.Write([]byte(field))
	}
	sum := h.Sum(nil)
	// SQLite INTEGER işaretli 64 bit; uint64'ü olduğu gibi int64'e çeviriyoruz.
	return int64(binary.BigEndian.Uint64(sum[:8])), nil
}
