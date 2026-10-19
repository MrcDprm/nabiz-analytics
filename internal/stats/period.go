// Package stats, panelin gösterdiği sayıları veritabanından hesaplar.
package stats

import (
	"errors"
	"time"
	_ "time/tzdata" // sunucuda saat dilimi dosyaları olmasa da Europe/Istanbul vb. çalışsın
)

// Interval, zaman grafiğindeki bir çubuğun genişliği.
type Interval string

const (
	Hour  Interval = "hour"
	Day   Interval = "day"
	Month Interval = "month"
)

// Range yarı açık bir aralıktır: From dahil, To hariç. Loc, ziyaretçinin değil
// paneli açan kişinin saat dilimi: "bugün" onun bugünü.
type Range struct {
	From, To time.Time
	Loc      *time.Location
	Interval Interval
	// Önceki dönem: kıyaslama için aynı uzunlukta, hemen öncesi.
	PrevFrom, PrevTo time.Time
}

var ErrBadPeriod = errors.New("geçersiz dönem")

const maxCustomDays = 366

// ParsePeriod panelin dönem seçimini somut bir aralığa çevirir.
// period: day | 7d | 30d | month | 12m | custom (from ve to: YYYY-AA-GG, ikisi de dahil).
func ParsePeriod(period, from, to, tz string, now time.Time) (Range, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" || tz == "Local" {
		loc = time.UTC
	}
	now = now.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	tomorrow := today.AddDate(0, 0, 1)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	r := Range{Loc: loc}
	switch period {
	case "day", "":
		r.From, r.To, r.Interval = today, tomorrow, Hour
		r.PrevFrom, r.PrevTo = today.AddDate(0, 0, -1), today
	case "7d":
		r.From, r.To, r.Interval = today.AddDate(0, 0, -6), tomorrow, Day
	case "30d":
		r.From, r.To, r.Interval = today.AddDate(0, 0, -29), tomorrow, Day
	case "month":
		r.From, r.To, r.Interval = thisMonth, thisMonth.AddDate(0, 1, 0), Day
		r.PrevFrom, r.PrevTo = thisMonth.AddDate(0, -1, 0), thisMonth
	case "12m":
		r.From, r.To, r.Interval = thisMonth.AddDate(0, -11, 0), thisMonth.AddDate(0, 1, 0), Month
		r.PrevFrom, r.PrevTo = r.From.AddDate(-1, 0, 0), r.From
	case "custom":
		f, err1 := time.ParseInLocation("2006-01-02", from, loc)
		t, err2 := time.ParseInLocation("2006-01-02", to, loc)
		if err1 != nil || err2 != nil || t.Before(f) {
			return r, ErrBadPeriod
		}
		t = t.AddDate(0, 0, 1) // bitiş günü dahil
		if t.Sub(f) > maxCustomDays*24*time.Hour+time.Hour {
			return r, ErrBadPeriod
		}
		r.From, r.To, r.Interval = f, t, Day
		if t.Sub(f) > 92*24*time.Hour {
			r.Interval = Month
		}
	default:
		return r, ErrBadPeriod
	}
	if r.PrevTo.IsZero() {
		days := int(r.To.Sub(r.From).Round(24*time.Hour) / (24 * time.Hour))
		r.PrevFrom, r.PrevTo = r.From.AddDate(0, 0, -days), r.From
	}
	return r, nil
}

// Buckets aralığı çubuklara böler. Sınırlar takvime göre hesaplanır (AddDate),
// bu yüzden yaz saati geçişinde de bir gün tam bir gündür.
func (r Range) Buckets() []time.Time {
	var out []time.Time
	for t := r.From; t.Before(r.To); t = r.next(t) {
		out = append(out, t)
	}
	return out
}

func (r Range) next(t time.Time) time.Time {
	switch r.Interval {
	case Hour:
		return t.Add(time.Hour)
	case Month:
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}
