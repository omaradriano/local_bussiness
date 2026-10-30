package bookings

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
)

// An engine is a combination of small modules shared by every engine: when it happens (timing),
// where it's handed over (fulfillment), what it takes up (resource) and how many periods it
// charges (pricing), plus custody when it goes with the customer and comes back. A new engine
// picks existing modules and only writes the one that's missing.
type engineRules struct {
	timing      func(*reservation) error
	fulfillment func(*reservation) error
	resource    resource
	pricing     pricing
	custody     bool
}

var rulesByKind = map[string]engineRules{
	engines.Service: {timing: slotTiming, fulfillment: atPlaceOrHome, resource: staffSlot{}, pricing: fixedPrice},
	engines.Product: {timing: dayTiming, fulfillment: pickupOrDelivery, resource: dailyCapacity{}, pricing: fixedPrice},
	engines.Rental:  {timing: rangeTiming, fulfillment: atPlace, resource: units{}, pricing: perPeriod, custody: true},
}

// hasCustody: what's booked goes with the customer and comes back. It brings check-out and return
// (Checkout, Return), Extend, the security deposit and late or damage fees after the return.
func hasCustody(kind string) bool { return rulesByKind[kind].custody }

// reservation is what the modules read and fill while a booking is created.
type reservation struct {
	ctx         context.Context
	tx          pgx.Tx
	loader      availability.Loader
	businessID  string
	in          *CreateInput
	item        itemRow
	set         bizSettings
	rentals     settings.Rentals
	active      []string // the business's active features
	loc         *time.Location
	now         time.Time
	start       time.Time
	end         *time.Time
	deliveryFee int64
	candidates  []string // staff or unit ids that are free, in preference order
	hours       []availability.Shift
}

func (r *reservation) businessHours() ([]availability.Shift, error) {
	if r.hours != nil {
		return r.hours, nil
	}
	h, err := r.loader.BusinessHours(r.ctx, r.businessID)
	r.hours = h
	return h, err
}

// --- timing ---

// slotTiming: an appointment lasts the item's duration.
func slotTiming(r *reservation) error {
	e := r.start.Add(time.Duration(r.item.DurationMinutes) * time.Minute)
	r.end = &e
	return nil
}

// dayTiming: a delivery date, no end.
func dayTiming(*reservation) error { return nil }

// rangeTiming: from pick-up to return.
func rangeTiming(r *reservation) error {
	if r.in.EndsAt == nil || !r.in.EndsAt.After(r.start) {
		return Invalid{"ends_at", "El regreso debe ser después de la salida."}
	}
	r.end = r.in.EndsAt
	return nil
}

// --- fulfillment ---

// atPlaceOrHome: at the business, or at the customer's place if it gives home service.
func atPlaceOrHome(r *reservation) error {
	in := r.in
	if in.Fulfillment != "delivery" {
		in.Fulfillment, in.DeliveryAddress = "", ""
		return nil
	}
	if !slices.Contains(r.active, "homeService") {
		return Invalid{"fulfillment", "Este negocio no tiene servicio a domicilio."}
	}
	if in.DeliveryAddress = strings.TrimSpace(in.DeliveryAddress); in.DeliveryAddress == "" {
		return Invalid{"delivery_address", "Escribe la dirección del servicio."}
	}
	return nil
}

// pickupOrDelivery: picked up, or delivered with the zone's fee.
func pickupOrDelivery(r *reservation) error {
	in := r.in
	if in.Fulfillment == "" {
		in.Fulfillment = "pickup"
	}
	if in.Fulfillment != "pickup" && in.Fulfillment != "delivery" {
		return Invalid{"fulfillment", "Elige si se recoge o se entrega."}
	}
	if in.Fulfillment != "delivery" {
		return nil
	}
	if in.DeliveryAddress = strings.TrimSpace(in.DeliveryAddress); in.DeliveryAddress == "" {
		return Invalid{"delivery_address", "Escribe la dirección de entrega."}
	}
	if zones := r.set.DeliveryZones; len(zones) > 0 {
		i := slices.IndexFunc(zones, func(z zone) bool { return z.Name == in.DeliveryZone })
		if i < 0 {
			return Invalid{"delivery_zone", "Elige la zona de entrega."}
		}
		r.deliveryFee = zones[i].Fee
	}
	return nil
}

// atPlace: always at the business.
func atPlace(r *reservation) error {
	r.in.Fulfillment, r.in.DeliveryAddress = "", ""
	return nil
}

// --- resource ---

// resource: check re-verifies availability inside the transaction; claim takes it for the booking.
type resource interface {
	check(*reservation) error
	claim(r *reservation, bookingID string) error
}

// staffSlot: a person's time, plus the business's gap after each appointment.
type staffSlot struct{}

func (staffSlot) check(r *reservation) error {
	res, err := r.loader.Answer(r.ctx, r.businessID, availability.Request{ItemID: r.in.ItemID, Date: r.start.In(r.loc).Format("2006-01-02"), StaffID: r.in.StaffID}, r.now)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(res.Slots, func(s availability.Slot) bool { return s.Start.Equal(r.start) })
	if i < 0 {
		return ErrSlotTaken
	}
	r.candidates = res.Slots[i].StaffIDs
	return nil
}

func (staffSlot) claim(r *reservation, bookingID string) error {
	occupied := r.end.Add(time.Duration(r.set.BufferMinutes) * time.Minute)
	if !claim(r.ctx, r.tx, `insert into public.booking_resources (business_id, booking_id, staff_id, during) values ($1, $2, $3, tstzrange($4, $5))`,
		r.businessID, bookingID, r.candidates, 1, r.start, occupied) {
		return ErrSlotTaken
	}
	return nil
}

// dailyCapacity: spots per day; nothing to claim, the count of orders is the capacity.
type dailyCapacity struct{}

func (dailyCapacity) check(r *reservation) error {
	day := r.start.In(r.loc).Format("2006-01-02")
	// Serialize orders of this business for this day so two can't take the last spot.
	if _, err := r.tx.Exec(r.ctx, `select pg_advisory_xact_lock(hashtextextended($1 || $2, 0))`, r.businessID, day); err != nil {
		return err
	}
	res, err := r.loader.Answer(r.ctx, r.businessID, availability.Request{ItemID: r.in.ItemID, From: day, To: day, Quantity: r.in.Quantity}, r.now)
	if err != nil {
		return err
	}
	if len(res.Days) != 1 || !res.Days[0].Available {
		return ErrSlotTaken
	}
	return nil
}

func (dailyCapacity) claim(*reservation, string) error { return nil }

// units: physical units over the range; no pick-up on a closed day (spanning one is fine).
type units struct{}

func (units) check(r *reservation) error {
	res, err := r.loader.Answer(r.ctx, r.businessID, availability.Request{ItemID: r.in.ItemID,
		From: r.start.Format(time.RFC3339), To: r.end.Format(time.RFC3339), Quantity: r.in.Quantity}, r.now)
	if err != nil {
		return err
	}
	if !res.Available {
		return ErrSlotTaken
	}
	hours, err := r.businessHours()
	if err != nil {
		return err
	}
	if !availability.OpenOn(r.loc, r.start, hours) {
		return Invalid{"starts_at", "El negocio está cerrado ese día; elige otro día de salida."}
	}
	r.candidates = res.FreeUnits
	return nil
}

func (units) claim(r *reservation, bookingID string) error {
	// shortcut: the inspection gap is enforced by the availability check, not by the exclusion constraint;
	// a race inside that gap is possible. Store [start, end + gap) if it ever matters.
	if !claim(r.ctx, r.tx, `insert into public.booking_resources (business_id, booking_id, item_unit_id, during) values ($1, $2, $3, tstzrange($4, $5))`,
		r.businessID, bookingID, r.candidates, r.in.Quantity, r.start, *r.end) {
		return ErrSlotTaken
	}
	return nil
}

// --- pricing ---

// pricing returns how many price units (periods) a booking charges. Shared by Create and QuoteBooking.
type pricing func(p periodQuery) (int64, error)

type periodQuery struct {
	loc       *time.Location
	hours     func() ([]availability.Shift, error)
	rentals   settings.Rentals
	priceUnit string
	start     time.Time
	end       *time.Time
}

// fixedPrice: one price per booking.
func fixedPrice(periodQuery) (int64, error) { return 1, nil }

// perPeriod: per day, weekend or week, with the business's closed-day rules.
func perPeriod(p periodQuery) (int64, error) {
	if p.end == nil || !p.end.After(p.start) {
		return 0, Invalid{"ends_at", "El regreso debe ser después de la salida."}
	}
	hours, err := p.hours()
	if err != nil {
		return 0, err
	}
	return rentalPeriods(p.loc, hours, p.rentals, p.priceUnit, p.start, *p.end)
}
