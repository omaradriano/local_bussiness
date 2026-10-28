package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
)

type MediaRepo interface {
	CreatePending(ctx context.Context, businessID, itemID, contentType string) (media.Image, error)
	Get(ctx context.Context, businessID, itemID, imageID string) (media.Image, error)
	ListForBusiness(ctx context.Context, businessID string) (map[string][]media.Image, error)
	Delete(ctx context.Context, businessID, itemID, imageID string) (media.Image, error)
}

type FileStore interface {
	PresignPut(ctx context.Context, bucket, key, contentType string, size int64) (string, error)
	PresignGet(ctx context.Context, bucket, key string) (string, error)
	Head(ctx context.Context, bucket, key string) (int64, string, error)
	Delete(ctx context.Context, bucket, key string) error
}

// Photos is optional: without S3 configured (e.g. a dev without AWS keys) the routes answer 503.
type Photos struct {
	Media        MediaRepo
	Files        FileStore
	EnqueueImage func(ctx context.Context, imageID string) error
	CDNBase      string // e.g. https://d1tklylecl3ocy.cloudfront.net
}

func (p *Photos) enabled(w http.ResponseWriter) bool {
	if p == nil || p.Files == nil {
		apierr.Write(w, http.StatusServiceUnavailable, "photos_unavailable", "Las fotos no están configuradas en este entorno.")
		return false
	}
	return true
}

// mountImages adds /catalog/{itemId}/images routes (owner only) inside a business group.
func mountImages(r chi.Router, p *Photos) {
	r.Route("/catalog/{itemId}/images", func(r chi.Router) {
		r.Use(auth.RequireRole("owner"), requireUUIDParam("itemId", "El artículo no existe."))

		// 1) Register the photo and get a presigned PUT; the app uploads straight to S3.
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if !p.enabled(w) {
				return
			}
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				ContentType string `json:"content_type"`
				Size        int64  `json:"size"`
			}
			if !decode(w, r, &body) {
				return
			}
			if msg := media.ValidateUpload(body.ContentType, body.Size); msg != "" {
				invalid(w, msg, map[string]string{"file": msg})
				return
			}
			img, err := p.Media.CreatePending(r.Context(), m.BusinessID, chi.URLParam(r, "itemId"), body.ContentType)
			if errors.Is(err, media.ErrNotFound) {
				invalid(w, "No se pudo agregar la foto.", map[string]string{"file": "El artículo no existe o ya tiene el máximo de fotos."})
				return
			}
			if err != nil {
				internalError(w, r, "create image", err)
				return
			}
			url, err := p.Files.PresignPut(r.Context(), files.Catalog, img.Key(), body.ContentType, body.Size)
			if err != nil {
				internalError(w, r, "presign", err)
				return
			}
			apierr.JSON(w, http.StatusCreated, map[string]any{
				"image":          img,
				"upload_url":     url,
				"upload_headers": map[string]string{"Content-Type": body.ContentType},
			})
		})

		// 2) After the upload: check the object really is there and within limits, then queue the variants.
		r.Post("/{imageId}/complete", func(w http.ResponseWriter, r *http.Request) {
			if !p.enabled(w) {
				return
			}
			img, ok := loadImage(w, r, p)
			if !ok {
				return
			}
			size, ct, err := p.Files.Head(r.Context(), files.Catalog, img.Key())
			if errors.Is(err, files.ErrNoObject) {
				invalid(w, "La foto no se terminó de subir.", map[string]string{"file": "La foto no se terminó de subir; intenta de nuevo."})
				return
			}
			if err != nil {
				internalError(w, r, "head image", err)
				return
			}
			if msg := media.ValidateUpload(ct, size); msg != "" {
				invalid(w, msg, map[string]string{"file": msg})
				return
			}
			if err := p.EnqueueImage(r.Context(), img.ID); err != nil {
				internalError(w, r, "enqueue image", err)
				return
			}
			apierr.JSON(w, http.StatusAccepted, map[string]any{"image": img})
		})

		r.Delete("/{imageId}", func(w http.ResponseWriter, r *http.Request) {
			if !p.enabled(w) {
				return
			}
			m, _ := auth.MemberFrom(r.Context())
			if !auth.IsUUID(chi.URLParam(r, "imageId")) {
				notFound(w, "La foto no existe.")
				return
			}
			img, err := p.Media.Delete(r.Context(), m.BusinessID, chi.URLParam(r, "itemId"), chi.URLParam(r, "imageId"))
			if errors.Is(err, media.ErrNotFound) {
				notFound(w, "La foto no existe.")
				return
			}
			if err != nil {
				internalError(w, r, "delete image", err)
				return
			}
			// Best effort: the row is gone, so a leftover object is only storage, never shown.
			for _, k := range img.Keys() {
				if err := p.Files.Delete(r.Context(), files.Catalog, k); err != nil {
					slog.WarnContext(r.Context(), "delete image object", "key", k, "err", err)
				}
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
}

func loadImage(w http.ResponseWriter, r *http.Request, p *Photos) (media.Image, bool) {
	m, _ := auth.MemberFrom(r.Context())
	imageID := chi.URLParam(r, "imageId")
	if !auth.IsUUID(imageID) {
		notFound(w, "La foto no existe.")
		return media.Image{}, false
	}
	img, err := p.Media.Get(r.Context(), m.BusinessID, chi.URLParam(r, "itemId"), imageID)
	if errors.Is(err, media.ErrNotFound) {
		notFound(w, "La foto no existe.")
		return img, false
	}
	if err != nil {
		internalError(w, r, "get image", err)
		return img, false
	}
	return img, true
}
