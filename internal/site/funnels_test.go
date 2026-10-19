package site

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

func TestFunnelsCRUD(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.W.Exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0), ('s2', 'b.com', 0)`)
	ctx := context.Background()

	good := []Step{{"page", "/projects"}, {"page", "/projects/*"}, {"event", "İletişim formu"}}
	f, err := AddFunnel(ctx, st.W, "s1", " Projeden iletişime ", good)
	if err != nil || f.Name != "Projeden iletişime" {
		t.Fatalf("ekle: %+v %v", f, err)
	}
	for _, bad := range [][]Step{
		{{"page", "/tek"}},
		{{"page", "/a"}, {"page", "yolsuz"}},
		{{"page", "/a"}, {"link", "/b"}},
	} {
		if _, err := AddFunnel(ctx, st.W, "s1", "x", bad); err != ErrInvalidFunnel {
			t.Errorf("%v: %v", bad, err)
		}
	}
	if _, err := AddFunnel(ctx, st.W, "s1", "", good); err != ErrInvalidFunnel {
		t.Errorf("adsız huni: %v", err)
	}

	all, _ := Funnels(ctx, st.R, "s1")
	if len(all) != 1 || len(all[0].Steps) != 3 || all[0].Steps[2].Value != "İletişim formu" {
		t.Fatalf("liste = %+v", all)
	}
	if ok, _ := DeleteFunnel(ctx, st.W, "s2", f.ID); ok {
		t.Fatal("başka sitenin hunisi silindi")
	}
	if ok, _ := DeleteFunnel(ctx, st.W, "s1", f.ID); !ok {
		t.Fatal("huni silinmedi")
	}
	var n int
	st.R.QueryRow(`SELECT count(*) FROM funnel_steps`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d adım kaldı", n)
	}
}
