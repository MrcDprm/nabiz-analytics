package stats

import (
	"context"
	"testing"
)

func step(kind, value string) GoalDef { return GoalDef{Kind: kind, Value: value} }

func TestFunnel(t *testing.T) {
	st := fixture(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		steps []GoalDef
		want  []int64
	}{
		// z1: / → /projects; z2: / + İndir; z3: /blog + Kayıt (z1 ile aynı ziyaretçi, ayrı ziyaret)
		{"sıra doğru", []GoalDef{step("page", "/"), step("page", "/projects")}, []int64{2, 1}},
		{"sıra ters sayılmaz", []GoalDef{step("page", "/projects"), step("page", "/")}, []int64{1, 0}},
		{"olay adımı", []GoalDef{step("page", "/"), step("event", "İndir")}, []int64{2, 1}},
		{"farklı ziyaretler birleşmez", []GoalDef{step("page", "/projects"), step("event", "Kayıt")}, []int64{1, 0}},
		{"joker", []GoalDef{step("page", "/b*"), step("event", "Kayıt")}, []int64{1, 1}},
	}
	for _, c := range cases {
		res, err := GetFunnel(ctx, st.R, today(t), c.steps)
		if err != nil {
			t.Fatal(err)
		}
		for i, w := range c.want {
			if res[i].Visitors != w {
				t.Errorf("%s: adım %d = %d, beklenen %d", c.name, i+1, res[i].Visitors, w)
			}
		}
	}
	res, _ := GetFunnel(ctx, st.R, today(t), cases[0].steps)
	if !near(res[1].Rate, 50) || !near(res[1].Drop, 50) || !near(res[0].Rate, 100) {
		t.Fatalf("oranlar = %+v", res)
	}
}

func TestProps(t *testing.T) {
	st := fixture(t)
	st.W.Exec(`INSERT INTO event_props (event_id, site_id, key, value)
		SELECT id, site_id, 'dosya', 'cv-tr.pdf' FROM events WHERE name = 'İndir'`)
	st.W.Exec(`INSERT INTO event_props (event_id, site_id, key, value)
		SELECT id, site_id, 'dil', 'tr' FROM events WHERE name = 'İndir'`)
	keys, err := PropKeys(context.Background(), st.R, today(t), "İndir")
	if err != nil || len(keys) != 2 {
		t.Fatalf("anahtarlar = %+v %v", keys, err)
	}
	vals, err := PropValues(context.Background(), st.R, today(t), "İndir", "dosya", 10)
	if err != nil || len(vals) != 1 || vals[0] != (Row{"cv-tr.pdf", 1, 1}) {
		t.Fatalf("değerler = %+v %v", vals, err)
	}
	// Başka olayın özellikleri karışmaz.
	if k, _ := PropKeys(context.Background(), st.R, today(t), "Kayıt"); len(k) != 0 {
		t.Fatalf("Kayıt özellikleri = %+v", k)
	}
}
