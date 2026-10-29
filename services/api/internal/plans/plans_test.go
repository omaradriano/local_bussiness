package plans_test

import (
	"slices"
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/features"
	. "github.com/omaradriano/local_bussiness/services/api/internal/plans"
)

var now = time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)

func TestCatalogIsComplete(t *testing.T) {
	for _, key := range Keys {
		def := Get(key)
		if def.Limits.Businesses == nil || *def.Limits.Businesses < 1 || def.Limits.PhotosPerItem == nil {
			t.Errorf("%s: businesses and photos per item need a limit: %+v", key, def.Limits)
		}
		for _, f := range def.ExcludedFeatures {
			if !slices.Contains(features.Known, f) {
				t.Errorf("%s excludes unknown feature %q", key, f)
			}
		}
		for _, c := range def.Capabilities {
			if !slices.Contains(Capabilities, c) {
				t.Errorf("%s has unknown capability %q", key, c)
			}
		}
	}
	if TrialDays != 14 || Get(Trial).Limits != Get(Pro).Limits {
		t.Fatalf("the trial is %d days of %+v", TrialDays, Get(Trial).Limits)
	}
}

func TestAccountStatus(t *testing.T) {
	trial := NewTrial(now)
	if trial.Plan != Trial || !trial.EndsAt.Equal(now.AddDate(0, 0, 14)) || trial.Status(now) != StatusTrial || !trial.Writable(now) {
		t.Fatalf("new trial: %+v %s", trial, trial.Status(now))
	}
	if trial.DaysLeft(now) != 14 || trial.DaysLeft(now.Add(13*24*time.Hour+time.Hour)) != 1 {
		t.Fatalf("days left: %d / %d", trial.DaysLeft(now), trial.DaysLeft(now.Add(13*24*time.Hour+time.Hour)))
	}
	after := now.AddDate(0, 0, 15)
	if trial.Status(after) != StatusExpired || trial.Writable(after) || trial.DaysLeft(after) != 0 {
		t.Fatalf("ended trial: %s", trial.Status(after))
	}
	pilot := Account{Plan: Pro} // no end: pilot or courtesy
	if pilot.Status(after) != StatusActive || !pilot.Writable(after) || pilot.DaysLeft(after) != -1 {
		t.Fatalf("pilot: %s", pilot.Status(after))
	}
	end := now.AddDate(0, 1, 0)
	paid := Account{Plan: Lite, EndsAt: &end}
	if paid.Status(now) != StatusActive || paid.Status(end.Add(time.Second)) != StatusExpired {
		t.Fatal("paid until a date")
	}
}

func TestValidPlanChange(t *testing.T) {
	for _, p := range []string{Trial, Lite, Pro, Business} {
		if !Valid(p) {
			t.Errorf("%s should be valid", p)
		}
	}
	if Valid("free") {
		t.Error("there is no free plan")
	}
}

func TestLimits(t *testing.T) {
	lite := Get(Lite)
	if n := lite.Limit("catalog_items"); n == nil || *n != 30 {
		t.Fatalf("lite catalog: %v", n)
	}
	if Get(Business).Limit("staff") != nil {
		t.Fatal("business has unlimited staff")
	}
	if Get(Lite).Limit("nonsense") != nil {
		t.Fatal("unknown limits don't limit")
	}
	thirty := 30
	if !HasRoom(nil, 1000) || !HasRoom(&thirty, 29) || HasRoom(&thirty, 30) {
		t.Fatal("room")
	}
	if got := LimitMessage(Lite, "catalog_items", 30); got != "Tu plan Lite permite hasta 30 artículos en el catálogo." {
		t.Fatalf("message: %q", got)
	}
	if got := LimitMessage(Trial, "staff", 10); got != "Tu prueba (plan Pro) permite hasta 10 personas que atienden." {
		t.Fatalf("trial message: %q", got)
	}
}

func TestCapabilities(t *testing.T) {
	if Get(Lite).Can("payroll") || !Get(Lite).Can("expenses") || !Get(Trial).Can("recurring_expenses") || !Get(Business).Can("compare_businesses") {
		t.Fatal("capabilities")
	}
	day := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	if !IsCalendarMonth(day("2026-02-01"), day("2026-02-28")) || IsCalendarMonth(day("2026-02-01"), day("2026-02-27")) ||
		IsCalendarMonth(day("2026-01-01"), day("2026-12-31")) || IsCalendarMonth(day("2026-02-02"), day("2026-02-28")) {
		t.Fatal("calendar month")
	}
}
