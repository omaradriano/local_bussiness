package bookings

import (
	"errors"
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
)

func TestEveryEngineIsMadeOfModules(t *testing.T) {
	if missing, extra := engines.Covers(rulesByKind); missing != nil || extra != nil {
		t.Fatalf("rulesByKind: missing %v, extra %v", missing, extra)
	}
	for kind, r := range rulesByKind {
		if r.timing == nil || r.fulfillment == nil || r.resource == nil || r.pricing == nil {
			t.Errorf("%s: every module is required", kind)
		}
	}
}

func invalidField(err error) string {
	var inv Invalid
	if errors.As(err, &inv) {
		return inv.Field
	}
	return ""
}

func TestTimingModules(t *testing.T) {
	start := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)
	r := &reservation{in: &CreateInput{}, item: itemRow{DurationMinutes: 45}, start: start}
	if err := slotTiming(r); err != nil || !r.end.Equal(start.Add(45*time.Minute)) {
		t.Fatalf("slot: end %v, err %v", r.end, err)
	}
	r = &reservation{in: &CreateInput{}, start: start}
	if err := dayTiming(r); err != nil || r.end != nil {
		t.Fatalf("day: end %v, err %v", r.end, err)
	}
	if err := rangeTiming(r); invalidField(err) != "ends_at" {
		t.Fatalf("range without return: %v", err)
	}
	before := start.Add(-time.Hour)
	r.in.EndsAt = &before
	if err := rangeTiming(r); invalidField(err) != "ends_at" {
		t.Fatalf("range with return before pick-up: %v", err)
	}
	after := start.Add(48 * time.Hour)
	r.in.EndsAt = &after
	if err := rangeTiming(r); err != nil || !r.end.Equal(after) {
		t.Fatalf("range: end %v, err %v", r.end, err)
	}
}

func TestFulfillmentModules(t *testing.T) {
	cases := []struct {
		name   string
		module func(*reservation) error
		in     CreateInput
		active []string
		zones  []zone
		field  string // expected Invalid field, "" = ok
		want   string // fulfillment after the module
		fee    int64
	}{
		{"at place clears delivery", atPlace, CreateInput{Fulfillment: "delivery", DeliveryAddress: "x"}, nil, nil, "", "", 0},
		{"service at the business", atPlaceOrHome, CreateInput{Fulfillment: "pickup"}, nil, nil, "", "", 0},
		{"home service needs the feature", atPlaceOrHome, CreateInput{Fulfillment: "delivery", DeliveryAddress: "x"}, nil, nil, "fulfillment", "", 0},
		{"home service needs an address", atPlaceOrHome, CreateInput{Fulfillment: "delivery"}, []string{"homeService"}, nil, "delivery_address", "", 0},
		{"home service", atPlaceOrHome, CreateInput{Fulfillment: "delivery", DeliveryAddress: " Calle 1 "}, []string{"homeService"}, nil, "", "delivery", 0},
		{"pickup by default", pickupOrDelivery, CreateInput{}, nil, nil, "", "pickup", 0},
		{"unknown fulfillment", pickupOrDelivery, CreateInput{Fulfillment: "drone"}, nil, nil, "fulfillment", "", 0},
		{"delivery needs a zone", pickupOrDelivery, CreateInput{Fulfillment: "delivery", DeliveryAddress: "x", DeliveryZone: "Sur"}, nil, []zone{{"Norte", 5000}}, "delivery_zone", "", 0},
		{"delivery charges the zone", pickupOrDelivery, CreateInput{Fulfillment: "delivery", DeliveryAddress: "x", DeliveryZone: "Norte"}, nil, []zone{{"Norte", 5000}}, "", "delivery", 5000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := c.in
			r := &reservation{in: &in, active: c.active, set: bizSettings{DeliveryZones: c.zones}}
			err := c.module(r)
			if got := invalidField(err); got != c.field || (c.field == "" && err != nil) {
				t.Fatalf("err %v, want field %q", err, c.field)
			}
			if c.field == "" && (in.Fulfillment != c.want || r.deliveryFee != c.fee) {
				t.Fatalf("fulfillment %q fee %d, want %q %d", in.Fulfillment, r.deliveryFee, c.want, c.fee)
			}
		})
	}
}

func TestPricingModules(t *testing.T) {
	start := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)
	if n, err := fixedPrice(periodQuery{}); n != 1 || err != nil {
		t.Fatalf("fixed: %d, %v", n, err)
	}
	if _, err := perPeriod(periodQuery{start: start}); invalidField(err) != "ends_at" {
		t.Fatalf("per period without return: %v", err)
	}
	open := func() ([]availability.Shift, error) { // open every day
		var s []availability.Shift
		for d := range 7 {
			s = append(s, availability.Shift{Weekday: d, Opens: "09:00", Closes: "18:00"})
		}
		return s, nil
	}
	end := start.Add(72 * time.Hour)
	if n, err := perPeriod(periodQuery{loc: time.UTC, hours: open, priceUnit: "day", start: start, end: &end}); n != 3 || err != nil {
		t.Fatalf("per day, 3 days: %d, %v", n, err)
	}
}

func TestCustodyModule(t *testing.T) {
	if !hasCustody(engines.Rental) || hasCustody(engines.Service) || hasCustody(engines.Product) || hasCustody("unknown") {
		t.Fatal("only rentals go with the customer and come back")
	}
	// Without custody there are no deposit or late-fee timing rules.
	if err := PaymentAllowed(engines.Service, "confirmed", "late_fee"); err != nil {
		t.Fatalf("service late_fee: %v", err)
	}
	if err := PaymentAllowed(engines.Rental, "confirmed", "late_fee"); invalidField(err) != "type" {
		t.Fatalf("rental late_fee before the return: %v", err)
	}
}
