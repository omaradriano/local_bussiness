// Package settings reads and updates the business policies kept in businesses.settings (jsonb),
// with defaults for anything not set yet.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// Rentals are the owner's rental rules.
type Rentals struct {
	SecurityDepositEnabled bool    `json:"security_deposit_enabled"` // ask for a refundable deposit
	LateFeeMultiplier      float64 `json:"late_fee_multiplier"`      // a late day costs the daily price × this
	SpanClosedDays         bool    `json:"rentals_span_closed_days"` // a rental may include days the business is closed
	ChargeClosedDays       bool    `json:"charge_closed_days"`       // those days are charged
	InspectionMinutes      int     `json:"inspection_minutes"`       // check-up gap between two rentals of a unit
}

func DefaultRentals() Rentals {
	return Rentals{SecurityDepositEnabled: true, LateFeeMultiplier: 1, SpanClosedDays: true, ChargeClosedDays: true}
}

// RentalsFrom reads the rental rules from the settings jsonb, keeping defaults for missing keys.
func RentalsFrom(raw []byte) Rentals {
	r := DefaultRentals()
	_ = json.Unmarshal(raw, &r)
	return r
}

// RentalsPatch: nil = unchanged.
type RentalsPatch struct {
	SecurityDepositEnabled *bool    `json:"security_deposit_enabled"`
	LateFeeMultiplier      *float64 `json:"late_fee_multiplier"`
	SpanClosedDays         *bool    `json:"rentals_span_closed_days"`
	ChargeClosedDays       *bool    `json:"charge_closed_days"`
	InspectionMinutes      *int     `json:"inspection_minutes"`
}

func (r Rentals) Apply(p RentalsPatch) Rentals {
	if p.SecurityDepositEnabled != nil {
		r.SecurityDepositEnabled = *p.SecurityDepositEnabled
	}
	if p.LateFeeMultiplier != nil {
		r.LateFeeMultiplier = math.Round(*p.LateFeeMultiplier*100) / 100
	}
	if p.SpanClosedDays != nil {
		r.SpanClosedDays = *p.SpanClosedDays
	}
	if p.ChargeClosedDays != nil {
		r.ChargeClosedDays = *p.ChargeClosedDays
	}
	if p.InspectionMinutes != nil {
		r.InspectionMinutes = *p.InspectionMinutes
	}
	return r
}

// Validate returns field -> message (es-MX).
func (r Rentals) Validate() map[string]string {
	errs := map[string]string{}
	if r.LateFeeMultiplier < 0 || r.LateFeeMultiplier > 10 {
		errs["late_fee_multiplier"] = "El multiplicador debe estar entre 0 y 10 (por ejemplo 1.5 o 2)."
	}
	if r.InspectionMinutes < 0 || r.InspectionMinutes > 7*24*60 {
		errs["inspection_minutes"] = "La revisión entre rentas debe ser de 0 a 168 horas."
	}
	return errs
}

// LateFeePerDay is what one late day costs for a daily price, rounded to whole centavos.
func (r Rentals) LateFeePerDay(dailyPrice int64) int64 {
	return int64(math.Round(float64(dailyPrice) * r.LateFeeMultiplier))
}

var ErrNotFound = errors.New("business not found")

type Repo struct{ DB store.DB }

func (s Repo) Rentals(ctx context.Context, businessID string) (Rentals, error) {
	var raw []byte
	err := s.DB.QueryRow(ctx, `select settings from public.businesses where id = $1`, businessID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Rentals{}, ErrNotFound
	}
	return RentalsFrom(raw), err
}

// SaveRentals merges the rental keys into settings (other keys are left as they are).
func (s Repo) SaveRentals(ctx context.Context, businessID string, r Rentals) error {
	raw, _ := json.Marshal(r)
	_, err := s.DB.Exec(ctx, `update public.businesses set settings = settings || $2::jsonb where id = $1`, businessID, raw)
	return err
}
