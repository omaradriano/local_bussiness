package bookings

import (
	"slices"
	"time"
)

// Payment is one money movement of a booking. Amounts are always positive; the type gives the meaning.
type Payment struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`   // advance | balance | security_deposit | refund | late_fee | damage_fee
	Method     string    `json:"method"` // cash | transfer | card
	Amount     int64     `json:"amount"`
	PaidAt     time.Time `json:"paid_at"`
	RecordedBy *string   `json:"recorded_by"`
	Notes      *string   `json:"notes"`
}

// Balance of a booking:
//   - Paid (advance + balance) goes toward the total; Due = total − Paid (never negative).
//   - DepositHeld = security deposits − refunds (refundable, separate from the total).
//   - Extra = late and damage fees collected on top.
type Balance struct {
	Paid        int64 `json:"paid"`
	Due         int64 `json:"due"`
	DepositHeld int64 `json:"deposit_held"`
	Extra       int64 `json:"extra_charges"`
}

func ComputeBalance(total int64, payments []Payment) Balance {
	var b Balance
	for _, p := range payments {
		switch p.Type {
		case "advance", "balance":
			b.Paid += p.Amount
		case "security_deposit":
			b.DepositHeld += p.Amount
		case "refund":
			b.DepositHeld -= p.Amount
		case "late_fee", "damage_fee":
			b.Extra += p.Amount
		}
	}
	b.Due = max(total-b.Paid, 0)
	return b
}

var (
	paymentTypes   = []string{"advance", "balance", "security_deposit", "refund", "late_fee", "damage_fee"}
	paymentMethods = []string{"cash", "transfer", "card"}
)

const maxPayment = 100_000_000 // $1,000,000.00

// ValidatePayment checks a new payment against the current balance: no paying more than is due,
// no refunding more deposit than is held.
func ValidatePayment(p Payment, b Balance) error {
	switch {
	case !slices.Contains(paymentTypes, p.Type):
		return Invalid{"type", "Tipo de pago inválido."}
	case !slices.Contains(paymentMethods, p.Method):
		return Invalid{"method", "Elige efectivo, transferencia o tarjeta."}
	case p.Amount <= 0 || p.Amount > maxPayment:
		return Invalid{"amount", "Escribe un monto mayor a cero."}
	case (p.Type == "advance" || p.Type == "balance") && p.Amount > b.Due:
		return Invalid{"amount", "El monto es mayor que el saldo pendiente."}
	case p.Type == "refund" && p.Amount > b.DepositHeld:
		return Invalid{"amount", "No se puede devolver más del depósito retenido."}
	}
	return nil
}
