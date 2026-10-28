package bookings

import (
	"context"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// Repo adapts the package functions to the API's interface.
type Repo struct{ DB store.DB }

func (r Repo) Create(ctx context.Context, businessID string, in CreateInput, now time.Time) (Booking, bool, error) {
	return Create(ctx, r.DB, businessID, in, now)
}

func (r Repo) Get(ctx context.Context, businessID, id string) (Booking, error) {
	return Get(ctx, r.DB, businessID, id)
}

func (r Repo) List(ctx context.Context, businessID string, f ListFilter) ([]Summary, error) {
	return List(ctx, r.DB, businessID, f)
}
