package bookings

import (
	"errors"
	"testing"
)

func cash(n int64) []Payment { return []Payment{{Method: "cash", Amount: n}} }

func TestValidatePartialRefund(t *testing.T) {
	ok := PartialRefundInput{Amount: 20000, Method: "cash", Reason: "Rosas maltratadas"}
	if err := ok.Validate(75000, []Payment{{Method: "cash", Amount: 75000}}); err != nil {
		t.Fatalf("valid refund: %v", err)
	}
	cases := []struct {
		name  string
		edit  func(*PartialRefundInput)
		paid  []Payment
		field string
	}{
		{"sin motivo", func(r *PartialRefundInput) { r.Reason = "  " }, cash(75000), "reason"},
		{"en cero", func(r *PartialRefundInput) { r.Amount = 0 }, cash(75000), "amount"},
		{"más de lo cobrado", func(r *PartialRefundInput) { r.Amount = 40000 }, cash(37500), "amount"},
		{"más que el total", func(r *PartialRefundInput) { r.Amount = 80000 }, cash(90000), "amount"},
		{"método sin cobro", func(r *PartialRefundInput) { r.Method = "card" }, cash(75000), "method"},
		{"método inválido", func(r *PartialRefundInput) { r.Method = "bitcoin" }, cash(75000), "method"},
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
