package settings

import "testing"

func ptr[T any](v T) *T { return &v }

func TestRentalsDefaultsWhenUnset(t *testing.T) {
	r := RentalsFrom([]byte(`{"deposit_percent": 0, "hold_hours": 24}`))
	if r != DefaultRentals() {
		t.Fatalf("got %+v", r)
	}
	r = RentalsFrom([]byte(`{"security_deposit_enabled": false, "late_fee_multiplier": 2, "inspection_minutes": 60}`))
	if r.SecurityDepositEnabled || r.LateFeeMultiplier != 2 || !r.SpanClosedDays || r.InspectionMinutes != 60 {
		t.Fatalf("partial keys: %+v", r)
	}
}

func TestRentalsPatchAndValidate(t *testing.T) {
	r := DefaultRentals().Apply(RentalsPatch{LateFeeMultiplier: ptr(1.499), ChargeClosedDays: ptr(false)})
	if r.LateFeeMultiplier != 1.5 || r.ChargeClosedDays || !r.SecurityDepositEnabled {
		t.Fatalf("patch: %+v", r)
	}
	if errs := DefaultRentals().Apply(RentalsPatch{LateFeeMultiplier: ptr(11.0)}).Validate(); errs["late_fee_multiplier"] == "" {
		t.Fatal("multiplier 11 accepted")
	}
	if errs := DefaultRentals().Apply(RentalsPatch{InspectionMinutes: ptr(-5)}).Validate(); errs["inspection_minutes"] == "" {
		t.Fatal("negative inspection accepted")
	}
}

func TestLateFeePerDay(t *testing.T) {
	cases := []struct {
		mult  float64
		price int64
		want  int64
	}{
		{1, 30000, 30000},
		{2, 30000, 60000}, // the first client charges double per late day
		{1.5, 33333, 50000},
		{0, 30000, 0},
	}
	for _, tc := range cases {
		r := DefaultRentals()
		r.LateFeeMultiplier = tc.mult
		if got := r.LateFeePerDay(tc.price); got != tc.want {
			t.Errorf("×%v of %d = %d, want %d", tc.mult, tc.price, got, tc.want)
		}
	}
}

func TestDocumentTypeInput(t *testing.T) {
	d := DocumentType{Name: "Pagaré", Required: true}.Apply(DocumentTypeInput{Name: ptr("  Contrato   de  renta "), Active: ptr(true)})
	if d.Name != "Contrato de renta" || !d.Active || !d.Required || d.Validate() != nil {
		t.Fatalf("apply: %+v %v", d, d.Validate())
	}
	if (DocumentType{}).Apply(DocumentTypeInput{Name: ptr("   ")}).Validate()["name"] == "" {
		t.Fatal("blank name accepted")
	}
}

func TestSettingsSectionsDefaultsAndPatch(t *testing.T) {
	s := From([]byte(`{"deposit_percent": 30, "hold_hours": 4, "late_fee_multiplier": 2}`))
	if s.General.DepositPercent != 30 || s.General.HoldHours != 4 || s.Rentals.LateFeeMultiplier != 2 || s.Delivery.Zones == nil {
		t.Fatalf("from: %+v", s)
	}
	if d := From([]byte(`{}`)); d.General.HoldHours != 24 || d.General.DepositPercent != 0 {
		t.Fatalf("defaults: %+v", d.General)
	}
	zones := []Zone{{Name: " Centro ", Fee: 5000}, {Name: "Norte", Fee: 8000}}
	next, errs := s.Apply(Patch{General: &GeneralPatch{DepositPercent: ptr(50)}, Delivery: &DeliveryPatch{Zones: &zones}})
	if len(errs) != 0 || next.General.DepositPercent != 50 || next.General.HoldHours != 4 || next.Delivery.Zones[0].Name != "Centro" {
		t.Fatalf("apply: %+v %v", next, errs)
	}
	// Only the touched sections are validated.
	cases := []struct {
		name  string
		patch Patch
		field string
	}{
		{"anticipo > 100", Patch{General: &GeneralPatch{DepositPercent: ptr(150)}}, "deposit_percent"},
		{"apartado de 0 h", Patch{General: &GeneralPatch{HoldHours: ptr(0)}}, "hold_hours"},
		{"zona sin nombre", Patch{Delivery: &DeliveryPatch{Zones: &[]Zone{{Name: " ", Fee: 1}}}}, "delivery_zones"},
		{"zonas repetidas", Patch{Delivery: &DeliveryPatch{Zones: &[]Zone{{Name: "Centro"}, {Name: "centro"}}}}, "delivery_zones"},
		{"envío negativo", Patch{Delivery: &DeliveryPatch{Zones: &[]Zone{{Name: "Sur", Fee: -1}}}}, "delivery_zones"},
		{"multiplicador 11", Patch{Rentals: &RentalsPatch{LateFeeMultiplier: ptr(11.0)}}, "late_fee_multiplier"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, errs := s.Apply(tc.patch); errs[tc.field] == "" {
				t.Fatalf("want error on %s, got %v", tc.field, errs)
			}
		})
	}
}
