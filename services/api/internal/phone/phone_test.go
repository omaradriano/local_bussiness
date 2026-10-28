package phone

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"614 123 4567", "+526141234567"},
		{"(614) 123-4567", "+526141234567"},
		{"+52 614 123 4567", "+526141234567"},
		{"526141234567", "+526141234567"},
		{"+52 614 123 456", ""},  // 9 digits after +52
		{"+52 6141234567 8", ""}, // 11 digits after +52
		{"+1 915 555 0100", "+19155550100"},
		{"12345", ""},
		{"", ""},
		{"abc", ""},
	}
	for _, tc := range cases {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
