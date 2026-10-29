package settings

import "testing"

func TestPrivacySection(t *testing.T) {
	s := From([]byte(`{"privacy_legal_name": "Ana López", "privacy_email": "ana@correo.mx"}`))
	if s.Privacy.LegalName != "Ana López" || s.Privacy.Email != "ana@correo.mx" || s.Privacy.Address != "" {
		t.Fatalf("from: %+v", s.Privacy)
	}
	next, errs := s.Apply(Patch{Privacy: &PrivacyPatch{Address: ptr("  Calle 1   #2,\nChihuahua  "), Email: ptr(" ANA@Correo.mx ")}})
	if len(errs) != 0 || next.Privacy.Address != "Calle 1 #2, Chihuahua" || next.Privacy.Email != "ana@correo.mx" || next.Privacy.LegalName != "Ana López" {
		t.Fatalf("apply: %+v %v", next.Privacy, errs)
	}
	for name, p := range map[string]PrivacyPatch{
		"correo sin @":     {Email: ptr("ana.correo.mx")},
		"nombre muy largo": {LegalName: ptr(string(make([]rune, 121)))},
	} {
		if _, errs := s.Apply(Patch{Privacy: &p}); len(errs) == 0 {
			t.Fatalf("%s: accepted", name)
		}
	}
	// Empty is valid: the notice falls back to the business's name and WhatsApp.
	if _, errs := s.Apply(Patch{Privacy: &PrivacyPatch{LegalName: ptr(""), Email: ptr("")}}); len(errs) != 0 {
		t.Fatalf("empty: %v", errs)
	}
}

func TestNoticeFallsBackToTheBusiness(t *testing.T) {
	wa := "+526141234567"
	n := NoticeFrom("Barber360", &wa, []byte(`{"privacy_address": "Calle 1"}`))
	if n.LegalName != "Barber360" || n.Phone != wa || n.Address != "Calle 1" || n.Email != "" || n.Version == "" {
		t.Fatalf("notice: %+v", n)
	}
	if n := NoticeFrom("Barber360", nil, []byte(`{"privacy_legal_name": "Juan Pérez"}`)); n.LegalName != "Juan Pérez" || n.Phone != "" {
		t.Fatalf("legal name: %+v", n)
	}
}
