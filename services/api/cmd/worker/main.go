package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/jobs"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// The worker runs the River jobs (schema "river"): today, generate_image_variants.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("worker", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	dbURL, prefix := os.Getenv("DATABASE_URL"), os.Getenv("S3_BUCKET_PREFIX")
	if dbURL == "" || prefix == "" {
		return errors.New("DATABASE_URL and S3_BUCKET_PREFIX are required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2"
	}
	fs, err := files.New(ctx, region, prefix)
	if err != nil {
		return err
	}
	if err := fs.Ping(ctx, files.Catalog); err != nil {
		log.Error("s3 not reachable", "bucket", prefix+"-"+files.Catalog, "err", err)
	} else {
		log.Info("s3 ok", "bucket", prefix+"-"+files.Catalog)
	}

	client, err := jobs.NewClient(pool, jobs.Workers(media.Repo{DB: pool}, fs, pool), true)
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	log.Info("worker started")

	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	log.Info("worker stopping")
	return client.Stop(stopCtx)
}
