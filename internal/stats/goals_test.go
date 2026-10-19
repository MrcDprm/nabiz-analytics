package stats

import (
	"context"
	"testing"
)

func TestGoals(t *testing.T) {
	st := fixture(t)
	// fixture: bugün 2 ziyaretçi; z1 /projects'i gördü, z2 "İndir" olayı gönderdi,
	// z3 (ziyaretçi 1) /blog'a girdi ve "Kayıt" gönderdi.
	goals := []GoalDef{
		{"g1", "page", "/projects"},
		{"g2", "event", "İndir"},
		{"g3", "page", "/b*"},  // /blog
		{"g4", "page", "/yok"}, // hiç ulaşılmadı
		{"g5", "page", "/%"},   // LIKE karakteri kaçışlanmalı: hiçbir sayfa eşleşmez
	}
	res, err := GetGoals(context.Background(), st.R, today(t), goals)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][3]float64{"g1": {1, 1, 50}, "g2": {1, 1, 50}, "g3": {1, 1, 50}, "g4": {0, 0, 0}, "g5": {0, 0, 0}}
	for _, r := range res {
		w := want[r.ID]
		if float64(r.Visitors) != w[0] || float64(r.Count) != w[1] || !near(r.Rate, w[2]) {
			t.Errorf("%s = %+v, beklenen %v", r.ID, r, w)
		}
	}
	// Ülke filtresiyle: sadece TR ziyaretçi (ziyaretçi 1) kalır; İndir'i DE'li gönderdi.
	res, _ = GetGoals(context.Background(), st.R, today(t, Filter{"country", "TR"}), goals[:2])
	if res[0].Visitors != 1 || !near(res[0].Rate, 100) || res[1].Visitors != 0 || res[1].Count != 0 {
		t.Fatalf("filtreli = %+v", res)
	}
}
