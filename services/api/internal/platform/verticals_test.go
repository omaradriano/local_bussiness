package platform

import "testing"

func TestCanCreate(t *testing.T) {
	cases := []struct {
		status string
		admin  bool
		want   bool
	}{
		{Public, false, true},
		{Public, true, true},
		{AdminOnly, false, false},
		{AdminOnly, true, true}, // the platform admin tries a giro out, even in production
		{Off, true, false},
		{"", true, false},
	}
	for _, c := range cases {
		if got := CanCreate(c.status, c.admin); got != c.want {
			t.Errorf("CanCreate(%q, admin=%v) = %v", c.status, c.admin, got)
		}
	}
}

func TestAvailableFor(t *testing.T) {
	all := []Vertical{{"barbershop", Public}, {"flowers", AdminOnly}, {"nails", Off}, {"tool_rental", Public}}
	if got := AvailableFor(all, false); len(got) != 2 || got[0].Vertical != "barbershop" || got[1].Vertical != "tool_rental" {
		t.Fatalf("customer: %+v", got)
	}
	if got := AvailableFor(all, true); len(got) != 3 || got[1] != (Vertical{"flowers", AdminOnly}) {
		t.Fatalf("admin: %+v", got)
	}
}

func TestStatusesDefault(t *testing.T) {
	// A giro without a row (just added) starts admin-only: tried out before customers see it.
	got := withDefaults([]string{"barbershop", "nails"}, map[string]string{"barbershop": Public})
	if got[0] != (Vertical{"barbershop", Public}) || got[1] != (Vertical{"nails", AdminOnly}) {
		t.Fatalf("got %+v", got)
	}
}
