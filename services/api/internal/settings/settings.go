// Package settings reads and updates the business policies kept in businesses.settings (jsonb),
// with defaults for anything not set yet.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"strings"
	"unicode/utf8"

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

// General: rules every business has (an advance and how long a hold waits for it).
type General struct {
	DepositPercent int `json:"deposit_percent"` // advance asked when booking, % of the total (0 = none)
	HoldHours      int `json:"hold_hours"`      // an unpaid hold expires after this
}

func DefaultGeneral() General { return General{DepositPercent: 0, HoldHours: 24} }

type GeneralPatch struct {
	DepositPercent *int `json:"deposit_percent"`
	HoldHours      *int `json:"hold_hours"`
}

func (g General) Apply(p GeneralPatch) General {
	if p.DepositPercent != nil {
		g.DepositPercent = *p.DepositPercent
	}
	if p.HoldHours != nil {
		g.HoldHours = *p.HoldHours
	}
	return g
}

func (g General) Validate() map[string]string {
	errs := map[string]string{}
	if g.DepositPercent < 0 || g.DepositPercent > 100 {
		errs["deposit_percent"] = "El anticipo debe ser de 0 a 100 %."
	}
	if g.HoldHours < 1 || g.HoldHours > 168 {
		errs["hold_hours"] = "El apartado debe durar de 1 a 168 horas."
	}
	return errs
}

// Zone: a delivery area and what delivering there costs (centavos).
type Zone struct {
	Name string `json:"name"`
	Fee  int64  `json:"fee"`
}

// Delivery: zones a flower shop delivers to; an empty list = delivery anywhere without a fee.
type Delivery struct {
	Zones []Zone `json:"delivery_zones"`
}

type DeliveryPatch struct {
	Zones *[]Zone `json:"delivery_zones"` // replaces the whole list
}

func (d Delivery) Apply(p DeliveryPatch) Delivery {
	if p.Zones != nil {
		d.Zones = make([]Zone, len(*p.Zones))
		for i, z := range *p.Zones {
			d.Zones[i] = Zone{Name: strings.Join(strings.Fields(z.Name), " "), Fee: z.Fee}
		}
	}
	return d
}

func (d Delivery) Validate() map[string]string {
	if len(d.Zones) > 30 {
		return map[string]string{"delivery_zones": "Máximo 30 zonas."}
	}
	seen := map[string]bool{}
	for _, z := range d.Zones {
		key := strings.ToLower(z.Name)
		switch n := utf8.RuneCountInString(z.Name); {
		case n == 0 || n > 40:
			return map[string]string{"delivery_zones": "Cada zona necesita un nombre (máximo 40 caracteres)."}
		case seen[key]:
			return map[string]string{"delivery_zones": "Hay dos zonas con el mismo nombre: " + z.Name + "."}
		case z.Fee < 0 || z.Fee > 10_000_000:
			return map[string]string{"delivery_zones": "El costo de envío de " + z.Name + " no es válido."}
		}
		seen[key] = true
	}
	return nil
}

// Settings: every section; the app shows the ones that apply to the type of business.
type Settings struct {
	General  General  `json:"general"`
	Rentals  Rentals  `json:"rentals"`
	Delivery Delivery `json:"delivery"`
}

type Patch struct {
	General  *GeneralPatch  `json:"general"`
	Rentals  *RentalsPatch  `json:"rentals"`
	Delivery *DeliveryPatch `json:"delivery"`
}

// From reads every section from the settings jsonb, with defaults for missing keys.
func From(raw []byte) Settings {
	s := Settings{General: DefaultGeneral(), Rentals: RentalsFrom(raw)}
	_ = json.Unmarshal(raw, &s.General)
	_ = json.Unmarshal(raw, &s.Delivery)
	if s.Delivery.Zones == nil {
		s.Delivery.Zones = []Zone{}
	}
	return s
}

// Apply merges the patch and validates the sections it touches.
func (s Settings) Apply(p Patch) (Settings, map[string]string) {
	errs := map[string]string{}
	if p.General != nil {
		s.General = s.General.Apply(*p.General)
		maps.Copy(errs, s.General.Validate())
	}
	if p.Rentals != nil {
		s.Rentals = s.Rentals.Apply(*p.Rentals)
		maps.Copy(errs, s.Rentals.Validate())
	}
	if p.Delivery != nil {
		s.Delivery = s.Delivery.Apply(*p.Delivery)
		maps.Copy(errs, s.Delivery.Validate())
	}
	return s, errs
}

var ErrNotFound = errors.New("business not found")

type Repo struct{ DB store.DB }

func (s Repo) Get(ctx context.Context, businessID string) (Settings, error) {
	var raw []byte
	err := s.DB.QueryRow(ctx, `select settings from public.businesses where id = $1`, businessID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, ErrNotFound
	}
	return From(raw), err
}

// Save merges every section's keys into settings (keys of other features are left as they are).
func (s Repo) Save(ctx context.Context, businessID string, st Settings) error {
	merged := map[string]any{}
	for _, part := range []any{st.General, st.Rentals, st.Delivery} {
		raw, _ := json.Marshal(part)
		_ = json.Unmarshal(raw, &merged)
	}
	raw, _ := json.Marshal(merged)
	_, err := s.DB.Exec(ctx, `update public.businesses set settings = settings || $2::jsonb where id = $1`, businessID, raw)
	return err
}
