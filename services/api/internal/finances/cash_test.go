package finances

import (
	"reflect"
	"testing"
)

func TestCashLines(t *testing.T) {
	got := cashLines([]cashRow{
		{"cash", "orders", 50000}, {"cash", "sales", 12000}, {"cash", "refunds", 10000}, {"cash", "deposits", 100000},
		{"card", "orders", 35000},
		{"transfer", "refunds", 2000}, // given back by transfer, nothing came in by it
	})
	want := []CashLine{
		{Method: "cash", Orders: 50000, Sales: 12000, Deposits: 100000, Refunds: 10000, Net: 152000},
		{Method: "transfer", Refunds: 2000, Net: -2000},
		{Method: "card", Orders: 35000, Net: 35000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if lines := cashLines(nil); len(lines) != 0 {
		t.Fatalf("an empty day has no lines: %+v", lines)
	}
}

func TestValidateClosing(t *testing.T) {
	today := d("2026-11-10")
	if err := validateClosing(d("2026-11-10"), 150000, "", today); err != nil {
		t.Fatalf("valid: %v", err)
	}
	for name, c := range map[string]struct {
		day     Day
		counted int64
		note    string
	}{
		"negativo":     {today, -1, ""},
		"día a futuro": {d("2026-11-11"), 0, ""},
		"nota larga":   {today, 0, string(make([]rune, 201))},
	} {
		if err := validateClosing(c.day, c.counted, c.note, today); err == nil {
			t.Errorf("%s: should fail", name)
		}
	}
}
