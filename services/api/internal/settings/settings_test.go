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
