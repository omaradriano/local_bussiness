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
