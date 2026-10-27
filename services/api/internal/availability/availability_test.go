package availability

import (
	"testing"
	"time"
)

var chih = mustLoc("America/Chihuahua") // UTC-6, sin horario de verano

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

// local builds an instant in the business time zone.
func local(day string, hhmm string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", day+" "+hhmm, chih)
	if err != nil {
		panic(err)
	}
	return t
}

// Monday 2026-11-02.
const monday = "2026-11-02"

func weekdays(open, close string, days ...int) []Shift {
	var out []Shift
	for _, d := range days {
		out = append(out, Shift{Weekday: d, Opens: open, Closes: close})
	}
	return out
}

func starts(slots []Slot) []string {
	var out []string
	for _, s := range slots {
		out = append(out, s.Start.In(chih).Format("15:04"))
	}
	return out
}

func TestF3_1_ServiceSlots(t *testing.T) {
	karla := Staff{ID: "karla", Hours: weekdays("10:00", "13:00", 1)}
	mayra := Staff{ID: "mayra", Hours: weekdays("12:00", "14:00", 1)}
	sinTurno := Staff{ID: "sin-turno"} // no tiene horario propio: usa el del negocio
	base := ServiceQuery{
		Loc:           chih,
		Date:          monday,
		Now:           local("2026-11-01", "12:00"),
		Duration:      60 * time.Minute,
		Buffer:        0,
		Step:          30 * time.Minute,
		BusinessHours: weekdays("09:00", "11:00", 1),
		Staff:         []Staff{karla},
	}

	cases := []struct {
		name string
		mut  func(*ServiceQuery)
		want []string
	}{
		{"turno de la empleada, cada 30 min, termina antes del cierre", func(*ServiceQuery) {}, []string{"10:00", "10:30", "11:00", "11:30", "12:00"}},
		{"fuera de turno: día que no trabaja", func(q *ServiceQuery) { q.Date = "2026-11-03" }, nil},
		{"duración que no cabe en el turno", func(q *ServiceQuery) { q.Duration = 4 * time.Hour }, nil},
		{"empleada ocupada de 10:30 a 11:30", func(q *ServiceQuery) {
			q.Staff = []Staff{{ID: "karla", Hours: karla.Hours, Busy: []Interval{{local(monday, "10:30"), local(monday, "11:30")}}}}
		}, []string{"11:30", "12:00"}},
		{"cita contigua: termina a las 10:00 y otra empieza a las 10:00", func(q *ServiceQuery) {
			q.Staff = []Staff{{ID: "karla", Hours: karla.Hours, Busy: []Interval{{local(monday, "09:00"), local(monday, "10:00")}}}}
		}, []string{"10:00", "10:30", "11:00", "11:30", "12:00"}},
		{"margen entre citas también ocupa", func(q *ServiceQuery) {
			q.Buffer = 15 * time.Minute
			q.Staff = []Staff{{ID: "karla", Hours: karla.Hours, Busy: []Interval{{local(monday, "11:45"), local(monday, "12:30")}}}}
		}, []string{"10:00", "10:30"}}, // 12:30 + 60 min ends after closing
		{"blackout del negocio", func(q *ServiceQuery) {
			q.Blackouts = []Blackout{{Interval: Interval{local(monday, "11:00"), local(monday, "12:00")}}}
		}, []string{"10:00", "12:00"}},
		{"blackout de otra empleada no afecta", func(q *ServiceQuery) {
			q.Blackouts = []Blackout{{StaffID: "mayra", Interval: Interval{local(monday, "00:00"), local(monday, "23:59")}}}
		}, []string{"10:00", "10:30", "11:00", "11:30", "12:00"}},
		{"vacaciones de la empleada", func(q *ServiceQuery) {
			q.Blackouts = []Blackout{{StaffID: "karla", Interval: Interval{local("2026-10-30", "00:00"), local("2026-11-05", "00:00")}}}
		}, nil},
		{"nada en el pasado", func(q *ServiceQuery) { q.Now = local(monday, "11:10") }, []string{"11:30", "12:00"}},
		{"sin empleada propia usa el horario del negocio", func(q *ServiceQuery) { q.Staff = []Staff{sinTurno} }, []string{"09:00", "09:30", "10:00"}},
		{"dos empleadas: se juntan los horarios sin repetir", func(q *ServiceQuery) { q.Staff = []Staff{karla, mayra} }, []string{"10:00", "10:30", "11:00", "11:30", "12:00", "12:30", "13:00"}},
		{"sin empleadas que hagan el servicio", func(q *ServiceQuery) { q.Staff = nil }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := base
			tc.mut(&q)
			got := starts(ServiceSlots(q))
			if !equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestF3_1_ServiceSlotsListStaffPerSlot(t *testing.T) {
	q := ServiceQuery{
		Loc: chih, Date: monday, Now: local("2026-11-01", "12:00"), Duration: time.Hour, Step: time.Hour,
		Staff: []Staff{{ID: "karla", Hours: weekdays("10:00", "12:00", 1)}, {ID: "mayra", Hours: weekdays("11:00", "13:00", 1)}},
	}
	slots := ServiceSlots(q)
	if len(slots) != 3 || !equal(slots[1].StaffIDs, []string{"karla", "mayra"}) || !equal(slots[2].StaffIDs, []string{"mayra"}) {
		t.Fatalf("got %+v", slots)
	}
	if !slots[0].End.Equal(local(monday, "11:00")) {
		t.Fatalf("end = %v", slots[0].End)
	}
}

// The business day is the local date: 23:00 in Chihuahua is already the next day in UTC.
func TestF3_1_CambioDeDiaEnZonaHoraria(t *testing.T) {
	q := ServiceQuery{
		Loc: chih, Date: monday, Now: local("2026-11-01", "12:00"), Duration: 30 * time.Minute, Step: 30 * time.Minute,
		Staff: []Staff{{ID: "karla", Hours: weekdays("22:00", "23:30", 1)}},
	}
	slots := ServiceSlots(q)
	if got := starts(slots); !equal(got, []string{"22:00", "22:30", "23:00"}) {
		t.Fatalf("got %v", got)
	}
	if slots[2].Start.UTC().Format("2006-01-02 15:04") != "2026-11-03 05:00" {
		t.Fatalf("23:00 local should be 05:00 UTC of the next day, got %v", slots[2].Start.UTC())
	}
	// A booking stored for Tuesday 01:00 UTC is Monday 19:00 local: it must not block Monday 22:00.
	q.Staff[0].Busy = []Interval{{time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC), time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)}}
	if got := starts(ServiceSlots(q)); len(got) != 3 {
		t.Fatalf("an earlier local booking blocked late slots: %v", got)
	}
}

func TestF3_1_ProductDays(t *testing.T) {
	base := ProductQuery{
		Loc:           chih,
		From:          monday,
		To:            "2026-11-04",
		Now:           local("2026-11-01", "12:00"),
		PrepHours:     24,
		Quantity:      1,
		BusinessHours: weekdays("09:00", "20:00", 1, 2, 3),
		DailyCapacity: 10,
	}
	cases := []struct {
		name string
		mut  func(*ProductQuery)
		want map[string]int // date -> remaining (absent = not available)
	}{
		{"todos los días abiertos con cupo", func(*ProductQuery) {}, map[string]int{monday: 10, "2026-11-03": 10, "2026-11-04": 10}},
		{"anticipación: el lunes a las 12 no da para entregar ese mismo día", func(q *ProductQuery) { q.Now = local(monday, "12:00") },
			map[string]int{"2026-11-03": 10, "2026-11-04": 10}},
		{"anticipación justa: entrega posible antes del cierre", func(q *ProductQuery) { q.Now = local("2026-11-01", "19:00") },
			map[string]int{monday: 10, "2026-11-03": 10, "2026-11-04": 10}},
		{"día cerrado", func(q *ProductQuery) { q.BusinessHours = weekdays("09:00", "20:00", 1, 3) }, map[string]int{monday: 10, "2026-11-04": 10}},
		{"cupo diario lleno", func(q *ProductQuery) { q.Booked = map[string]Booked{monday: {All: 10}} }, map[string]int{"2026-11-03": 10, "2026-11-04": 10}},
		{"cupo del artículo más estricto que el global", func(q *ProductQuery) {
			q.ItemCapacity = 3
			q.Booked = map[string]Booked{monday: {All: 4, Item: 2}}
		}, map[string]int{monday: 1, "2026-11-03": 3, "2026-11-04": 3}},
		{"piden más de lo que queda", func(q *ProductQuery) {
			q.Quantity = 2
			q.Booked = map[string]Booked{monday: {All: 9}}
		}, map[string]int{"2026-11-03": 10, "2026-11-04": 10}},
		{"blackout del negocio todo el día", func(q *ProductQuery) {
			q.Blackouts = []Blackout{{Interval: Interval{local("2026-11-03", "00:00"), local("2026-11-04", "00:00")}}}
		}, map[string]int{monday: 10, "2026-11-04": 10}},
		{"sin cupo configurado: no hay límite", func(q *ProductQuery) { q.DailyCapacity = 0; q.Booked = map[string]Booked{monday: {All: 500}} },
			map[string]int{monday: -1, "2026-11-03": -1, "2026-11-04": -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := base
			tc.mut(&q)
			got := map[string]int{}
			for _, d := range ProductDays(q) {
				if d.Available {
					got[d.Date] = d.Remaining
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestF3_1_RentalUnits(t *testing.T) {
	from, to := local(monday, "09:00"), local("2026-11-04", "09:00")
	base := RentalQuery{
		From: from, To: to, Now: local("2026-11-01", "12:00"), Quantity: 1, Inspection: time.Hour,
		Units: []Unit{{ID: "rm-01"}, {ID: "rm-02"}},
	}
	cases := []struct {
		name string
		mut  func(*RentalQuery)
		want []string
	}{
		{"dos libres", func(*RentalQuery) {}, []string{"rm-01", "rm-02"}},
		{"una rentada en el rango", func(q *RentalQuery) {
			q.Units[0].Busy = []Interval{{local("2026-11-03", "09:00"), local("2026-11-05", "09:00")}}
		}, []string{"rm-02"}},
		{"sin unidades libres", func(q *RentalQuery) {
			q.Quantity = 1
			for i := range q.Units {
				q.Units[i].Busy = []Interval{{local(monday, "08:00"), local("2026-11-10", "08:00")}}
			}
		}, nil},
		{"margen de revisión: regresa a las 8:30 y sale a las 9:00 no alcanza", func(q *RentalQuery) {
			q.Units = []Unit{{ID: "rm-01", Busy: []Interval{{local("2026-10-30", "09:00"), local(monday, "08:30")}}}}
		}, nil},
		{"margen de revisión cumplido", func(q *RentalQuery) {
			q.Units = []Unit{{ID: "rm-01", Busy: []Interval{{local("2026-10-30", "09:00"), local(monday, "08:00")}}}}
		}, []string{"rm-01"}},
		{"unidad en reparación", func(q *RentalQuery) {
			q.Blackouts = []Blackout{{UnitID: "rm-02", Interval: Interval{local(monday, "00:00"), local("2026-11-02", "23:00")}}}
		}, []string{"rm-01"}},
		{"cierre del negocio en el rango", func(q *RentalQuery) {
			q.Blackouts = []Blackout{{Interval: Interval{local("2026-11-03", "00:00"), local("2026-11-03", "23:59")}}}
		}, nil},
		{"piden más unidades de las libres", func(q *RentalQuery) { q.Quantity = 3 }, []string{"rm-01", "rm-02"}},
		{"rango en el pasado", func(q *RentalQuery) { q.Now = local(monday, "10:00") }, nil},
		{"regreso antes de la salida", func(q *RentalQuery) { q.From, q.To = q.To, q.From }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := base
			q.Units = append([]Unit(nil), base.Units...)
			tc.mut(&q)
			got := RentalFreeUnits(q)
			if !equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
