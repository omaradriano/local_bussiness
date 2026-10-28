package catalog

import "testing"

func TestUnitCode(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want string
	}{
		{"Rotomartillo", 1, "ROT-01"},
		{"Revolvedora de concreto", 2, "REV-02"},
		{"Ñandú eléctrico", 3, "NAN-03"},
		{"Andamio (sección)", 12, "AND-12"},
		{"12 V", 1, "V-01"},
		{"123", 1, "U-01"},
	}
	for _, tc := range cases {
		if got := UnitCode(tc.name, tc.n); got != tc.want {
			t.Errorf("UnitCode(%q, %d) = %q, want %q", tc.name, tc.n, got, tc.want)
		}
	}
}
