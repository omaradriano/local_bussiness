// Package availability answers "what is free?" for the three kinds of catalog item. Pure functions:
// they get a snapshot of the data and an injected "now", so every rule is table-tested without a DB.
// Days and hours are read in the business time zone; instants are absolute (time.Time).
package availability

import (
	"sort"
	"time"
)

// Interval is [Start, End).
type Interval struct{ Start, End time.Time }

func (a Interval) overlaps(b Interval) bool { return a.Start.Before(b.End) && b.Start.Before(a.End) }

// Shift: weekday 0 = domingo, "HH:MM" wall-clock in the business time zone.
type Shift struct {
	Weekday       int
	Opens, Closes string
}

// Blackout closes the whole business, or only one staff member or rental unit.
type Blackout struct {
	StaffID string
	UnitID  string
	Interval
}

func (b Blackout) appliesTo(staffID, unitID string) bool {
	switch {
	case b.StaffID != "":
		return b.StaffID == staffID
	case b.UnitID != "":
		return b.UnitID == unitID
	default:
		return true // whole business
	}
}

// shiftsOn turns the weekly shifts that fall on a local date into absolute intervals.
func shiftsOn(loc *time.Location, date time.Time, shifts []Shift) []Interval {
	var out []Interval
	for _, s := range shifts {
		if s.Weekday != int(date.Weekday()) {
			continue
		}
		open, err1 := atClock(loc, date, s.Opens)
		closeAt, err2 := atClock(loc, date, s.Closes)
		if err1 == nil && err2 == nil && open.Before(closeAt) {
			out = append(out, Interval{open, closeAt})
		}
	}
	return out
}

func atClock(loc *time.Location, date time.Time, hhmm string) (time.Time, error) {
	c, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	y, m, d := date.Date()
	return time.Date(y, m, d, c.Hour(), c.Minute(), 0, 0, loc), nil
}

func parseDate(loc *time.Location, day string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", day, loc)
	return t, err == nil
}

// ---------------------------------------------------------------- services

type Staff struct {
	ID    string
	Hours []Shift    // empty: the business hours apply
	Busy  []Interval // existing booking_resources (they already include each booking's buffer)
}

type ServiceQuery struct {
	Loc           *time.Location
	Date          string // local YYYY-MM-DD
	Now           time.Time
	Duration      time.Duration
	Buffer        time.Duration // cleanup after the service; may run past closing
	Step          time.Duration // slot granularity, e.g. 15 or 30 min
	BusinessHours []Shift
	Staff         []Staff // only active people who do this service
	Blackouts     []Blackout
}

type Slot struct {
	Start    time.Time `json:"starts_at"`
	End      time.Time `json:"ends_at"`
	StaffIDs []string  `json:"staff_ids"` // who can attend it, in input order
}

// ServiceSlots lists start times on the date where at least one person can attend the service:
// it fits inside their shift, doesn't cross a blackout or another booking (incl. buffer), and isn't past.
func ServiceSlots(q ServiceQuery) []Slot {
	date, ok := parseDate(q.Loc, q.Date)
	if !ok || q.Duration <= 0 || q.Step <= 0 {
		return nil
	}
	byStart := map[int64]*Slot{}
	for _, st := range q.Staff {
		hours := st.Hours
		if len(hours) == 0 {
			hours = q.BusinessHours
		}
		for _, shift := range shiftsOn(q.Loc, date, hours) {
			for start := shift.Start; !start.Add(q.Duration).After(shift.End); start = start.Add(q.Step) {
				if start.Before(q.Now) {
					continue
				}
				occupied := Interval{start, start.Add(q.Duration + q.Buffer)}
				if !free(occupied, st.Busy, q.Blackouts, st.ID, "") {
					continue
				}
				key := start.Unix()
				if byStart[key] == nil {
					byStart[key] = &Slot{Start: start, End: start.Add(q.Duration)}
				}
				byStart[key].StaffIDs = append(byStart[key].StaffIDs, st.ID)
			}
		}
	}
	out := make([]Slot, 0, len(byStart))
	for _, s := range byStart {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	if len(out) == 0 {
		return nil
	}
	return out
}

func free(want Interval, busy []Interval, blackouts []Blackout, staffID, unitID string) bool {
	for _, b := range busy {
		if want.overlaps(b) {
			return false
		}
	}
	for _, b := range blackouts {
		if b.appliesTo(staffID, unitID) && want.overlaps(b.Interval) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- products (made to order)

// Booked is what's already ordered for a local date: all products, and this item.
type Booked struct{ All, Item int }

type ProductQuery struct {
	Loc           *time.Location
	From, To      string // local dates, inclusive
	Now           time.Time
	PrepHours     int // minimum lead time
	Quantity      int
	BusinessHours []Shift
	Blackouts     []Blackout        // whole-business ones count
	DailyCapacity int               // business-wide orders per day; 0 = no limit
	ItemCapacity  int               // this item per day; 0 = no limit
	Booked        map[string]Booked // by local date
}

type Day struct {
	Date      string `json:"date"`
	Available bool   `json:"available"`
	Remaining int    `json:"remaining"`        // -1 = no limit
	Closed    bool   `json:"closed,omitempty"` // rentals: no pick-up/return that day, but a rental can span it
}

const maxDays = 62

// ProductDays says, for each date, whether an order can be delivered: the business is open at some
// time after the lead time that isn't blacked out, and there is capacity left for the quantity.
func ProductDays(q ProductQuery) []Day {
	from, ok1 := parseDate(q.Loc, q.From)
	to, ok2 := parseDate(q.Loc, q.To)
	if !ok1 || !ok2 || to.Before(from) {
		return nil
	}
	earliest := q.Now.Add(time.Duration(q.PrepHours) * time.Hour)
	var out []Day
	for d, n := from, 0; !d.After(to) && n < maxDays; d, n = d.AddDate(0, 0, 1), n+1 {
		key := d.Format("2006-01-02")
		day := Day{Date: key, Remaining: remaining(q, q.Booked[key])}
		day.Available = deliverable(q, d, earliest) && (day.Remaining == -1 || day.Remaining >= q.Quantity)
		out = append(out, day)
	}
	return out
}

func remaining(q ProductQuery, b Booked) int {
	r := -1
	if q.DailyCapacity > 0 {
		r = max(0, q.DailyCapacity-b.All)
	}
	if q.ItemCapacity > 0 {
		item := max(0, q.ItemCapacity-b.Item)
		if r == -1 || item < r {
			r = item
		}
	}
	return r
}

// deliverable: some instant of the day's open hours is at/after the lead time and outside blackouts.
func deliverable(q ProductQuery, date, earliest time.Time) bool {
	for _, shift := range shiftsOn(q.Loc, date, q.BusinessHours) {
		if shift.Start.Before(earliest) {
			shift.Start = earliest
		}
		if !shift.Start.Before(shift.End) {
			continue
		}
		if !coveredByBlackouts(shift, q.Blackouts) {
			return true
		}
	}
	return false
}

// coveredByBlackouts reports whether whole-business blackouts leave no gap inside w.
func coveredByBlackouts(w Interval, blackouts []Blackout) bool {
	var bs []Interval
	for _, b := range blackouts {
		if b.appliesTo("", "") && b.overlaps(w) {
			bs = append(bs, b.Interval)
		}
	}
	sort.Slice(bs, func(i, j int) bool { return bs[i].Start.Before(bs[j].Start) })
	cursor := w.Start
	for _, b := range bs {
		if b.Start.After(cursor) {
			return false // gap before this blackout
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
	}
	return !cursor.Before(w.End)
}

// ---------------------------------------------------------------- rentals

type Unit struct {
	ID   string
	Busy []Interval // existing rentals of this unit
}

type RentalQuery struct {
	From, To   time.Time
	Now        time.Time
	Quantity   int
	Inspection time.Duration // check-up gap required between two rentals of the same unit
	Units      []Unit        // only units in status "available"
	Blackouts  []Blackout
}

// RentalFreeUnits returns the units that can go out for the whole range, keeping the inspection gap
// before and after. The caller compares the count with the requested quantity.
func RentalFreeUnits(q RentalQuery) []string {
	if !q.From.Before(q.To) || q.From.Before(q.Now) {
		return nil
	}
	want := Interval{q.From.Add(-q.Inspection), q.To.Add(q.Inspection)}
	var out []string
	for _, u := range q.Units {
		if free(want, u.Busy, q.Blackouts, "", u.ID) {
			out = append(out, u.ID)
		}
	}
	return out
}

type RentalDaysQuery struct {
	Loc           *time.Location
	From, To      string // local dates, inclusive
	Now           time.Time
	Quantity      int
	Inspection    time.Duration
	BusinessHours []Shift
	Units         []Unit // only units in status "available"
	Blackouts     []Blackout
}

// RentalDays paints a calendar: a day is available when the business opens and at least Quantity units
// are free during its open hours (plus the inspection gap). Remaining = free units that day.
// The exact pick-up/return range is still checked with RentalFreeUnits before booking.
func RentalDays(q RentalDaysQuery) []Day {
	from, ok1 := parseDate(q.Loc, q.From)
	to, ok2 := parseDate(q.Loc, q.To)
	if !ok1 || !ok2 || to.Before(from) {
		return nil
	}
	var out []Day
	for d, n := from, 0; !d.After(to) && n < maxDays; d, n = d.AddDate(0, 0, 1), n+1 {
		day := Day{Date: d.Format("2006-01-02")}
		// The window that must be free: open hours, or the whole day when closed (the tool can
		// stay with the customer over a Sunday; it just can't go out or come back that day).
		window := Interval{d, d.AddDate(0, 0, 1)}
		shifts := shiftsOn(q.Loc, d, q.BusinessHours)
		day.Closed = len(shifts) == 0
		for i, s := range shifts {
			if i == 0 || s.Start.Before(window.Start) {
				window.Start = s.Start
			}
			if i == 0 || s.End.After(window.End) {
				window.End = s.End
			}
		}
		if window.End.After(q.Now) {
			want := Interval{window.Start.Add(-q.Inspection), window.End.Add(q.Inspection)}
			for _, u := range q.Units {
				if free(want, u.Busy, q.Blackouts, "", u.ID) {
					day.Remaining++
				}
			}
			day.Available = !day.Closed && day.Remaining >= max(q.Quantity, 1)
		}
		out = append(out, day)
	}
	return out
}

// OpenOn says whether the business opens on the local date of t (any shift that weekday).
func OpenOn(loc *time.Location, t time.Time, hours []Shift) bool {
	l := t.In(loc)
	return len(shiftsOn(loc, time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc), hours)) > 0
}

// ClosedDaysBetween counts the local dates strictly between start's and end's dates on which the
// business doesn't open (e.g. the Sunday of a Saturday-to-Monday rental).
func ClosedDaysBetween(loc *time.Location, start, end time.Time, hours []Shift) int {
	s, e := start.In(loc), end.In(loc)
	d := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	last := time.Date(e.Year(), e.Month(), e.Day(), 0, 0, 0, 0, loc)
	n := 0
	for ; d.Before(last); d = d.AddDate(0, 0, 1) {
		if len(shiftsOn(loc, d, hours)) == 0 {
			n++
		}
	}
	return n
}
