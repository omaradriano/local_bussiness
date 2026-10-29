package customers

import (
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/privacy"
)

func TestPrivacyAcceptance(t *testing.T) {
	day1 := time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)
	yes, no := true, false

	c := Customer{}.Apply(Input{Name: s("Ana"), Phone: s("6141234567"), PrivacyAccepted: &yes}, day1)
	if c.PrivacyAcceptedAt == nil || !c.PrivacyAcceptedAt.Equal(day1) || c.PrivacyVersion == nil || *c.PrivacyVersion != privacy.Version {
		t.Fatalf("accept: %v %v", c.PrivacyAcceptedAt, c.PrivacyVersion)
	}
	// Accepting the same version again keeps the first date.
	if again := c.Apply(Input{PrivacyAccepted: &yes}, day1.Add(48*time.Hour)); !again.PrivacyAcceptedAt.Equal(day1) {
		t.Fatalf("re-accept moved the date: %v", again.PrivacyAcceptedAt)
	}
	// A newer notice is accepted again with its date.
	old := "2025-01-01"
	c.PrivacyVersion = &old
	if renewed := c.Apply(Input{PrivacyAccepted: &yes}, day1.Add(time.Hour)); *renewed.PrivacyVersion != privacy.Version || !renewed.PrivacyAcceptedAt.Equal(day1.Add(time.Hour)) {
		t.Fatalf("new version: %v %v", renewed.PrivacyAcceptedAt, *renewed.PrivacyVersion)
	}
	// Unticking or not sending it doesn't erase the record (withdrawing consent is an ARCO request).
	for _, in := range []Input{{PrivacyAccepted: &no}, {}} {
		if kept := c.Apply(in, day1); kept.PrivacyAcceptedAt == nil {
			t.Fatal("acceptance erased")
		}
	}
	if none := (Customer{}).Apply(Input{Name: s("Luis")}, day1); none.PrivacyAcceptedAt != nil || none.PrivacyVersion != nil {
		t.Fatal("recorded without accepting")
	}
}
