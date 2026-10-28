package bookings

import (
	"context"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/steps"
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

func (r Repo) Transition(ctx context.Context, businessID, id, to, actor, note string) (Booking, error) {
	return Transition(ctx, r.DB, businessID, id, to, actor, note)
}

func (r Repo) AddPayment(ctx context.Context, businessID, id string, p Payment, actor string, now time.Time) (Booking, error) {
	return AddPayment(ctx, r.DB, businessID, id, p, actor, now)
}

func (r Repo) StepUpload(ctx context.Context, businessID, bookingID, stepID string) (string, error) {
	return StepUpload(ctx, r.DB, businessID, bookingID, stepID)
}

func (r Repo) CompleteStep(ctx context.Context, businessID, bookingID, stepID string, sub steps.Submission, actor string, now time.Time, exists FileExists) (Booking, error) {
	return CompleteStep(ctx, r.DB, businessID, bookingID, stepID, sub, actor, now, exists)
}

func (r Repo) Checkout(ctx context.Context, businessID, id, condition, actor string, now time.Time) (Booking, error) {
	return Checkout(ctx, r.DB, businessID, id, condition, actor, now)
}

func (r Repo) Return(ctx context.Context, businessID, id, condition, actor string, now time.Time) (Booking, error) {
	return Return(ctx, r.DB, businessID, id, condition, actor, now)
}

func (r Repo) Extend(ctx context.Context, businessID, id string, newEnd time.Time, actor string, now time.Time) (Booking, error) {
	return Extend(ctx, r.DB, businessID, id, newEnd, actor, now)
}
