// Package jobs defines the background jobs (River, schema "river") and builds the clients:
// the API only inserts jobs, the worker runs them.
package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/images"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
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

func Workers(m media.Repo, f FileStore) *river.Workers {
	ws := river.NewWorkers()
	river.AddWorker(ws, &ImageVariantsWorker{Media: m, Files: f})
	return ws
}

// NewClient: with run=false it only inserts (API); with run=true it also works the default queue (worker).
func NewClient(pool *pgxpool.Pool, workers *river.Workers, run bool) (*river.Client[pgx.Tx], error) {
	cfg := &river.Config{Schema: "river", Workers: workers}
	if run {
		cfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}}
	}
	return river.NewClient(riverpgxv5.New(pool), cfg)
}
