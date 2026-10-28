package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// ErrSlotTaken: what was asked for is no longer free (someone else got it, or it never was).
var ErrSlotTaken = errors.New("slot taken")

// Invalid is a field problem worth showing to the user (es-MX).
type Invalid struct{ Field, Message string }

func (e Invalid) Error() string { return e.Message }

type CustomerInput struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type CreateInput struct {
	Customer        CustomerInput     `json:"customer"`
	ItemID          string            `json:"item_id"`
	Quantity        int               `json:"quantity"`
	StartsAt        *time.Time        `json:"starts_at"` // service: appointment; product: delivery; rental: pick-up
	EndsAt          *time.Time        `json:"ends_at"`   // rental: return
	StaffID         string            `json:"staff_id"`  // service: preferred person (optional)
	Fulfillment     string            `json:"fulfillment"`
	DeliveryAddress string            `json:"delivery_address"`
	DeliveryZone    string            `json:"delivery_zone"`
	Options         map[string]string `json:"options"` // option name -> answer
	Notes           string            `json:"notes"`
	// Counter only: what the customer pays right now (type is set here: all of it = balance, part = advance).
	InitialPayment *Payment `json:"initial_payment"`

	Source         string `json:"-"` // admin | public
	CreatedBy      string `json:"-"` // user id for admin bookings
	IdempotencyKey string `json:"-"`
}

type zone struct {
	Name string `json:"name"`
	Fee  int64  `json:"fee"`
}

type bizSettings struct {
	DepositPercent int64  `json:"deposit_percent"`
	HoldHours      int    `json:"hold_hours"`
	DeliveryZones  []zone `json:"delivery_zones"`
}

type itemRow struct {
	Kind            string
	Name            string
	Price           int64
	DurationMinutes int
	BufferMinutes   int
	PriceUnit       string
	SecurityDeposit int64
}

type option struct {
	Name       string
	Type       string
	Choices    []string
	ExtraPrice int64
	Required   bool
}

// Validate checks the input shape (not availability). Kind-specific checks happen once the item is known.
func (in *CreateInput) Validate() error {
	if in.Quantity == 0 {
		in.Quantity = 1
	}
	if in.Quantity < 1 || in.Quantity > 100 {
		return Invalid{"quantity", "La cantidad debe ser de 1 a 100."}
	}
	if in.Customer.ID == "" {
		in.Customer.Name = strings.TrimSpace(in.Customer.Name)
		if n := utf8.RuneCountInString(in.Customer.Name); n == 0 || n > 80 {
			return Invalid{"customer.name", "Escribe el nombre del cliente."}
		}
		if in.Customer.Phone = NormalizePhone(in.Customer.Phone); in.Customer.Phone == "" {
			return Invalid{"customer.phone", "Escribe un teléfono válido de 10 dígitos."}
		}
	}
	if in.StartsAt == nil {
		return Invalid{"starts_at", "Elige la fecha y hora."}
	}
	if utf8.RuneCountInString(in.Notes) > 1000 {
		return Invalid{"notes", "Las notas son muy largas."}
	}
	return nil
}

// Create books the item atomically. It returns the booking and whether it already existed (same Idempotency-Key).
// Availability is re-checked inside the transaction; the DB exclusion constraints settle races (F3-2).
func Create(ctx context.Context, db store.DB, businessID string, in CreateInput, now time.Time) (Booking, bool, error) {
	if err := in.Validate(); err != nil {
		return Booking{}, false, err
	}
	if in.IdempotencyKey != "" {
		if id, err := idByKey(ctx, db, businessID, in.IdempotencyKey); err != nil || id != "" {
			if err != nil {
				return Booking{}, false, err
			}
			b, err := Get(ctx, db, businessID, id)
			return b, true, err
		}
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, false, err
	}
	defer tx.Rollback(ctx)

	var tz string
	var rawSettings []byte
	if err := tx.QueryRow(ctx, `select timezone, settings from public.businesses where id = $1`, businessID).Scan(&tz, &rawSettings); err != nil {
		return Booking{}, false, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Booking{}, false, err
	}
	var set bizSettings
	_ = json.Unmarshal(rawSettings, &set)
	rentals := settings.RentalsFrom(rawSettings)

	var it itemRow
	err = tx.QueryRow(ctx, `select kind, name, price, coalesce(duration_minutes, 0), buffer_minutes, coalesce(price_unit, ''), coalesce(security_deposit, 0)
		from public.catalog_items where business_id = $1 and id = $2 and active`, businessID, in.ItemID).
		Scan(&it.Kind, &it.Name, &it.Price, &it.DurationMinutes, &it.BufferMinutes, &it.PriceUnit, &it.SecurityDeposit)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, Invalid{"item_id", "El artículo no existe o está inactivo."}
	}
	if err != nil {
		return Booking{}, false, err
	}

	start := *in.StartsAt
	var end *time.Time
	periods := int64(1)
	var deliveryFee int64
	switch it.Kind {
	case "service":
		e := start.Add(time.Duration(it.DurationMinutes) * time.Minute)
		end = &e
		in.Fulfillment, in.DeliveryAddress = "", ""
	case "product":
		if in.Fulfillment == "" {
			in.Fulfillment = "pickup"
		}
		if in.Fulfillment != "pickup" && in.Fulfillment != "delivery" {
			return Booking{}, false, Invalid{"fulfillment", "Elige si se recoge o se entrega."}
		}
		if in.Fulfillment == "delivery" {
			if in.DeliveryAddress = strings.TrimSpace(in.DeliveryAddress); in.DeliveryAddress == "" {
				return Booking{}, false, Invalid{"delivery_address", "Escribe la dirección de entrega."}
			}
			if len(set.DeliveryZones) > 0 {
				i := slices.IndexFunc(set.DeliveryZones, func(z zone) bool { return z.Name == in.DeliveryZone })
				if i < 0 {
					return Booking{}, false, Invalid{"delivery_zone", "Elige la zona de entrega."}
				}
				deliveryFee = set.DeliveryZones[i].Fee
			}
		}
	default: // rental
		if in.EndsAt == nil || !in.EndsAt.After(start) {
			return Booking{}, false, Invalid{"ends_at", "El regreso debe ser después de la salida."}
		}
		end = in.EndsAt
		in.Fulfillment, in.DeliveryAddress = "", ""
	}

	optsExtra, answers, err := checkOptions(ctx, tx, businessID, in.ItemID, in.Options)
	if err != nil {
		return Booking{}, false, err
	}

	// Availability, re-checked inside the transaction.
	loader := availability.Loader{DB: tx}
	var candidates []string // staff ids (service) or unit ids (rental) that are free, in preference order
	switch it.Kind {
	case "service":
		res, err := loader.Answer(ctx, businessID, availability.Request{ItemID: in.ItemID, Date: start.In(loc).Format("2006-01-02"), StaffID: in.StaffID}, now)
		if err != nil {
			return Booking{}, false, err
		}
		i := slices.IndexFunc(res.Slots, func(s availability.Slot) bool { return s.Start.Equal(start) })
		if i < 0 {
			return Booking{}, false, ErrSlotTaken
		}
		candidates = res.Slots[i].StaffIDs
	case "product":
		day := start.In(loc).Format("2006-01-02")
		// Serialize orders of this business for this day so two can't take the last spot.
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1 || $2, 0))`, businessID, day); err != nil {
			return Booking{}, false, err
		}
		res, err := loader.Answer(ctx, businessID, availability.Request{ItemID: in.ItemID, From: day, To: day, Quantity: in.Quantity}, now)
		if err != nil {
			return Booking{}, false, err
		}
		if len(res.Days) != 1 || !res.Days[0].Available {
			return Booking{}, false, ErrSlotTaken
		}
	default:
		res, err := loader.Answer(ctx, businessID, availability.Request{ItemID: in.ItemID,
			From: start.Format(time.RFC3339), To: end.Format(time.RFC3339), Quantity: in.Quantity}, now)
		if err != nil {
			return Booking{}, false, err
		}
		if !res.Available {
			return Booking{}, false, ErrSlotTaken
		}
		// No pick-up or return on a closed day (spanning one is fine).
		hours, err := loader.BusinessHours(ctx, businessID)
		if err != nil {
			return Booking{}, false, err
		}
		if !availability.OpenOn(loc, start, hours) {
			return Booking{}, false, Invalid{"starts_at", "El negocio está cerrado ese día; elige otro día de salida."}
		}
		if periods, err = rentalPeriods(loc, hours, rentals, it.PriceUnit, start, *end); err != nil {
			return Booking{}, false, err
		}
		candidates = res.FreeUnits
	}

	customerID, err := upsertCustomer(ctx, tx, businessID, in.Customer)
	if err != nil {
		return Booking{}, false, err
	}

	securityDeposit := it.SecurityDeposit
	if !rentals.SecurityDepositEnabled {
		securityDeposit = 0 // the business doesn't ask for one
	}
	q := Compute(QuoteInput{UnitPrice: it.Price, OptionsExtra: optsExtra, Quantity: int64(in.Quantity), Periods: periods,
		DeliveryFee: deliveryFee, DepositPercent: set.DepositPercent, SecurityDeposit: securityDeposit})
	status, hold := InitialStatus(q.Advance, set.HoldHours, now)
	pay := in.InitialPayment
	if pay != nil {
		if pay.Amount == 0 {
			pay.Amount = q.Total // no amount = pays it all (the form doesn't know the exact total)
		}
		pay.Type = "advance"
		if pay.Amount == q.Total {
			pay.Type = "balance"
		}
		if err := ValidatePayment(*pay, Balance{Due: q.Total}); err != nil {
			return Booking{}, false, err
		}
		if status == "pending_deposit" && pay.Amount >= q.Advance {
			status, hold = "confirmed", nil
		}
	}

	var bookingID string
	err = tx.QueryRow(ctx, `
		insert into public.bookings (business_id, customer_id, kind, status, starts_at, ends_at, fulfillment, delivery_address,
		  subtotal, deposit_required, total, notes, source, hold_expires_at, idempotency_key, created_by)
		values ($1, $2, $3, $4, $5, $6, nullif($7, ''), nullif($8, ''), $9, $10, $11, nullif($12, ''), $13, $14, nullif($15, ''), nullif($16, '')::uuid)
		returning id::text`,
		businessID, customerID, it.Kind, status, start, end, in.Fulfillment, in.DeliveryAddress,
		q.Subtotal, q.Advance, q.Total, strings.TrimSpace(in.Notes), in.Source, hold, in.IdempotencyKey, in.CreatedBy).Scan(&bookingID)
	if isUnique(err) && in.IdempotencyKey != "" {
		tx.Rollback(ctx) // a concurrent retry with the same key won: return that one
		id, err := idByKey(ctx, db, businessID, in.IdempotencyKey)
		if err != nil {
			return Booking{}, false, err
		}
		b, err := Get(ctx, db, businessID, id)
		return b, true, err
	}
	if err != nil {
		return Booking{}, false, err
	}

	opts, _ := json.Marshal(map[string]any{"answers": answers, "options_extra": optsExtra, "periods": periods,
		"delivery_fee": deliveryFee, "security_deposit": q.SecurityDeposit, "item_name": it.Name})
	if _, err := tx.Exec(ctx, `insert into public.booking_items (business_id, booking_id, item_id, quantity, unit_price, options)
		values ($1, $2, $3, $4, $5, $6)`, businessID, bookingID, in.ItemID, in.Quantity, it.Price, opts); err != nil {
		return Booking{}, false, err
	}

	switch it.Kind {
	case "service":
		occupied := end.Add(time.Duration(it.BufferMinutes) * time.Minute)
		if !claim(ctx, tx, `insert into public.booking_resources (business_id, booking_id, staff_id, during) values ($1, $2, $3, tstzrange($4, $5))`,
			businessID, bookingID, candidates, 1, start, occupied) {
			return Booking{}, false, ErrSlotTaken
		}
	case "rental":
		// shortcut: the inspection gap is enforced by the availability check, not by the exclusion constraint;
		// a race inside that gap is possible. Store [start, end + gap) if it ever matters.
		if !claim(ctx, tx, `insert into public.booking_resources (business_id, booking_id, item_unit_id, during) values ($1, $2, $3, tstzrange($4, $5))`,
			businessID, bookingID, candidates, in.Quantity, start, *end) {
			return Booking{}, false, ErrSlotTaken
		}
	}

	if _, err := tx.Exec(ctx, `insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, note)
		values ($1, $2, null, $3, nullif($4, '')::uuid, $5)`, businessID, bookingID, status, in.CreatedBy, "Alta ("+in.Source+")"); err != nil {
		return Booking{}, false, err
	}
	if pay != nil {
		if _, err := tx.Exec(ctx, `insert into public.payments (business_id, booking_id, type, method, amount, paid_at, recorded_by, notes)
			values ($1, $2, $3, $4, $5, $6, nullif($7, '')::uuid, nullif($8, ''))`,
			businessID, bookingID, pay.Type, pay.Method, pay.Amount, now, in.CreatedBy, strings.TrimSpace(deref(pay.Notes))); err != nil {
			return Booking{}, false, err
		}
	}
	// Copy the configured steps that apply, so later config changes don't affect this booking.
	if _, err := tx.Exec(ctx, `
		insert into public.booking_steps (business_id, booking_id, step_snapshot)
		select ws.business_id, $2, to_jsonb(ws) from public.workflow_steps ws
		where ws.business_id = $1 and ws.active and ws.applies_to_kind = $3 and (ws.item_id is null or ws.item_id = $4)
		order by ws.sort`, businessID, bookingID, it.Kind, in.ItemID); err != nil {
		return Booking{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Booking{}, false, err
	}
	b, err := Get(ctx, db, businessID, bookingID)
	return b, false, err
}

// claim inserts one resource row per needed candidate, trying the next one when the exclusion
// constraint says it was just taken (23P01). Each try runs in a savepoint.
func claim(ctx context.Context, tx pgx.Tx, sql, businessID, bookingID string, candidates []string, need int, from, to time.Time) bool {
	got := 0
	for _, c := range candidates {
		if got == need {
			break
		}
		sp, err := tx.Begin(ctx)
		if err != nil {
			return false
		}
		if _, err := sp.Exec(ctx, sql, businessID, bookingID, c, from, to); err != nil {
			sp.Rollback(ctx)
			continue
		}
		if sp.Commit(ctx) == nil {
			got++
		}
	}
	return got == need
}

func checkOptions(ctx context.Context, tx pgx.Tx, businessID, itemID string, answers map[string]string) (int64, map[string]string, error) {
	rows, err := tx.Query(ctx, `select name, type, coalesce(choices, '[]'::jsonb), extra_price, required
		from public.item_options where business_id = $1 and item_id = $2 order by sort`, businessID, itemID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var extra int64
	clean := map[string]string{}
	for rows.Next() {
		var o option
		var choices []byte
		if err := rows.Scan(&o.Name, &o.Type, &choices, &o.ExtraPrice, &o.Required); err != nil {
			return 0, nil, err
		}
		_ = json.Unmarshal(choices, &o.Choices)
		a := strings.TrimSpace(answers[o.Name])
		if a == "" {
			if o.Required {
				return 0, nil, Invalid{"options." + o.Name, fmt.Sprintf("Falta: %s.", o.Name)}
			}
			continue
		}
		if o.Type == "choice" && !slices.Contains(o.Choices, a) {
			return 0, nil, Invalid{"options." + o.Name, fmt.Sprintf("Elige una opción válida de %s.", o.Name)}
		}
		if utf8.RuneCountInString(a) > 300 {
			return 0, nil, Invalid{"options." + o.Name, fmt.Sprintf("%s es muy largo.", o.Name)}
		}
		clean[o.Name] = a
		extra += o.ExtraPrice
	}
	return extra, clean, rows.Err()
}

func upsertCustomer(ctx context.Context, tx pgx.Tx, businessID string, c CustomerInput) (string, error) {
	var id string
	if c.ID != "" {
		err := tx.QueryRow(ctx, `select id::text from public.customers where business_id = $1 and id = $2`, businessID, c.ID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", Invalid{"customer.id", "El cliente no existe."}
		}
		return id, err
	}
	// Same phone in the same business = same customer; keep the name they already had.
	err := tx.QueryRow(ctx, `
		insert into public.customers (business_id, name, phone, email) values ($1, $2, $3, nullif($4, ''))
		on conflict (business_id, phone) do update set email = coalesce(public.customers.email, excluded.email)
		returning id::text`, businessID, c.Name, c.Phone, strings.TrimSpace(c.Email)).Scan(&id)
	return id, err
}

func idByKey(ctx context.Context, db store.DB, businessID, key string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `select id::text from public.bookings where business_id = $1 and idempotency_key = $2`, businessID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
