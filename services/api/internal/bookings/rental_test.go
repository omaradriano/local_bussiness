package bookings

import (
	"testing"
	"time"
)

func TestLateDays(t *testing.T) {
	due := time.Date(2026, 11, 4, 15, 0, 0, 0, time.UTC)
	h := time.Hour
	cases := []struct {
		returned time.Duration // after due
		want     int64
	}{
		{-3 * h, 0},           // early
		{0, 0},                // on time
		{59 * time.Minute, 0}, // within the hour of grace
		{2 * h, 1},            // a started day counts
		{25 * h, 1},           // 24 h after the grace: still one day
		{26 * h, 2},
		{49 * h, 2},
		{50 * h, 3},
	}
	for _, tc := range cases {
		if got := LateDays(due, due.Add(tc.returned)); got != tc.want {
			t.Errorf("returned %v after due: %d days, want %d", tc.returned, got, tc.want)
		}
	}
}

func TestFileKeysStayInsideTheBooking(t *testing.T) {
	k := NewFileKey("biz", "book", "jpg")
	if len(k) < len(FilePrefix("biz", "book"))+5 || k[:len("biz/bookings/book/")] != "biz/bookings/book/" {
		t.Fatalf("key %q", k)
	}
	if NewFileKey("biz", "book", "jpg") == k {
		t.Fatal("keys must be unique")
	}
}

// With «el día de regreso cuenta completo» a return any time on the due day isn't late.
func TestLateDaysByCalendarDay(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chihuahua")
	due := time.Date(2026, 11, 4, 14, 0, 0, 0, loc)
	cases := []struct {
		returned time.Time
		want     int64
	}{
		{due.Add(-3 * time.Hour), 0},
		{time.Date(2026, 11, 4, 23, 59, 0, 0, loc), 0},
		{time.Date(2026, 11, 5, 0, 30, 0, 0, loc), 1},
		{time.Date(2026, 11, 6, 9, 0, 0, 0, loc), 2},
	}
	for _, tc := range cases {
		if got := LateDaysByDay(loc, due, tc.returned); got != tc.want {
			t.Errorf("returned %v: %d days, want %d", tc.returned, got, tc.want)
		}
	}
}
