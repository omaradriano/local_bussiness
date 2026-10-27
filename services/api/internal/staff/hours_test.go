package staff

import "testing"

func TestValidateHours(t *testing.T) {
	cases := []struct {
		name  string
		hours []Hours
		valid bool
	}{
		{"vacío (sin horario)", nil, true},
		{"lunes a sábado", []Hours{{1, "10:00", "19:00"}, {6, "10:00", "14:00"}}, true},
		{"turno partido", []Hours{{1, "09:00", "14:00"}, {1, "16:00", "20:00"}}, true},
		{"turnos contiguos", []Hours{{1, "09:00", "14:00"}, {1, "14:00", "20:00"}}, true},
		{"día inválido", []Hours{{7, "10:00", "19:00"}}, false},
		{"cierra antes de abrir", []Hours{{1, "19:00", "10:00"}}, false},
		{"misma hora", []Hours{{1, "10:00", "10:00"}}, false},
		{"formato inválido", []Hours{{1, "10am", "19:00"}}, false},
		{"hora imposible", []Hours{{1, "25:00", "26:00"}}, false},
		{"turnos encimados", []Hours{{1, "09:00", "14:00"}, {1, "13:00", "20:00"}}, false},
		{"más de 4 turnos en un día", []Hours{{2, "08:00", "09:00"}, {2, "10:00", "11:00"}, {2, "12:00", "13:00"}, {2, "14:00", "15:00"}, {2, "16:00", "17:00"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := ValidateHours(tc.hours)
			if tc.valid && msg != "" {
				t.Fatalf("want valid, got %q", msg)
			}
			if !tc.valid && msg == "" {
				t.Fatal("want error, got valid")
			}
		})
	}
}

func TestValidateStaff(t *testing.T) {
	if errs := (Member{Name: "Karla", Color: "#E91E63"}).Validate(); len(errs) != 0 {
		t.Fatalf("valid staff rejected: %v", errs)
	}
	if errs := (Member{Name: "  "}).Validate(); errs["name"] == "" {
		t.Error("empty name accepted")
	}
	if errs := (Member{Name: "Karla", Color: "rojo"}).Validate(); errs["color"] == "" {
		t.Error("bad color accepted")
	}
}
