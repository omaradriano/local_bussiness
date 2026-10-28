// Package media tracks catalog photos: the row in catalog_item_images, where the original lives in S3,
// and the variants the worker generates.
package media

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrNotFound = errors.New("image or item not found")

const MaxUploadBytes = 5 << 20 // 5 MB; the app shrinks photos to 2048 px before uploading
const MaxImagesPerItem = 8

var extByType = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// ValidateUpload returns "" when the declared file is acceptable, else the reason (es-MX).
func ValidateUpload(contentType string, size int64) string {
	if _, ok := extByType[contentType]; !ok {
		return "Solo se aceptan fotos JPG, PNG o WebP."
	}
	if size <= 0 || size > MaxUploadBytes {
		return "La foto debe pesar como máximo 5 MB."
	}
	return ""
}

type Image struct {
	ID       string            `json:"id"`
	ItemID   string            `json:"item_id"`
	Status   string            `json:"status"` // processing | ready | failed
	Width    *int              `json:"width"`
	Height   *int              `json:"height"`
	Blurhash *string           `json:"blurhash"`
	URLs     map[string]string `json:"urls"` // variant -> public CDN URL (when ready)

	key      string
	variants map[string]string // variant -> S3 key
}

func (i Image) Key() string { return i.key }

// WithURLs fills URLs from the CDN base for ready images.
func (i Image) WithURLs(cdnBase string) Image {
	i.URLs = map[string]string{}
	for name, key := range i.variants {
		i.URLs[name] = cdnBase + "/" + key
	}
	return i
}

// VariantKey is where a generated size is stored, next to the original.
func VariantKey(originalKey, name string) string {
	return path.Join(path.Dir(originalKey), name+".webp")
}

type Repo struct{ DB store.DB }

const cols = `id::text, item_id::text, status, width, height, blurhash, s3_key, variants`

func scan(row pgx.Row) (Image, error) {
	var i Image
	err := row.Scan(&i.ID, &i.ItemID, &i.Status, &i.Width, &i.Height, &i.Blurhash, &i.key, &i.variants)
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

// CreatePending registers a photo for an item of the business; the key is
// {business}/catalog/{item}/{image}/original.{ext}. ErrNotFound if the item isn't the business's.
func (r Repo) CreatePending(ctx context.Context, businessID, itemID, contentType string) (Image, error) {
	ext := extByType[contentType]
	return scan(r.DB.QueryRow(ctx, `
		with new as (select gen_random_uuid() as id)
		insert into public.catalog_item_images (id, business_id, item_id, s3_key, status, sort)
		select new.id, i.business_id, i.id,
		       format('%s/catalog/%s/%s/original.%s', i.business_id, i.id, new.id, $3::text), 'processing',
		       coalesce((select max(sort) + 1 from public.catalog_item_images x where x.item_id = i.id), 1)
		from public.catalog_items i, new
		where i.business_id = $1 and i.id = $2
		  and (select count(*) from public.catalog_item_images x where x.item_id = i.id) < $4
		returning `+cols, businessID, itemID, ext, MaxImagesPerItem))
}

func (r Repo) Get(ctx context.Context, businessID, itemID, imageID string) (Image, error) {
	return scan(r.DB.QueryRow(ctx, `select `+cols+` from public.catalog_item_images
		where business_id = $1 and item_id = $2 and id = $3`, businessID, itemID, imageID))
}

// GetByID is for the worker, which only knows the image id from the job.
func (r Repo) GetByID(ctx context.Context, imageID string) (Image, error) {
	return scan(r.DB.QueryRow(ctx, `select `+cols+` from public.catalog_item_images where id = $1`, imageID))
}

// ListForBusiness returns every photo of the business grouped by item, in display order.
func (r Repo) ListForBusiness(ctx context.Context, businessID string) (map[string][]Image, error) {
	rows, err := r.DB.Query(ctx, `select `+cols+` from public.catalog_item_images
		where business_id = $1 and status <> 'failed' order by item_id, sort`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Image{}
	for rows.Next() {
		i, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out[i.ItemID] = append(out[i.ItemID], i)
	}
	return out, rows.Err()
}

func (r Repo) MarkReady(ctx context.Context, imageID string, width, height int, blurhash string, variants map[string]string) error {
	_, err := r.DB.Exec(ctx, `update public.catalog_item_images
		set status = 'ready', width = $2, height = $3, blurhash = $4, variants = $5 where id = $1`,
		imageID, width, height, blurhash, variants)
	return err
}

func (r Repo) MarkFailed(ctx context.Context, imageID string) error {
	_, err := r.DB.Exec(ctx, `update public.catalog_item_images set status = 'failed' where id = $1`, imageID)
	return err
}

func (r Repo) Delete(ctx context.Context, businessID, itemID, imageID string) (Image, error) {
	return scan(r.DB.QueryRow(ctx, `delete from public.catalog_item_images
		where business_id = $1 and item_id = $2 and id = $3 returning `+cols, businessID, itemID, imageID))
}

// Keys returns every S3 key of the image (original and variants), for deletion.
func (i Image) Keys() []string {
	keys := []string{i.key}
	for _, k := range i.variants {
		keys = append(keys, k)
	}
	return keys
}

func (i Image) String() string { return fmt.Sprintf("image %s (%s)", i.ID, i.Status) }

// Ext is the file extension for an accepted content type.
func Ext(contentType string) string { return extByType[contentType] }
