// Package settings reads and updates the business policies kept in businesses.settings (jsonb),
// with defaults for anything not set yet.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/features"
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
	// Ask the advance only when the customer books online (catalog); counter bookings are confirmed
	// right away and paid at the end (barbershops).
	AdvanceOnlyOnline bool `json:"advance_only_online"`
}

func DefaultGeneral() General { return General{DepositPercent: 0, HoldHours: 24} }

type GeneralPatch struct {
	DepositPercent    *int  `json:"deposit_percent"`
	HoldHours         *int  `json:"hold_hours"`
	AdvanceOnlyOnline *bool `json:"advance_only_online"`
}

func (g General) Apply(p GeneralPatch) General {
	if p.DepositPercent != nil {
		g.DepositPercent = *p.DepositPercent
	}
	if p.HoldHours != nil {
		g.HoldHours = *p.HoldHours
	}
	if p.AdvanceOnlyOnline != nil {
		g.AdvanceOnlyOnline = *p.AdvanceOnlyOnline
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

// Agenda: rules of the appointment engine for the whole business.
type Agenda struct {
	BufferMinutes int `json:"buffer_minutes"` // gap after each appointment (clean up, get ready)
}

type AgendaPatch struct {
	BufferMinutes *int `json:"buffer_minutes"`
}

func (a Agenda) Apply(p AgendaPatch) Agenda {
	if p.BufferMinutes != nil {
		a.BufferMinutes = *p.BufferMinutes
	}
	return a
}

func (a Agenda) Validate() map[string]string {
	if a.BufferMinutes < 0 || a.BufferMinutes > 240 {
		return map[string]string{"buffer_minutes": "El margen entre citas debe ser de 0 a 240 minutos."}
	}
	return nil
}

// Settings: every section; the app shows the ones that apply to the type of business.
type Settings struct {
	General  General  `json:"general"`
	Agenda   Agenda   `json:"agenda"`
	Rentals  Rentals  `json:"rentals"`
	Delivery Delivery `json:"delivery"`
	// Features the giro supports, with what's available and what the owner has on. The owner only
	// changes "enabled" (stored in settings.features); availability is the platform admin's.
	Features []features.State `json:"features"`

	vertical string
	grants   map[string]bool
	enabled  map[string]bool
}

type Patch struct {
	General  *GeneralPatch   `json:"general"`
	Agenda   *AgendaPatch    `json:"agenda"`
	Rentals  *RentalsPatch   `json:"rentals"`
	Delivery *DeliveryPatch  `json:"delivery"`
	Features map[string]bool `json:"features"` // feature -> on/off (only available ones)
}

// From reads every section from the settings jsonb, with defaults for missing keys.
func From(raw []byte) Settings {
	s := Settings{General: DefaultGeneral(), Rentals: RentalsFrom(raw), Features: []features.State{}}
	_ = json.Unmarshal(raw, &s.General)
	_ = json.Unmarshal(raw, &s.Agenda)
	_ = json.Unmarshal(raw, &s.Delivery)
	if s.Delivery.Zones == nil {
		s.Delivery.Zones = []Zone{}
	}
	var f struct {
		Features map[string]bool `json:"features"`
	}
	_ = json.Unmarshal(raw, &f)
	s.enabled = f.Features
	return s
}

// WithBusiness adds the business's giro and the platform admin's grants, so Features can be listed.
func (s Settings) WithBusiness(vertical string, grants map[string]bool) Settings {
	s.vertical, s.grants = vertical, grants
	s.Features = features.States(vertical, grants, s.enabled)
	return s
}

// Apply merges the patch and validates the sections it touches.
func (s Settings) Apply(p Patch) (Settings, map[string]string) {
	errs := map[string]string{}
	if p.General != nil {
		s.General = s.General.Apply(*p.General)
		maps.Copy(errs, s.General.Validate())
	}
	if p.Agenda != nil {
		s.Agenda = s.Agenda.Apply(*p.Agenda)
		maps.Copy(errs, s.Agenda.Validate())
	}
	if p.Rentals != nil {
		s.Rentals = s.Rentals.Apply(*p.Rentals)
		maps.Copy(errs, s.Rentals.Validate())
	}
	if p.Delivery != nil {
		s.Delivery = s.Delivery.Apply(*p.Delivery)
		maps.Copy(errs, s.Delivery.Validate())
	}
	if p.Features != nil {
		enabled := maps.Clone(s.enabled)
		if enabled == nil {
			enabled = map[string]bool{}
		}
		for key, on := range p.Features {
			i := slices.IndexFunc(s.Features, func(st features.State) bool { return st.Key == key })
			if i < 0 || !s.Features[i].Available {
				errs["features"] = "Esa función no está disponible para tu negocio."
				continue
			}
			enabled[key] = on
		}
		s.enabled = enabled
		s.Features = features.States(s.vertical, s.grants, enabled)
	}
	return s, errs
}

var ErrNotFound = errors.New("business not found")

type Repo struct{ DB store.DB }

func (s Repo) Get(ctx context.Context, businessID string) (Settings, error) {
	var raw, grants []byte
	var vertical string
	err := s.DB.QueryRow(ctx, `select settings, vertical, feature_grants from public.businesses where id = $1`, businessID).
		Scan(&raw, &vertical, &grants)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, ErrNotFound
	}
	return From(raw).WithBusiness(vertical, features.Parse(grants)), err
}

// Save merges every section's keys into settings (keys of other features are left as they are).
func (s Repo) Save(ctx context.Context, businessID string, st Settings) error {
	merged := map[string]any{}
	for _, part := range []any{st.General, st.Agenda, st.Rentals, st.Delivery} {
		raw, _ := json.Marshal(part)
		_ = json.Unmarshal(raw, &merged)
	}
	if st.enabled != nil {
		merged["features"] = st.enabled
	}
	raw, _ := json.Marshal(merged)
	_, err := s.DB.Exec(ctx, `update public.businesses set settings = settings || $2::jsonb where id = $1`, businessID, raw)
	return err
}
