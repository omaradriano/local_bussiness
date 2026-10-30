package engines

import (
	"errors"
	"slices"
	"testing"
)

func TestGetFailsClosedOnUnknownKind(t *testing.T) {
	table := map[string]int{Service: 1}
	if v, err := Get(table, Service); err != nil || v != 1 {
		t.Fatalf("Get(service) = %v, %v", v, err)
	}
	var unknown ErrUnknown
	if _, err := Get(table, "membership"); !errors.As(err, &unknown) || unknown.Kind != "membership" {
		t.Fatalf("Get(membership) err = %v, want ErrUnknown", err)
	}
}

func TestCoversReportsMissingAndExtra(t *testing.T) {
	missing, extra := Covers(map[string]bool{Service: true, Rental: true, "sale": true})
	if !slices.Equal(missing, []string{Product}) || !slices.Equal(extra, []string{"sale"}) {
		t.Fatalf("missing %v, extra %v", missing, extra)
	}
	full := map[string]bool{}
	for _, k := range Kinds {
		full[k] = true
	}
	if missing, extra := Covers(full); missing != nil || extra != nil {
		t.Fatalf("full table: missing %v, extra %v", missing, extra)
	}
}
