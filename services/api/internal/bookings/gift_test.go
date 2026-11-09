package bookings

import (
	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"strings"
	"testing"
	"time"
)

func TestGiftDetails(t *testing.T) {
	gift := []string{"giftOrder"}
	cases := []struct {
		name   string
		in     CreateInput
		active []string
		field  string // expected Invalid field, "" = ok
		want   CreateInput
	}{
		{"without the feature they're dropped", CreateInput{RecipientName: "Ana", RecipientPhone: "6141234567", CardMessage: "Te quiero"}, nil, "", CreateInput{}},
		{"all optional", CreateInput{}, gift, "", CreateInput{}},
		{"trimmed and phone normalized", CreateInput{RecipientName: " Ana ", RecipientPhone: "614 123 4567", CardMessage: " Feliz día "}, gift, "",
			CreateInput{RecipientName: "Ana", RecipientPhone: "+526141234567", CardMessage: "Feliz día"}},
		{"bad phone", CreateInput{RecipientPhone: "123"}, gift, "recipient_phone", CreateInput{}},
		{"long name", CreateInput{RecipientName: strings.Repeat("a", 81)}, gift, "recipient_name", CreateInput{}},
		{"long card", CreateInput{CardMessage: strings.Repeat("a", 501)}, gift, "card_message", CreateInput{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := c.in
			err := giftDetails(&in, c.active)
			if got := invalidField(err); got != c.field || (c.field == "" && err != nil) {
				t.Fatalf("err %v, want field %q", err, c.field)
			}
			if c.field == "" && (in.RecipientName != c.want.RecipientName || in.RecipientPhone != c.want.RecipientPhone || in.CardMessage != c.want.CardMessage) {
				t.Fatalf("got %q %q %q", in.RecipientName, in.RecipientPhone, in.CardMessage)
			}
		})
	}
}

func TestCustomPrice(t *testing.T) {
	custom := []string{"customOrder"}
	p := func(v int64) *int64 { return &v }
	cases := []struct {
		name   string
		item   itemRow
		in     CreateInput
		active []string
		field  string
		price  int64 // item price after the rule
		desc   string
	}{
		{"fixed price ignores what's sent", itemRow{Price: 65000}, CreateInput{CustomPrice: p(1), CustomDescription: "x"}, custom, "", 65000, ""},
		{"free price needs the feature", itemRow{CustomPrice: true}, CreateInput{CustomPrice: p(80000), CustomDescription: "Rosas"}, nil, "item_id", 0, ""},
		{"free price needs a price", itemRow{CustomPrice: true}, CreateInput{CustomDescription: "Rosas"}, custom, "custom_price", 0, ""},
		{"free price above zero", itemRow{CustomPrice: true}, CreateInput{CustomPrice: p(0), CustomDescription: "Rosas"}, custom, "custom_price", 0, ""},
		{"free price has a cap", itemRow{CustomPrice: true}, CreateInput{CustomPrice: p(maxPayment + 1), CustomDescription: "Rosas"}, custom, "custom_price", 0, ""},
		{"free price needs a description", itemRow{CustomPrice: true}, CreateInput{CustomPrice: p(80000), CustomDescription: " "}, custom, "custom_description", 0, ""},
		{"long description", itemRow{CustomPrice: true}, CreateInput{CustomPrice: p(80000), CustomDescription: strings.Repeat("a", 501)}, custom, "custom_description", 0, ""},
		{"free price", itemRow{CustomPrice: true}, CreateInput{CustomPrice: p(80000), CustomDescription: " 24 rosas rojas "}, custom, "", 80000, "24 rosas rojas"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, it := c.in, c.item
			err := customPrice(&in, &it, c.active)
			if got := invalidField(err); got != c.field || (c.field == "" && err != nil) {
				t.Fatalf("err %v, want field %q", err, c.field)
			}
			if c.field == "" && (it.Price != c.price || in.CustomDescription != c.desc) {
				t.Fatalf("price %d desc %q, want %d %q", it.Price, in.CustomDescription, c.price, c.desc)
			}
		})
	}
}

// A day order can carry a time window («de 10 a 13 h») inside the same day.
func TestDayTimingWindow(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chihuahua")
	start := time.Date(2026, 11, 2, 10, 0, 0, 0, loc)
	at := func(d time.Time) *time.Time { return &d }
	cases := []struct {
		name  string
		end   *time.Time
		field string
	}{
		{"no window", nil, ""},
		{"window", at(start.Add(3 * time.Hour)), ""},
		{"ends before it starts", at(start.Add(-time.Hour)), "ends_at"},
		{"ends the next day", at(start.Add(15 * time.Hour)), "starts_at"}, // 01:00: closed by then
		{"window ends after closing", at(start.Add(11 * time.Hour)), "starts_at"},
	}
	monday := []availability.Shift{{Weekday: 1, Opens: "09:00", Closes: "20:00"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &reservation{in: &CreateInput{EndsAt: c.end}, loc: loc, start: start, hours: monday}
			err := dayTiming(r)
			if got := invalidField(err); got != c.field || (c.field == "" && err != nil) {
				t.Fatalf("err %v, want field %q", err, c.field)
			}
			if c.field == "" && r.end != c.end {
				t.Fatalf("end %v, want %v", r.end, c.end)
			}
		})
	}
	// Monday 23:00 and 03:00 with the shop closing at 20:00.
	for _, h := range []int{23, 3} {
		r := &reservation{in: &CreateInput{}, loc: loc, start: time.Date(2026, 11, 2, h, 0, 0, 0, loc), hours: monday}
		if invalidField(dayTiming(r)) != "starts_at" {
			t.Fatalf("%d:00 outside the hours was accepted", h)
		}
	}
}

// A walk-in may leave a name and a phone for this order only (who picks it up); nothing is registered.
func TestWalkInDetails(t *testing.T) {
	c := CustomerInput{WalkIn: true, Name: " Doña Mari ", Phone: "614 123 4567"}
	if err := walkInDetails(&c); err != nil || c.Name != "Doña Mari" || c.Phone != "+526141234567" {
		t.Fatalf("walk-in: %+v %v", c, err)
	}
	c = CustomerInput{WalkIn: true}
	if err := walkInDetails(&c); err != nil || c.Name != "" || c.Phone != "" {
		t.Fatalf("anonymous walk-in: %+v %v", c, err)
	}
	if err := walkInDetails(&CustomerInput{WalkIn: true, Phone: "123"}); invalidField(err) != "customer.phone" {
		t.Fatalf("bad phone: %v", err)
	}
	if err := walkInDetails(&CustomerInput{WalkIn: true, Name: strings.Repeat("a", 81)}); invalidField(err) != "customer.name" {
		t.Fatalf("long name: %v", err)
	}
}
