package finances

import "testing"

func d(s string) Day {
	v, ok := ParseDay(s)
	if !ok {
		panic(s)
	}
	return v
}

func TestPlanInstallments(t *testing.T) {
	// A truck: 50,000 down on 15 Jan, then 24 monthly payments of 8,000 from 31 Jan.
	truck := Plan{PurchasedOn: d("2026-01-15"), DownPayment: 5_000_000, InstallmentAmount: 800_000, Installments: 24, FirstDue: d("2026-01-31")}
	if got := truck.Due(1); !got.Equal(d("2026-02-28")) {
		t.Fatalf("31 Jan + 1 month = %v, want 28 Feb", got)
	}
	if got := truck.Due(2); !got.Equal(d("2026-03-31")) {
		t.Fatalf("keeps the 31st when the month has it: %v", got)
	}
	cases := []struct {
		name     string
		from, to string
		want     int64
	}{
		{"January: down payment + first installment", "2026-01-01", "2026-01-31", 5_000_000 + 800_000},
		{"February: one installment", "2026-02-01", "2026-02-28", 800_000},
		{"whole 2026: down payment + 12 installments", "2026-01-01", "2026-12-31", 5_000_000 + 12*800_000},
		{"after the last installment", "2028-02-01", "2028-12-31", 0},
		{"before the purchase", "2025-01-01", "2025-12-31", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truck.InRange(d(tc.from), d(tc.to)); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
	paid, remaining, next := truck.Progress(d("2026-03-10"))
	if paid != 2 || remaining != 22*800_000 || next == nil || !next.Equal(d("2026-03-31")) {
		t.Fatalf("progress: %d %d %v", paid, remaining, next)
	}
	if paid, remaining, next := truck.Progress(d("2030-01-01")); paid != 24 || remaining != 0 || next != nil {
		t.Fatalf("finished: %d %d %v", paid, remaining, next)
	}
}

func TestProratedSalary(t *testing.T) {
	today := d("2026-11-15")
	// 12,000 a month ≈ 394.52 a day: 15 days so far in November.
	if got := ProratedSalary(1_200_000, "monthly", d("2026-11-01"), d("2026-11-30"), today); got != 591781 {
		t.Fatalf("monthly, half of November: %d", got)
	}
	// 2,000 a week for a whole year = 104,000.
	if got := ProratedSalary(200_000, "weekly", d("2025-01-01"), d("2025-12-31"), today); got != 10_400_000 {
		t.Fatalf("weekly, a year: %d", got)
	}
	if got := ProratedSalary(1_200_000, "monthly", d("2026-12-01"), d("2026-12-31"), today); got != 0 {
		t.Fatalf("future period: %d", got)
	}
	if got := Commission(35_000, 40); got != 14_000 {
		t.Fatalf("40%% of 350: %d", got)
	}
}
