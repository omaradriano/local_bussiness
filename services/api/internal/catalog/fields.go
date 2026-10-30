package catalog

import (
	"slices"

	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
)

// An item carries the fields of its engine's modules (see bookings/modules.go). Each group is
// written once and shared by every engine that uses that module; an item of another engine has
// them cleared.
type fieldGroup struct {
	validate func(it Item, errs map[string]string)
	clear    func(it *Item)
}

type kindRules struct {
	fields []*fieldGroup
	units  bool // physical units with a code each
}

var rulesByKind = map[string]kindRules{
	engines.Service: {fields: []*fieldGroup{&durationFields}},
	engines.Product: {fields: []*fieldGroup{&dailyFields}},
	engines.Rental:  {fields: []*fieldGroup{&periodFields, &custodyFields}, units: true},
}

var fieldGroups = []*fieldGroup{&durationFields, &dailyFields, &periodFields, &custodyFields}

// durationFields: a time slot (how long it takes and the gap after it).
var durationFields = fieldGroup{
	validate: func(it Item, errs map[string]string) {
		if it.DurationMinutes == nil || *it.DurationMinutes < 5 || *it.DurationMinutes > 600 {
			errs["duration_minutes"] = "La duración debe ser de 5 a 600 minutos."
		}
		if it.BufferMinutes < 0 || it.BufferMinutes > 240 {
			errs["buffer_minutes"] = "El margen entre citas debe ser de 0 a 240 minutos."
		}
	},
	clear: func(it *Item) { it.DurationMinutes, it.BufferMinutes = nil, 0 },
}

// dailyFields: a delivery day (lead time and spots per day).
var dailyFields = fieldGroup{
	validate: func(it Item, errs map[string]string) {
		if it.PrepHours == nil || *it.PrepHours < 0 || *it.PrepHours > 720 {
			errs["prep_hours"] = "La anticipación debe ser de 0 a 720 horas."
		}
		if it.DailyCapacity != nil && (*it.DailyCapacity < 1 || *it.DailyCapacity > 1000) {
			errs["daily_capacity"] = "El cupo diario debe ser de 1 a 1000, o dejarse vacío."
		}
	},
	clear: func(it *Item) { it.PrepHours, it.DailyCapacity = nil, nil },
}

// periodFields: priced per day, weekend or week.
var periodFields = fieldGroup{
	validate: func(it Item, errs map[string]string) {
		if it.PriceUnit == nil || !slices.Contains([]string{"day", "weekend", "week"}, *it.PriceUnit) {
			errs["price_unit"] = "Elige si se cobra por día, fin de semana o semana."
		}
	},
	clear: func(it *Item) { it.PriceUnit = nil },
}

// custodyFields: it goes with the customer (security deposit, late fee).
var custodyFields = fieldGroup{
	validate: func(it Item, errs map[string]string) {
		money(errs, "security_deposit", it.SecurityDeposit, "Escribe el depósito de garantía (puede ser $0).")
		money(errs, "late_fee_per_day", it.LateFeePerDay, "Escribe el recargo por día de retraso (puede ser $0).")
	},
	clear: func(it *Item) { it.SecurityDeposit, it.LateFeePerDay = nil, nil },
}

func money(errs map[string]string, field string, v *int64, label string) {
	if v == nil || *v < 0 || *v > maxPrice {
		errs[field] = label
	}
}
