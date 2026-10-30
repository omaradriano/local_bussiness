package bookings

import "testing"

func TestDiscountValidate(t *testing.T) {
	cases := []struct {
		d  Discount
		ok bool
	}{
		{Discount{}, true}, // none
		{Discount{Type: "amount", Value: 5000}, true},
		{Discount{Type: "percent", Value: 15}, true},
		{Discount{Type: "percent", Value: 100}, true},
		{Discount{Type: "percent", Value: 101}, false},
		{Discount{Type: "amount", Value: -1}, false},
		{Discount{Type: "gift", Value: 10}, false},
		{Discount{Type: "amount", Value: maxDiscount + 1}, false},
	}
	for _, tc := range cases {
		if err := tc.d.Validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: err %v, want ok=%v", tc.d, err, tc.ok)
		}
	}
}

func TestDiscountOf(t *testing.T) {
	cases := []struct {
		d     Discount
		total int64
		want  int64
	}{
		{Discount{}, 50000, 0},
		{Discount{Type: "amount", Value: 5000}, 50000, 5000},
		{Discount{Type: "amount", Value: 90000}, 50000, 50000}, // never more than the total
		{Discount{Type: "percent", Value: 10}, 50000, 5000},
		{Discount{Type: "percent", Value: 15}, 33333, 5000}, // 4999.95 rounded to whole centavos
		{Discount{Type: "percent", Value: 100}, 50000, 50000},
	}
	for _, tc := range cases {
		if got := tc.d.Of(tc.total); got != tc.want {
			t.Errorf("%+v of %d = %d, want %d", tc.d, tc.total, got, tc.want)
		}
	}
}

// The discount comes off the total (with delivery), the advance is a percent of what's left and
// the security deposit isn't discounted.
func TestComputeWithDiscount(t *testing.T) {
	q := Compute(QuoteInput{UnitPrice: 30000, Quantity: 1, Periods: 2, DeliveryFee: 10000, DepositPercent: 50, SecurityDeposit: 100000,
		Discount: Discount{Type: "percent", Value: 10}})
	if q.Subtotal != 60000 || q.Discount != 7000 || q.Total != 63000 || q.Advance != 31500 || q.SecurityDeposit != 100000 {
		t.Fatalf("%+v", q)
	}
}

// Changing the discount of an existing booking keeps the advance in the same proportion.
func TestRescaleAdvance(t *testing.T) {
	if got := rescaleAdvance(15000, 50000, 45000); got != 13500 {
		t.Fatalf("30%% of 45000 = %d", got)
	}
	if got := rescaleAdvance(0, 50000, 45000); got != 0 {
		t.Fatalf("no advance: %d", got)
	}
	if got := rescaleAdvance(15000, 0, 0); got != 0 {
		t.Fatalf("zero base: %d", got)
	}
}
