// Package billing, barındırılan hizmetin (cloud modu) paketlerini, kullanım
// limitlerini ve iyzico aboneliğini yönetir. Kendi sunucunda kurulumda (selfhost)
// hiç kullanılmaz: orada paket ve limit yoktur.
package billing

// Plan bir paketin limitleri ve fiyatı. Fiyatlar kuruş cinsinden (kayan nokta
// yuvarlama hatası olmasın diye tamsayı).
type Plan struct {
	ID           string `json:"id"`
	Pageviews    int64  `json:"pageviews"` // aylık görüntüleme
	Sites        int    `json:"sites"`
	Members      int    `json:"members"` // site başına, sahip dahil
	MonthlyKurus int64  `json:"monthly_kurus"`
	YearlyKurus  int64  `json:"yearly_kurus"` // yıllıkta 2 ay ücretsiz
}

const (
	Free    = "free"
	Starter = "starter"
	Growth  = "growth"
)

// Plans, sıralı (ucuzdan pahalıya). Fiyatlar taslak; yayından önce kesinleşecek.
var Plans = []Plan{
	{ID: Free, Pageviews: 10_000, Sites: 2, Members: 1},
	{ID: Starter, Pageviews: 100_000, Sites: 10, Members: 3, MonthlyKurus: 14_900, YearlyKurus: 149_000},
	{ID: Growth, Pageviews: 1_000_000, Sites: 50, Members: 10, MonthlyKurus: 39_900, YearlyKurus: 399_000},
}

func PlanByID(id string) (Plan, bool) {
	for _, p := range Plans {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// Paid: ücretli paket mi (satın alınabilir mi)?
func Paid(id string) bool { return id == Starter || id == Growth }

func ValidInterval(i string) bool { return i == "monthly" || i == "yearly" }
