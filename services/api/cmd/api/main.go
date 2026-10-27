package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // business time zones must resolve in the distroless image too

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/bookings"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"

	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/httpapi"
	"github.com/omaradriano/local_bussiness/services/api/internal/jobs"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
	"github.com/omaradriano/local_bussiness/services/api/internal/staff"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("api", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	port := env("PORT", "8080") // Railway injects PORT
	supabaseURL := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	dbURL := os.Getenv("DATABASE_URL")
	if supabaseURL == "" || dbURL == "" {
		return errors.New("SUPABASE_URL and DATABASE_URL are required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	keys, err := auth.JWKS(ctx, supabaseURL)
	if err != nil {
		return err
	}
	pool, err := store.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	photos, err := setupPhotos(ctx, log, pool)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr: ":" + port,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Authenticate: auth.Authenticate(keys, supabaseURL+"/auth/v1"),
			Store:        store.New(pool),
			Catalog:      catalog.Repo{DB: pool},
			Staff:        staff.Repo{DB: pool},
			Availability: availability.Loader{DB: pool},
			Bookings:     bookings.Repo{DB: pool},
			CreateBusiness: func(ctx context.Context, userID string, in businesses.Input) (businesses.Business, error) {
				return businesses.Create(ctx, pool, userID, in)
			},
			AllowedOrigins: strings.Split(env("CORS_ORIGINS", "http://localhost:8082"), ","),
			Photos:         photos,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info("api stopping")
	return srv.Shutdown(shutdownCtx)
}

// setupPhotos wires S3 + the job queue. Without S3_BUCKET_PREFIX photos are off (routes answer 503).
func setupPhotos(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool) (*httpapi.Photos, error) {
	prefix := os.Getenv("S3_BUCKET_PREFIX")
	if prefix == "" {
		log.Warn("photos disabled: S3_BUCKET_PREFIX not set")
		return nil, nil
	}
	fs, err := files.New(ctx, env("AWS_REGION", "us-west-2"), prefix)
	if err != nil {
		return nil, err
	}
	// Startup check of the AWS credentials, without ever logging them.
	if err := fs.Ping(ctx, files.Catalog); err != nil {
		log.Error("s3 not reachable", "bucket", prefix+"-"+files.Catalog, "err", err)
	} else {
		log.Info("s3 ok", "bucket", prefix+"-"+files.Catalog)
	}
	mediaRepo := media.Repo{DB: pool}
	queue, err := jobs.NewClient(pool, jobs.Workers(mediaRepo, fs), false)
	if err != nil {
		return nil, err
	}
	return &httpapi.Photos{
		Media: mediaRepo,
		Files: fs,
		EnqueueImage: func(ctx context.Context, imageID string) error {
			_, err := queue.Insert(ctx, jobs.ImageVariantsArgs{ImageID: imageID}, nil)
			return err
		},
		CDNBase: strings.TrimRight(os.Getenv("CATALOG_CDN_URL"), "/"),
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
