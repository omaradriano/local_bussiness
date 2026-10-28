package availability

import (
	"context"
	"slices"
	"time"
)

// BadRequest is a parameter problem worth showing to the user (es-MX).
type BadRequest struct{ Field, Message string }

func (e BadRequest) Error() string { return e.Message }

// Request: service -> Date (+ StaffID); product -> From/To dates; rental -> From/To instants + Quantity.
type Request struct {
	ItemID   string
	Date     string
	From, To string
	Quantity int
	StaffID  string
}

type Response struct {
	Kind      string   `json:"kind"`
	Slots     []Slot   `json:"slots,omitempty"`
	Days      []Day    `json:"days,omitempty"`
	FreeUnits []string `json:"free_units,omitempty"`
	Available bool     `json:"available"`
}

// Answer loads what the item's kind needs and runs the matching rule.
func (l Loader) Answer(ctx context.Context, businessID string, req Request, now time.Time) (Response, error) {
	biz, err := l.Business(ctx, businessID)
	if err != nil {
		return Response{}, err
	}
	item, err := l.Item(ctx, businessID, req.ItemID)
	if err != nil {
		return Response{}, err
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	switch item.Kind {
	case "service":
		return l.service(ctx, businessID, biz, item, req, now)
	case "product":
		return l.product(ctx, businessID, biz, item, req, now)
	default:
		return l.rental(ctx, businessID, biz, item, req, now)
	}
}

func (l Loader) service(ctx context.Context, businessID string, biz Business, item Item, req Request, now time.Time) (Response, error) {
	day, ok := parseDate(biz.Loc, req.Date)
	if !ok {
		return Response{}, BadRequest{"date", "Elige un día (AAAA-MM-DD)."}
	}
	// Load a bit more than the local day: shifts end on the same date, buffers may spill over.
	w := Interval{day.Add(-12 * time.Hour), day.Add(36 * time.Hour)}
	staff, err := l.ServiceStaff(ctx, businessID, item.ID, w)
	if err != nil {
		return Response{}, err
	}
	if req.StaffID != "" {
		staff = slices.DeleteFunc(staff, func(s Staff) bool { return s.ID != req.StaffID })
	}
	hours, err := l.BusinessHours(ctx, businessID)
	if err != nil {
		return Response{}, err
	}
	blackouts, err := l.Blackouts(ctx, businessID, w)
	if err != nil {
		return Response{}, err
	}
	slots := ServiceSlots(ServiceQuery{
		Loc: biz.Loc, Date: req.Date, Now: now,
		Duration: time.Duration(item.DurationMinutes) * time.Minute, Buffer: time.Duration(biz.BufferMinutes) * time.Minute,
		Step: time.Duration(biz.SlotMinutes) * time.Minute, BusinessHours: hours, Staff: staff, Blackouts: blackouts,
	})
	return Response{Kind: "service", Slots: slots, Available: len(slots) > 0}, nil
}

func (l Loader) product(ctx context.Context, businessID string, biz Business, item Item, req Request, now time.Time) (Response, error) {
	from, ok := parseDate(biz.Loc, req.From)
	if !ok {
		return Response{}, BadRequest{"from", "Elige desde qué día (AAAA-MM-DD)."}
	}
	to := from.AddDate(0, 0, 13) // two weeks by default
	if req.To != "" {
		if to, ok = parseDate(biz.Loc, req.To); !ok || to.Before(from) {
			return Response{}, BadRequest{"to", "El último día debe ser igual o posterior al primero."}
		}
	}
	if to.Sub(from) > maxDays*24*time.Hour {
		return Response{}, BadRequest{"to", "Consulta como máximo dos meses."}
	}
	toStr := to.Format("2006-01-02")
	hours, err := l.BusinessHours(ctx, businessID)
	if err != nil {
		return Response{}, err
	}
	blackouts, err := l.Blackouts(ctx, businessID, Interval{from, to.AddDate(0, 0, 1)})
	if err != nil {
		return Response{}, err
	}
	booked, err := l.ProductBooked(ctx, businessID, item.ID, biz.Loc.String(), req.From, toStr)
	if err != nil {
		return Response{}, err
	}
	days := ProductDays(ProductQuery{
		Loc: biz.Loc, From: req.From, To: toStr, Now: now, PrepHours: item.PrepHours, Quantity: req.Quantity,
		BusinessHours: hours, Blackouts: blackouts, DailyCapacity: biz.DailyCapacity, ItemCapacity: item.DailyCapacity, Booked: booked,
	})
	return Response{Kind: "product", Days: days, Available: slices.ContainsFunc(days, func(d Day) bool { return d.Available })}, nil
}

func (l Loader) rental(ctx context.Context, businessID string, biz Business, item Item, req Request, now time.Time) (Response, error) {
	if _, isDay := parseDate(biz.Loc, req.From); isDay {
		return l.rentalDays(ctx, businessID, biz, item, req, now)
	}
	from, err1 := time.Parse(time.RFC3339, req.From)
	to, err2 := time.Parse(time.RFC3339, req.To)
	if err1 != nil || err2 != nil {
		return Response{}, BadRequest{"from", "Indica salida y regreso con fecha y hora."}
	}
	if !from.Before(to) {
		return Response{}, BadRequest{"to", "El regreso debe ser después de la salida."}
	}
	if to.Sub(from) > 366*24*time.Hour {
		return Response{}, BadRequest{"to", "La renta no puede durar más de un año."}
	}
	gap := time.Duration(biz.InspectionMinutes) * time.Minute
	w := Interval{from.Add(-gap), to.Add(gap)}
	units, err := l.RentalUnits(ctx, businessID, item.ID, w)
	if err != nil {
		return Response{}, err
	}
	blackouts, err := l.Blackouts(ctx, businessID, w)
	if err != nil {
		return Response{}, err
	}
	free := RentalFreeUnits(RentalQuery{From: from, To: to, Now: now, Quantity: req.Quantity, Inspection: gap, Units: units, Blackouts: blackouts})
	return Response{Kind: "rental", FreeUnits: free, Available: len(free) >= req.Quantity}, nil
}

// rentalDays answers the calendar: free units per day between two local dates.
func (l Loader) rentalDays(ctx context.Context, businessID string, biz Business, item Item, req Request, now time.Time) (Response, error) {
	from, _ := parseDate(biz.Loc, req.From)
	to, ok := parseDate(biz.Loc, req.To)
	if !ok || to.Before(from) {
		return Response{}, BadRequest{"to", "El último día debe ser igual o posterior al primero."}
	}
	if to.Sub(from) > maxDays*24*time.Hour {
		return Response{}, BadRequest{"to", "Consulta como máximo dos meses."}
	}
	gap := time.Duration(biz.InspectionMinutes) * time.Minute
	w := Interval{from.Add(-gap), to.AddDate(0, 0, 1).Add(gap)}
	units, err := l.RentalUnits(ctx, businessID, item.ID, w)
	if err != nil {
		return Response{}, err
	}
	hours, err := l.BusinessHours(ctx, businessID)
	if err != nil {
		return Response{}, err
	}
	blackouts, err := l.Blackouts(ctx, businessID, w)
	if err != nil {
		return Response{}, err
	}
	days := RentalDays(RentalDaysQuery{Loc: biz.Loc, From: req.From, To: to.Format("2006-01-02"), Now: now, Quantity: req.Quantity,
		Inspection: gap, BusinessHours: hours, Units: units, Blackouts: blackouts})
	return Response{Kind: "rental", Days: days, Available: slices.ContainsFunc(days, func(d Day) bool { return d.Available })}, nil
}
