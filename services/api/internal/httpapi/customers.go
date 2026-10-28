package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
)

type CustomersRepo interface {
	Vertical(ctx context.Context, businessID string) (string, error)
	Search(ctx context.Context, businessID, q string, limit int) ([]customers.Customer, error)
	Get(ctx context.Context, businessID, id string) (customers.Customer, error)
	Create(ctx context.Context, businessID string, c customers.Customer) (customers.Customer, error)
	IDByPhone(ctx context.Context, businessID, phone string) (string, error)
	Update(ctx context.Context, businessID string, c customers.Customer) (customers.Customer, error)
	AddDocuments(ctx context.Context, businessID, customerID, docType string, keys []string, actor string) (customers.Customer, error)
	DeleteDocument(ctx context.Context, businessID, customerID, docID string) (string, error)
}

func writeCustomer(w http.ResponseWriter, r *http.Request, status int, c customers.Customer, err error) {
	switch {
	case errors.Is(err, customers.ErrNotFound):
		notFound(w, "El cliente no existe.")
	case err != nil:
		internalError(w, r, "customer", err)
	default:
		apierr.JSON(w, status, c)
	}
}

// mountCustomers adds /customers routes inside a business group. Any member can register and
// edit customers (counter work); only the owner marks them trusted or deletes documents.
func mountCustomers(r chi.Router, d Deps) {
	r.Get("/customers", func(w http.ResponseWriter, r *http.Request) {
		m, _ := auth.MemberFrom(r.Context())
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 100 {
			limit = 30
		}
		list, err := d.Customers.Search(r.Context(), m.BusinessID, r.URL.Query().Get("q"), limit)
		if err != nil {
			internalError(w, r, "search customers", err)
			return
		}
		apierr.JSON(w, http.StatusOK, map[string]any{"customers": list})
	})

	save := func(w http.ResponseWriter, r *http.Request, current customers.Customer, create bool) {
		m, _ := auth.MemberFrom(r.Context())
		var in customers.Input
		if !decode(w, r, &in) {
			return
		}
		if in.Trusted != nil && m.Role != "owner" {
			in.Trusted = nil // only the owner decides who is trusted
		}
		c := current.Apply(in)
		vertical, err := d.Customers.Vertical(r.Context(), m.BusinessID)
		if err != nil {
			internalError(w, r, "vertical", err)
			return
		}
		rules := customers.Rules{RequireAddress: businesses.HasFeature(vertical, "customerAddress"), RequireGuarantor: businesses.HasFeature(vertical, "guarantor")}
		if errs := c.Validate(rules); len(errs) > 0 {
			invalid(w, "Revisa los datos del cliente.", errs)
			return
		}
		var saved customers.Customer
		if create {
			saved, err = d.Customers.Create(r.Context(), m.BusinessID, c)
		} else {
			saved, err = d.Customers.Update(r.Context(), m.BusinessID, c)
		}
		if errors.Is(err, customers.ErrPhoneTaken) {
			id, _ := d.Customers.IDByPhone(r.Context(), m.BusinessID, c.Phone)
			apierr.JSON(w, http.StatusConflict, apierr.Error{Code: "customer_exists", Message: "Ya hay un cliente con ese teléfono.",
				Details: map[string]string{"phone": "Ya hay un cliente con ese teléfono.", "customer_id": id}})
			return
		}
		writeCustomer(w, r, map[bool]int{true: http.StatusCreated, false: http.StatusOK}[create], saved, err)
	}

	r.Post("/customers", func(w http.ResponseWriter, r *http.Request) { save(w, r, customers.Customer{}, true) })

	r.Route("/customers/{customerId}", func(r chi.Router) {
		r.Use(requireUUIDParam("customerId", "El cliente no existe."))
		load := func(w http.ResponseWriter, r *http.Request) (customers.Customer, bool) {
			m, _ := auth.MemberFrom(r.Context())
			c, err := d.Customers.Get(r.Context(), m.BusinessID, chi.URLParam(r, "customerId"))
			if err != nil {
				writeCustomer(w, r, 0, c, err)
				return c, false
			}
			return c, true
		}

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if c, ok := load(w, r); ok {
				apierr.JSON(w, http.StatusOK, c)
			}
		})

		r.Patch("/", func(w http.ResponseWriter, r *http.Request) {
			if c, ok := load(w, r); ok {
				save(w, r, c, false)
			}
		})

		// Documents: presigned upload to the private bucket, then register the keys.
		r.Post("/uploads", func(w http.ResponseWriter, r *http.Request) {
			if !d.Photos.enabled(w) {
				return
			}
			c, ok := load(w, r)
			if !ok {
				return
			}
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				DocType     string `json:"doc_type"`
				ContentType string `json:"content_type"`
				Size        int64  `json:"size"`
			}
			if !decode(w, r, &body) {
				return
			}
			if !slices.Contains(customers.DocTypes, body.DocType) {
				invalid(w, "Tipo de documento inválido.", map[string]string{"doc_type": "Tipo de documento inválido."})
				return
			}
			if msg := media.ValidateUpload(body.ContentType, body.Size); msg != "" {
				invalid(w, msg, map[string]string{"file": msg})
				return
			}
			key := customers.NewFileKey(m.BusinessID, c.ID, media.Ext(body.ContentType))
			url, err := d.Photos.Files.PresignPut(r.Context(), files.PrivateDocs, key, body.ContentType, body.Size)
			if err != nil {
				internalError(w, r, "presign", err)
				return
			}
			apierr.JSON(w, http.StatusCreated, map[string]any{"key": key, "upload_url": url,
				"upload_headers": map[string]string{"Content-Type": body.ContentType}})
		})

		r.Post("/documents", func(w http.ResponseWriter, r *http.Request) {
			if !d.Photos.enabled(w) {
				return
			}
			c, ok := load(w, r)
			if !ok {
				return
			}
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				DocType string   `json:"doc_type"`
				Keys    []string `json:"keys"`
			}
			if !decode(w, r, &body) {
				return
			}
			if !slices.Contains(customers.DocTypes, body.DocType) || len(body.Keys) == 0 || len(body.Keys) > 6 {
				invalid(w, "Revisa el documento.", map[string]string{"doc_type": "Agrega de 1 a 6 fotos de un tipo de documento válido."})
				return
			}
			prefix := customers.FilePrefix(m.BusinessID, c.ID)
			for _, k := range body.Keys {
				if !strings.HasPrefix(k, prefix) || strings.Contains(k, "..") {
					invalid(w, "Hay un archivo que no es de este cliente.", map[string]string{"keys": "Archivo inválido."})
					return
				}
				if _, _, err := d.Photos.Files.Head(r.Context(), files.PrivateDocs, k); err != nil {
					invalid(w, "Una foto no se terminó de subir; vuelve a intentarlo.", map[string]string{"keys": "Foto incompleta."})
					return
				}
			}
			saved, err := d.Customers.AddDocuments(r.Context(), m.BusinessID, c.ID, body.DocType, body.Keys, m.UserID)
			writeCustomer(w, r, http.StatusOK, saved, err)
		})

		r.Get("/files", func(w http.ResponseWriter, r *http.Request) {
			if !d.Photos.enabled(w) {
				return
			}
			m, _ := auth.MemberFrom(r.Context())
			key := r.URL.Query().Get("key")
			// Documents uploaded from a rental live under the booking's prefix; both belong to the business.
			if !strings.HasPrefix(key, m.BusinessID+"/") || strings.Contains(key, "..") {
				notFound(w, "El archivo no existe.")
				return
			}
			url, err := d.Photos.Files.PresignGet(r.Context(), files.PrivateDocs, key)
			if err != nil {
				internalError(w, r, "presign get", err)
				return
			}
			apierr.JSON(w, http.StatusOK, map[string]string{"url": url})
		})

		r.With(auth.RequireRole("owner"), requireUUIDParam("docId", "El documento no existe.")).Delete("/documents/{docId}", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			key, err := d.Customers.DeleteDocument(r.Context(), m.BusinessID, chi.URLParam(r, "customerId"), chi.URLParam(r, "docId"))
			if errors.Is(err, customers.ErrNotFound) {
				notFound(w, "El documento no existe.")
				return
			}
			if err != nil {
				internalError(w, r, "delete document", err)
				return
			}
			if d.Photos != nil && d.Photos.Files != nil {
				_ = d.Photos.Files.Delete(r.Context(), files.PrivateDocs, key) // best effort: the row is gone
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
}
