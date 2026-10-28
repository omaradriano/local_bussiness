package catalog

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// UnitCode suggests a unit code from the item name: "Revolvedora de concreto", 2 -> "REV-02".
func UnitCode(itemName string, n int) string {
	plain, _, _ := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), itemName)
	var prefix strings.Builder
	for _, r := range strings.ToUpper(plain) {
		if r >= 'A' && r <= 'Z' {
			prefix.WriteRune(r)
			if prefix.Len() == 3 {
				break
			}
		}
	}
	if prefix.Len() == 0 {
		prefix.WriteString("U")
	}
	return fmt.Sprintf("%s-%02d", prefix.String(), n)
}
