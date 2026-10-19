package site

import "testing"

func TestAtLeast(t *testing.T) {
	cases := []struct {
		role, want string
		ok         bool
	}{
		{"owner", "admin", true}, {"admin", "admin", true}, {"viewer", "admin", false},
		{"admin", "owner", false}, {"", "viewer", false}, {"hacker", "viewer", false},
	}
	for _, c := range cases {
		if AtLeast(c.role, c.want) != c.ok {
			t.Errorf("AtLeast(%q, %q) != %v", c.role, c.want, c.ok)
		}
	}
}

func TestValidGoal(t *testing.T) {
	good := [][2]string{{"page", "/iletisim"}, {"page", "/blog/*"}, {"event", "CV indir"}}
	bad := [][2]string{
		{"page", "iletisim"}, {"page", "/a*b"}, {"page", "/*/*"}, {"event", ""},
		{"event", "a\x00b"}, {"link", "/x"}, {"event", string(make([]byte, 65))},
	}
	for _, g := range good {
		if !validGoal(g[0], g[1]) {
			t.Errorf("%v reddedildi", g)
		}
	}
	for _, g := range bad {
		if validGoal(g[0], g[1]) {
			t.Errorf("%q kabul edildi", g)
		}
	}
}
