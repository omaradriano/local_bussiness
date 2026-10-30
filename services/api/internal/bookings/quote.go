package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// rentalPeriods: how many days (price units) a rental from start to end charges, with the business's
// closed-day rules. The return can't fall on a closed day. The pick-up day is checked by the caller
// (an extension keeps the original one).
func rentalPeriods(loc *time.Location, hours []availability.Shift, r settings.Rentals, priceUnit string, start, end time.Time) (int64, error) {
	if !availability.OpenOn(loc, end, hours) {
		return 0, Invalid{"ends_at", "El negocio está cerrado ese día; elige otro día de regreso."}
	}
	periods := RentalPeriods(priceUnit, end.Sub(start))
	if r.ReturnByEndOfDay {
		periods = RentalPeriodsByDay(priceUnit, CalendarDays(loc, start, end))
	}
	if closed := availability.ClosedDaysBetween(loc, start, end, hours); closed > 0 {
		if !r.SpanClosedDays {
			return 0, Invalid{"ends_at", "El negocio no renta sobre días cerrados; elige fechas sin días cerrados en medio."}
		}
		if !r.ChargeClosedDays && priceUnit == "day" {
			periods = max(1, periods-int64(closed))
		}
	}
	return periods, nil
}

type QuoteRequest struct {
	ItemID   string
	Quantity int
	StartsAt time.Time
	EndsAt   *time.Time // rentals
	Discount Discount
}

type QuoteResult struct {
	Quote
	Periods int64 `json:"periods"`
}

// QuoteBooking prices a booking before creating it (same rules as Create, without options or delivery),
// so the form can show the total while the dates are picked.
func QuoteBooking(ctx context.Context, db store.DB, businessID string, in QuoteRequest) (QuoteResult, error) {
	var tz, kind, priceUnit string
	var raw []byte
	var price, deposit int64
	err := db.QueryRow(ctx, `select b.timezone, b.settings, ci.kind, ci.price, coalesce(ci.price_unit, 'day'), coalesce(ci.security_deposit, 0)
		from public.catalog_items ci join public.businesses b on b.id = ci.business_id
		where ci.business_id = $1 and ci.id = $2 and ci.active`, businessID, in.ItemID).Scan(&tz, &raw, &kind, &price, &priceUnit, &deposit)
	if errors.Is(err, pgx.ErrNoRows) {
		return QuoteResult{}, Invalid{"item_id", "El artículo no existe o está inactivo."}
	}
	if err != nil {
		return QuoteResult{}, err
	}
	qty := max(in.Quantity, 1)
	var set bizSettings
	_ = json.Unmarshal(raw, &set)
	rules, err := engines.Get(rulesByKind, kind)
	if err != nil {
		return QuoteResult{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return QuoteResult{}, err
	}
	rentals := settings.RentalsFrom(raw)
	hours := func() ([]availability.Shift, error) {
		return availability.Loader{DB: db}.BusinessHours(ctx, businessID)
	}
	periods, err := rules.pricing(periodQuery{loc: loc, hours: hours, rentals: rentals, priceUnit: priceUnit, start: in.StartsAt, end: in.EndsAt})
	if err != nil {
		return QuoteResult{}, err
	}
	if !rentals.SecurityDepositEnabled {
		deposit = 0
	}
	if err := in.Discount.Validate(); err != nil {
		return QuoteResult{}, err
	}
	q := Compute(QuoteInput{UnitPrice: price, Quantity: int64(qty), Periods: periods, DepositPercent: set.DepositPercent, SecurityDeposit: deposit,
		Discount: in.Discount})
	return QuoteResult{Quote: q, Periods: periods}, nil
}
