package privacy

import (
	"testing"
	"time"
)

func TestVersionIsADate(t *testing.T) {
	if _, err := time.Parse("2006-01-02", Version); err != nil {
		t.Fatalf("version %q is not YYYY-MM-DD: %v", Version, err)
	}
}
