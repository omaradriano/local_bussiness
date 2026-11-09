package bookings

import "testing"

func TestFoldQuery(t *testing.T) {
	cases := map[string]string{"": "", "  ": "", " Valentín ": "%valentin%", "PEÑA": "%pena%", "50%": `%50\%%`, "a_b": `%a\_b%`}
	for in, want := range cases {
		if got := foldQuery(in); got != want {
			t.Errorf("foldQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
