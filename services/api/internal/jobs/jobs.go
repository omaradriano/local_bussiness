// Package jobs defines the background jobs (River, schema "river") and builds the clients:
// the API only inserts jobs, the worker runs them.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/omaradriano/local_bussiness/services/api/internal/bookings"
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/images"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// ImageVariantsArgs: generate_image_variants for one catalog photo.
type ImageVariantsArgs struct {
	ImageID string `json:"image_id"`
}

func (ImageVariantsArgs) Kind() string { return "generate_image_variants" }

type FileStore interface {
	Get(ctx context.Context, bucket, key string, maxBytes int64) ([]byte, error)
	PutImmutable(ctx context.Context, bucket, key, contentType string, data []byte) error
}

type ImageVariantsWorker struct {
	river.WorkerDefaults[ImageVariantsArgs]
	Media media.Repo
	Files FileStore
}

// Work is idempotent: a ready or deleted image is a no-op, and variant keys are fixed per image.
func (w *ImageVariantsWorker) Work(ctx context.Context, job *river.Job[ImageVariantsArgs]) error {
	img, err := w.Media.GetByID(ctx, job.Args.ImageID)
	if errors.Is(err, media.ErrNotFound) || (err == nil && img.Status == "ready") {
		return nil
	}
	if err != nil {
		return err
	}
	src, err := w.Files.Get(ctx, files.Catalog, img.Key(), media.MaxUploadBytes)
	if errors.Is(err, files.ErrNoObject) {
		return w.fail(ctx, img.ID, err)
	}
	if err != nil {
		return err // transient: River retries
	}
	res, err := images.Process(src)
	if err != nil {
		return w.fail(ctx, img.ID, err)
	}
	keys := map[string]string{}
	for name, data := range res.Variants {
		key := media.VariantKey(img.Key(), name)
		if err := w.Files.PutImmutable(ctx, files.Catalog, key, "image/webp", data); err != nil {
			return err
		}
		keys[name] = key
	}
	return w.Media.MarkReady(ctx, img.ID, res.Width, res.Height, res.Blurhash, keys)
}

// fail marks the image as failed and stops retrying: the input will never become valid.
func (w *ImageVariantsWorker) fail(ctx context.Context, imageID string, cause error) error {
	if err := w.Media.MarkFailed(ctx, imageID); err != nil {
		return err
	}
	return river.JobCancel(fmt.Errorf("image %s: %w", imageID, cause))
}

// ExpireHoldsArgs / MarkOverdueArgs: periodic sweeps of bookings (see bookings.ExpireHolds / MarkOverdue).
type ExpireHoldsArgs struct{}

func (ExpireHoldsArgs) Kind() string { return "expire_unpaid_holds" }

type MarkOverdueArgs struct{}

func (MarkOverdueArgs) Kind() string { return "mark_overdue_rentals" }

type sweepWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
	DB    store.DB
	Sweep func(ctx context.Context, db store.DB, now time.Time) (int, error)
}

func (w *sweepWorker[T]) Work(ctx context.Context, _ *river.Job[T]) error {
	n, err := w.Sweep(ctx, w.DB, time.Now())
	if n > 0 {
		slog.InfoContext(ctx, "sweep", "kind", (*new(T)).Kind(), "bookings", n)
	}
	return err
}

func Workers(m media.Repo, f FileStore, db store.DB) *river.Workers {
	ws := river.NewWorkers()
	river.AddWorker(ws, &ImageVariantsWorker{Media: m, Files: f})
	river.AddWorker(ws, &sweepWorker[ExpireHoldsArgs]{DB: db, Sweep: bookings.ExpireHolds})
	river.AddWorker(ws, &sweepWorker[MarkOverdueArgs]{DB: db, Sweep: bookings.MarkOverdue})
	return ws
}

// periodic: expire unpaid holds every 5 min, flag late rentals every 15 min (both run at start too).
var periodic = []*river.PeriodicJob{
	river.NewPeriodicJob(river.PeriodicInterval(5*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return ExpireHoldsArgs{}, nil }, &river.PeriodicJobOpts{RunOnStart: true}),
	river.NewPeriodicJob(river.PeriodicInterval(15*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return MarkOverdueArgs{}, nil }, &river.PeriodicJobOpts{RunOnStart: true}),
}

// NewClient: with run=false it only inserts (API); with run=true it also works the default queue (worker).
func NewClient(pool *pgxpool.Pool, workers *river.Workers, run bool) (*river.Client[pgx.Tx], error) {
	cfg := &river.Config{Schema: "river", Workers: workers}
	if run {
		cfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}}
		cfg.PeriodicJobs = periodic
	}
	return river.NewClient(riverpgxv5.New(pool), cfg)
}
