package media

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestValidateUpload(t *testing.T) {
	cases := []struct {
		ct    string
		size  int64
		valid bool
	}{
		{"image/jpeg", 800_000, true},
		{"image/png", MaxUploadBytes, true},
		{"image/webp", 1, true},
		{"image/gif", 1000, false},
		{"application/pdf", 1000, false},
		{"image/jpeg", 0, false},
		{"image/jpeg", MaxUploadBytes + 1, false},
	}
	for _, tc := range cases {
		if got := ValidateUpload(tc.ct, tc.size) == ""; got != tc.valid {
			t.Errorf("ValidateUpload(%q, %d) valid=%v, want %v", tc.ct, tc.size, got, tc.valid)
		}
	}
}

func TestVariantKeyLivesNextToOriginal(t *testing.T) {
	if got := VariantKey("b/catalog/i/img/original.jpg", "card"); got != "b/catalog/i/img/card.webp" {
		t.Fatal(got)
	}
}

func TestIntegrationImagesLifecycle(t *testing.T) {
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

	const bizA, bizB, item = "00000000-0000-0000-0000-00000000000a", "00000000-0000-0000-0000-00000000000b", "00000000-0000-0000-0000-0000000000c1"
	for _, q := range []string{
		`insert into public.businesses (id, slug, name, vertical) values ('` + bizA + `', 'img-a', 'A', 'nails'), ('` + bizB + `', 'img-b', 'B', 'nails')`,
		`insert into public.catalog_items (id, business_id, kind, name, price, duration_minutes) values ('` + item + `', '` + bizA + `', 'service', 'Gelish', 25000, 60)`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	r := Repo{DB: tx}

	img, err := r.CreatePending(ctx, bizA, item, "image/jpeg")
	if err != nil || img.Status != "processing" {
		t.Fatalf("create: %+v %v", img, err)
	}
	wantPrefix := bizA + "/catalog/" + item + "/" + img.ID + "/original.jpg"
	if img.Key() != wantPrefix {
		t.Fatalf("key = %q, want %q", img.Key(), wantPrefix)
	}
	if _, err := r.CreatePending(ctx, bizB, item, "image/jpeg"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other business: %v", err)
	}

	variants := map[string]string{"card": VariantKey(img.Key(), "card")}
	if err := r.MarkReady(ctx, img.ID, 2000, 1500, "LKO2?U%2Tw=w]~RBVZRi};RPxuwH", variants); err != nil {
		t.Fatal(err)
	}
	byItem, err := r.ListForBusiness(ctx, bizA)
	if err != nil || len(byItem[item]) != 1 {
		t.Fatalf("list: %v %v", byItem, err)
	}
	got := byItem[item][0].WithURLs("https://cdn.test")
	if got.Status != "ready" || !strings.HasPrefix(got.URLs["card"], "https://cdn.test/"+bizA+"/") || *got.Width != 2000 {
		t.Fatalf("ready image: %+v", got)
	}

	for i := 1; i < MaxImagesPerItem; i++ {
		if _, err := r.CreatePending(ctx, bizA, item, "image/png"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.CreatePending(ctx, bizA, item, "image/png"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("over the per-item limit: %v", err)
	}

	if _, err := r.Delete(ctx, bizB, item, img.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete from other business: %v", err)
	}
	if del, err := r.Delete(ctx, bizA, item, img.ID); err != nil || len(del.Keys()) != 2 {
		t.Fatalf("delete: %+v %v", del, err)
	}
}
