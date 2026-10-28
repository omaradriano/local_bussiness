package bookings

import "testing"

func TestF3_5_SaldoCuadraConTotalMenosPagos(t *testing.T) {
	cases := []struct {
		name     string
		total    int64
		payments []Payment
		want     Balance
	}{
		{"sin pagos", 25000, nil, Balance{Due: 25000}},
		{"anticipo y saldo", 25000, []Payment{{Type: "advance", Amount: 7500}, {Type: "balance", Amount: 17500}},
			Balance{Paid: 25000, Due: 0}},
		{"pago parcial", 65000, []Payment{{Type: "advance", Amount: 32500}}, Balance{Paid: 32500, Due: 32500}},
		{"depósito de garantía no cuenta para el total", 90000,
			[]Payment{{Type: "security_deposit", Amount: 100000}, {Type: "advance", Amount: 90000}},
			Balance{Paid: 90000, Due: 0, DepositHeld: 100000}},
		{"devolución del depósito", 90000,
			[]Payment{{Type: "security_deposit", Amount: 100000}, {Type: "balance", Amount: 90000}, {Type: "refund", Amount: 70000}},
			Balance{Paid: 90000, DepositHeld: 30000}},
		{"recargos por retraso y daño se suman como cobros extra", 90000,
			[]Payment{{Type: "balance", Amount: 90000}, {Type: "late_fee", Amount: 30000}, {Type: "damage_fee", Amount: 15000}},
			Balance{Paid: 90000, Extra: 45000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeBalance(tc.total, tc.payments); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestValidatePayment(t *testing.T) {
	bal := Balance{Paid: 7500, Due: 17500, DepositHeld: 50000}
	cases := []struct {
		name  string
		p     Payment
		field string // "" = valid
	}{
		{"saldo exacto", Payment{Type: "balance", Method: "cash", Amount: 17500}, ""},
		{"más de lo que se debe", Payment{Type: "balance", Method: "cash", Amount: 17501}, "amount"},
		{"anticipo mayor al pendiente", Payment{Type: "advance", Method: "transfer", Amount: 20000}, "amount"},
		{"monto en cero", Payment{Type: "balance", Method: "cash", Amount: 0}, "amount"},
		{"tipo inválido", Payment{Type: "propina", Method: "cash", Amount: 100}, "type"},
		{"método inválido", Payment{Type: "balance", Method: "cheque", Amount: 100}, "method"},
		{"devolver más depósito del retenido", Payment{Type: "refund", Method: "cash", Amount: 60000}, "amount"},
		{"devolución válida", Payment{Type: "refund", Method: "cash", Amount: 50000}, ""},
		{"depósito de garantía (sin tope)", Payment{Type: "security_deposit", Method: "card", Amount: 100000}, ""},
		{"recargo", Payment{Type: "late_fee", Method: "cash", Amount: 30000}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePayment(tc.p, bal)
			if tc.field == "" && err != nil {
				t.Fatalf("want valid, got %v", err)
			}
			if tc.field != "" {
				inv, ok := err.(Invalid)
				if !ok || inv.Field != tc.field {
					t.Fatalf("want error on %s, got %v", tc.field, err)
				}
			}
		})
	}
}
