package catalog

import (
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestValidateByKind(t *testing.T) {
	service := Item{Kind: "service", Name: "Gelish", Price: 25000, DurationMinutes: ptr(60)}
	product := Item{Kind: "product", Name: "Ramo", Price: 65000, PrepHours: ptr(24)}
	rental := Item{Kind: "rental", Name: "Rotomartillo", Price: 30000, PriceUnit: ptr("day"), SecurityDeposit: ptr(int64(100000)), LateFeePerDay: ptr(int64(30000))}

	cases := []struct {
		name      string
		item      Item
		mut       func(*Item)
		wantField string // "" = valid
	}{
		{"servicio válido", service, func(*Item) {}, ""},
		{"producto válido", product, func(*Item) {}, ""},
		{"renta válida", rental, func(*Item) {}, ""},
		{"tipo desconocido", service, func(i *Item) { i.Kind = "combo" }, "kind"},
		{"sin nombre", service, func(i *Item) { i.Name = " " }, "name"},
		{"nombre largo", service, func(i *Item) { i.Name = strings.Repeat("x", 121) }, "name"},
		{"precio negativo", service, func(i *Item) { i.Price = -1 }, "price"},
		{"precio absurdo", service, func(i *Item) { i.Price = 100_000_001 }, "price"},
		{"servicio sin duración", service, func(i *Item) { i.DurationMinutes = nil }, "duration_minutes"},
		{"servicio de 2 minutos", service, func(i *Item) { i.DurationMinutes = ptr(2) }, "duration_minutes"},
		{"margen negativo", service, func(i *Item) { i.BufferMinutes = -5 }, "buffer_minutes"},
		{"producto sin anticipación", product, func(i *Item) { i.PrepHours = nil }, "prep_hours"},
		{"cupo en cero", product, func(i *Item) { i.DailyCapacity = ptr(0) }, "daily_capacity"},
		{"renta sin unidad de precio", rental, func(i *Item) { i.PriceUnit = nil }, "price_unit"},
		{"renta por hora", rental, func(i *Item) { i.PriceUnit = ptr("hour") }, "price_unit"},
		{"renta sin depósito", rental, func(i *Item) { i.SecurityDeposit = nil }, "security_deposit"},
		{"renta sin recargo", rental, func(i *Item) { i.LateFeePerDay = nil }, "late_fee_per_day"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it := tc.item
			tc.mut(&it)
			errs := it.Validate()
			if tc.wantField == "" && len(errs) != 0 {
				t.Fatalf("want valid, got %v", errs)
			}
			if tc.wantField != "" {
				if _, ok := errs[tc.wantField]; !ok {
					t.Fatalf("want error on %q, got %v", tc.wantField, errs)
				}
			}
		})
	}
}

func TestNormalizeClearsFieldsOfOtherKinds(t *testing.T) {
	it := Item{Kind: "service", Name: "  Gelish ", DurationMinutes: ptr(60), PrepHours: ptr(24), PriceUnit: ptr("day")}.Normalize()
	if it.Name != "Gelish" || it.PrepHours != nil || it.PriceUnit != nil || it.DurationMinutes == nil {
		t.Fatalf("got %+v", it)
	}
}

func TestApplyPatchKeepsUntouchedFields(t *testing.T) {
	it := Item{Kind: "service", Name: "Gelish", Price: 25000, Active: true, DurationMinutes: ptr(60)}
	got := it.Apply(Patch{Price: ptr(int64(27000)), Active: ptr(false)})
	if got.Price != 27000 || got.Active || got.Name != "Gelish" || *got.DurationMinutes != 60 || got.Kind != "service" {
		t.Fatalf("got %+v", got)
	}
}
