// Package phone normalizes phone numbers to E.164 (needed for WhatsApp links).
package phone

import (
	"strings"
	"unicode"
)

// Normalize returns E.164 or "" if it can't be a phone. A bare 10-digit number is Mexican (+52).
// A Mexican number must have exactly 10 digits after +52.
func Normalize(in string) string {
	plus := strings.HasPrefix(strings.TrimSpace(in), "+")
	var b strings.Builder
	for _, r := range in {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case len(d) == 10 && !plus:
		return "+52" + d
	case strings.HasPrefix(d, "52") && len(d) != 12:
		return "" // +52 with the wrong number of digits
	case len(d) >= 11 && len(d) <= 15 && d[0] != '0':
		return "+" + d
	default:
		return ""
	}
}
