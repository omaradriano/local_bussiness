package bookings

import (
	"errors"
	"testing"
)

func TestValidatePartialRefund(t *testing.T) {
	ok := PartialRefundInput{Amount: 20000, Method: "cash", Reason: "Rosas maltratadas"}
	if err := ok.Validate(75000, 75000); err != nil {
		t.Fatalf("valid refund: %v", err)
	}
	cases := []struct {
		name  string
		edit  func(*PartialRefundInput)
		paid  int64
		field string
	}{
		{"sin motivo", func(r *PartialRefundInput) { r.Reason = "  " }, 75000, "reason"},
		{"en cero", func(r *PartialRefundInput) { r.Amount = 0 }, 75000, "amount"},
		{"más de lo cobrado", func(r *PartialRefundInput) { r.Amount = 40000 }, 37500, "amount"},
		{"más que el total", func(r *PartialRefundInput) { r.Amount = 80000 }, 90000, "amount"},
		{"método inválido", func(r *PartialRefundInput) { r.Method = "bitcoin" }, 75000, "method"},
	}
	for _, tc := range cases {
		r := ok
		tc.edit(&r)
		var inv Invalid
		if err := r.Validate(75000, tc.paid); err == nil || !errors.As(err, &inv) || inv.Field != tc.field {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}
