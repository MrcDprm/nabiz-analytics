package team

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/MrcDprm/nabiz-analytics/internal/store"
)

type fixture struct {
	t       *Team
	st      *store.Store
	now     *time.Time
	owner   int64
	visitor int64 // site üyesi olmayan, hesabı olan biri
}

func setup(t *testing.T) fixture {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nabiz.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := st.W.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users (id, email, password_hash, created_at) VALUES (1, 'sahip@example.com', 'x', 0), (2, 'veli@example.com', 'x', 0)`)
	exec(`INSERT INTO sites (id, domain, created_at) VALUES ('s1', 'a.com', 0)`)
	exec(`INSERT INTO memberships (site_id, user_id, role) VALUES ('s1', 1, 'owner')`)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	return fixture{New(st, func() time.Time { return now }), st, &now, 1, 2}
}

func TestInviteAndAccept(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.t.CreateInvite(ctx, "s1", "veli@example.com", "owner", 1); err != ErrBadRole {
		t.Fatalf("sahip olarak davet: %v", err)
	}
	if _, err := f.t.CreateInvite(ctx, "s1", "sahip@example.com", "viewer", 1); err != ErrAlreadyIn {
		t.Fatalf("üyeyi davet: %v", err)
	}
	old, _ := f.t.CreateInvite(ctx, "s1", "veli@example.com", "viewer", 1)
	token, err := f.t.CreateInvite(ctx, "s1", "veli@example.com", "admin", 1) // eskisinin yerini alır
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.t.Info(ctx, old); err != ErrBadInvite {
		t.Fatalf("eski davet hâlâ geçerli: %v", err)
	}
	info, err := f.t.Info(ctx, token)
	if err != nil || info.Domain != "a.com" || info.Role != "admin" || !info.HasAccount {
		t.Fatalf("bilgi = %+v %v", info, err)
	}
	if _, err := f.t.Accept(ctx, token, 1, "sahip@example.com"); err != ErrWrongAccount {
		t.Fatalf("başka hesapla kabul: %v", err)
	}
	if _, err := f.t.Accept(ctx, token, 2, "veli@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.t.Accept(ctx, token, 2, "veli@example.com"); err != ErrBadInvite {
		t.Fatalf("davet iki kez kullanıldı: %v", err)
	}
	members, invites, _ := f.t.Members(ctx, "s1")
	if len(members) != 2 || members[1] != (Member{"veli@example.com", "admin"}) || len(invites) != 0 {
		t.Fatalf("üyeler = %+v, davetler = %+v", members, invites)
	}
}

func TestInviteExpires(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	token, _ := f.t.CreateInvite(ctx, "s1", "yeni@example.com", "viewer", 1)
	if info, _ := f.t.Info(ctx, token); info.HasAccount {
		t.Fatal("hesabı olmayan için HasAccount true")
	}
	*f.now = f.now.Add(InviteLifetime + time.Minute)
	if _, err := f.t.Info(ctx, token); err != ErrBadInvite {
		t.Fatalf("süresi dolan davet: %v", err)
	}
	f.t.Sweep(ctx)
	var n int
	f.st.R.QueryRow(`SELECT count(*) FROM invites`).Scan(&n)
	if n != 0 {
		t.Fatal("süresi dolan davet silinmedi")
	}
}

func TestRolesRemoveTransfer(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.st.W.Exec(`INSERT INTO memberships (site_id, user_id, role) VALUES ('s1', 2, 'viewer')`)

	if err := f.t.SetRole(ctx, "s1", "sahip@example.com", "viewer"); err != ErrOwner {
		t.Fatalf("sahibin rolü: %v", err)
	}
	if err := f.t.SetRole(ctx, "s1", "yok@example.com", "admin"); err != ErrNotMember {
		t.Fatalf("üye olmayan: %v", err)
	}
	if err := f.t.SetRole(ctx, "s1", "veli@example.com", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := f.t.Remove(ctx, "s1", "sahip@example.com"); err != ErrOwner {
		t.Fatalf("sahibi çıkar: %v", err)
	}
	if err := f.t.Transfer(ctx, "s1", 1, "sahip@example.com"); err != ErrSelf {
		t.Fatalf("kendine devir: %v", err)
	}
	if err := f.t.Transfer(ctx, "s1", 1, "veli@example.com"); err != nil {
		t.Fatal(err)
	}
	members, _, _ := f.t.Members(ctx, "s1")
	if members[0] != (Member{"veli@example.com", "owner"}) || members[1] != (Member{"sahip@example.com", "admin"}) {
		t.Fatalf("devirden sonra = %+v", members)
	}
	// Eski sahip artık yönetici; ayrılabilir.
	if err := f.t.Remove(ctx, "s1", "sahip@example.com"); err != nil {
		t.Fatal(err)
	}
	if members, _, _ := f.t.Members(ctx, "s1"); len(members) != 1 {
		t.Fatalf("ayrıldıktan sonra = %+v", members)
	}
}
