package jobs

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
)

type memFiles map[string][]byte

func (m memFiles) Get(_ context.Context, bucket, key string, _ int64) ([]byte, error) {
	b, ok := m[bucket+"/"+key]
	if !ok {
		return nil, files.ErrNoObject
	}
	return b, nil
}

func (m memFiles) PutImmutable(_ context.Context, bucket, key, _ string, data []byte) error {
	m[bucket+"/"+key] = data
	return nil
}

func jpg(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func job(id string) *river.Job[ImageVariantsArgs] {
	return &river.Job[ImageVariantsArgs]{JobRow: &rivertype.JobRow{}, Args: ImageVariantsArgs{ImageID: id}}
}

func TestIntegrationImageVariantsJob(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	const biz, item = "00000000-0000-0000-0000-00000000000a", "00000000-0000-0000-0000-0000000000c1"
	for _, q := range []string{
		`insert into public.businesses (id, slug, name, vertical) values ('` + biz + `', 'jobs-a', 'A', 'nails')`,
		`insert into public.catalog_items (id, business_id, kind, name, price, duration_minutes) values ('` + item + `', '` + biz + `', 'service', 'Gelish', 25000, 60)`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	repo := media.Repo{DB: tx}
	fs := memFiles{}
	w := &ImageVariantsWorker{Media: repo, Files: fs}

	// A real photo becomes ready with its three variants stored next to the original.
	good, _ := repo.CreatePending(ctx, biz, item, "image/jpeg")
	fs[files.Catalog+"/"+good.Key()] = jpg(t)
	if err := w.Work(ctx, job(good.ID)); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(ctx, good.ID)
	if got.Status != "ready" || got.Blurhash == nil || len(fs) != 4 {
		t.Fatalf("after work: %+v, files=%d", got, len(fs))
	}
	// Running it again (River retry) is a no-op.
	if err := w.Work(ctx, job(good.ID)); err != nil || len(fs) != 4 {
		t.Fatalf("second run: %v files=%d", err, len(fs))
	}

	// A file that isn't an image is marked failed and the job is cancelled, not retried.
	bad, _ := repo.CreatePending(ctx, biz, item, "image/png")
	fs[files.Catalog+"/"+bad.Key()] = []byte("not an image")
	err = w.Work(ctx, job(bad.ID))
	var cancel *river.JobCancelError
	if !errorAs(err, &cancel) {
		t.Fatalf("want JobCancel, got %v", err)
	}
	if b, _ := repo.GetByID(ctx, bad.ID); b.Status != "failed" {
		t.Fatalf("bad image status = %s", b.Status)
	}

	// A deleted image is a no-op.
	if err := w.Work(ctx, job("00000000-0000-0000-0000-00000000dead")); err != nil {
		t.Fatalf("deleted image: %v", err)
	}
}

func errorAs(err error, target any) bool { return err != nil && errors.As(err, target) }

type fakeDeleter struct{ calls []string }

func (f *fakeDeleter) DeletePrefix(_ context.Context, bucket, prefix string) (int, error) {
	f.calls = append(f.calls, bucket+":"+prefix)
	return 1, nil
}

func TestPurgeBusinessFiles(t *testing.T) {
	f := &fakeDeleter{}
	const biz = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	if err := PurgeBusinessFiles(context.Background(), f, biz); err != nil {
		t.Fatal(err)
	}
	want := []string{"catalog:" + biz + "/", "private-docs:" + biz + "/", "rental-evidence:" + biz + "/"}
	if !slices.Equal(f.calls, want) {
		t.Fatalf("calls: %v", f.calls)
	}
	// Anything but a business id never reaches S3 (an empty prefix would be the whole bucket).
	for _, bad := range []string{"", "/", "../x", "aaaaaaaa"} {
		f.calls = nil
		if err := PurgeBusinessFiles(context.Background(), f, bad); err == nil || len(f.calls) != 0 {
			t.Fatalf("%q: err=%v calls=%v", bad, err, f.calls)
		}
	}
}

func (m memFiles) DeletePrefix(context.Context, string, string) (int, error) { return 0, nil }
