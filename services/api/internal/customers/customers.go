// Package customers keeps the business's customer profiles: contact, address, guarantor (aval),
// "trusted" flag and their documents (ID, proof of address, guarantor ID) in the private bucket.
package customers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/omaradriano/local_bussiness/services/api/internal/features"
	"github.com/omaradriano/local_bussiness/services/api/internal/phone"
	"github.com/omaradriano/local_bussiness/services/api/internal/privacy"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var (
	ErrNotFound   = errors.New("customer not found")
	ErrPhoneTaken = errors.New("phone already registered")
)

// DocTypes a customer can have on file.
var DocTypes = []string{"ine", "proof_of_address", "guarantor_id"}

type Document struct {
	ID        string    `json:"id"`
	Type      string    `json:"doc_type"`
	Key       string    `json:"key"` // bucket private-docs
	CreatedAt time.Time `json:"created_at"`
}

type Customer struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Phone          string  `json:"phone"` // E.164
	Email          *string `json:"email"`
	Address        *string `json:"address"`
	GuarantorName  *string `json:"guarantor_name"`
	GuarantorPhone *string `json:"guarantor_phone"`
	Trusted        bool    `json:"trusted"`
	Notes          *string `json:"notes"`
	// When the customer accepted the business's privacy notice, and which version (nil = not recorded).
	PrivacyAcceptedAt *time.Time `json:"privacy_accepted_at"`
	PrivacyVersion    *string    `json:"privacy_version"`
	Documents         []Document `json:"documents"`
	CreatedAt         time.Time  `json:"created_at"`
}

// HasDoc reports whether the customer has at least one document of that type on file.
func (c Customer) HasDoc(docType string) bool {
	for _, d := range c.Documents {
		if d.Type == docType {
			return true
		}
	}
	return false
}

// Input creates or edits a profile; nil fields are left unchanged on edit.
type Input struct {
	Name           *string `json:"name"`
	Phone          *string `json:"phone"`
	Email          *string `json:"email"`
	Address        *string `json:"address"`
	GuarantorName  *string `json:"guarantor_name"`
	GuarantorPhone *string `json:"guarantor_phone"`
	Trusted        *bool   `json:"trusted"`
	Notes          *string `json:"notes"`
	// true records the acceptance of the current notice; false or nil leave the record as it is
	// (withdrawing consent is an ARCO request, not an unticked box).
	PrivacyAccepted *bool `json:"privacy_accepted"`
}

func trimmed(p *string) *string {
	if p == nil {
		return nil
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return nil
	}
	return &s
}

// Apply merges the input into c, normalizing text and phones; now dates a privacy acceptance.
func (c Customer) Apply(in Input, now time.Time) Customer {
	if in.Name != nil {
		c.Name = strings.TrimSpace(*in.Name)
	}
	if in.Phone != nil {
		c.Phone = phone.Normalize(*in.Phone)
	}
	if in.Email != nil {
		c.Email = trimmed(in.Email)
	}
	if in.Address != nil {
		c.Address = trimmed(in.Address)
	}
	if in.GuarantorName != nil {
		c.GuarantorName = trimmed(in.GuarantorName)
	}
	if in.GuarantorPhone != nil {
		if g := trimmed(in.GuarantorPhone); g != nil {
			n := phone.Normalize(*g)
			c.GuarantorPhone = &n
		} else {
			c.GuarantorPhone = nil
		}
	}
	if in.Trusted != nil {
		c.Trusted = *in.Trusted
	}
	if in.Notes != nil {
		c.Notes = trimmed(in.Notes)
	}
	// The same version accepted again keeps its first date; a newer notice is recorded anew.
	if in.PrivacyAccepted != nil && *in.PrivacyAccepted && (c.PrivacyVersion == nil || *c.PrivacyVersion != privacy.Version) {
		v := privacy.Version
		c.PrivacyAcceptedAt, c.PrivacyVersion = &now, &v
	}
	return c
}

// Rules: what the business's giro asks of a customer (from its features).
type Rules struct {
	RequireAddress   bool // feature customerAddress
	RequireGuarantor bool // feature guarantor
}

// Validate returns field -> message (es-MX); the giro's rules add required fields.
func (c Customer) Validate(rules Rules) map[string]string {
	errs := map[string]string{}
	if n := utf8.RuneCountInString(c.Name); n == 0 || n > 80 {
		errs["name"] = "Escribe el nombre del cliente."
	}
	if c.Phone == "" {
		errs["phone"] = "Escribe un teléfono válido (10 dígitos en México)."
	}
	if c.Address != nil && utf8.RuneCountInString(*c.Address) > 300 {
		errs["address"] = "La dirección es muy larga."
	}
	if c.GuarantorPhone != nil && *c.GuarantorPhone == "" {
		errs["guarantor_phone"] = "El teléfono del aval no es válido."
	}
	if rules.RequireAddress && c.Address == nil {
		errs["address"] = "Escribe la dirección del cliente."
	}
	if rules.RequireGuarantor {
		if c.GuarantorName == nil {
			errs["guarantor_name"] = "Escribe el nombre del aval."
		}
		if c.GuarantorPhone == nil {
			errs["guarantor_phone"] = "Escribe el teléfono del aval."
		}
	}
	return errs
}

// FilePrefix: every document of a customer lives under this key prefix.
func FilePrefix(businessID, customerID string) string {
	return businessID + "/customers/" + customerID + "/"
}

func NewFileKey(businessID, customerID, ext string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return FilePrefix(businessID, customerID) + hex.EncodeToString(b) + "." + ext
}

// ---------------------------------------------------------------- repo

type Repo struct{ DB store.DB }

const cols = `id::text, name, phone, email, address, guarantor_name, guarantor_phone, trusted, notes, privacy_accepted_at, privacy_version, created_at`

func scan(row pgx.Row) (Customer, error) {
	var c Customer
	err := row.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Address, &c.GuarantorName, &c.GuarantorPhone, &c.Trusted, &c.Notes, &c.PrivacyAcceptedAt, &c.PrivacyVersion, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return c, ErrPhoneTaken
	}
	return c, err
}

// Search by name or phone digits (empty q = most recent); documents included.
func (r Repo) Search(ctx context.Context, businessID, q string, limit int) ([]Customer, error) {
	q = strings.TrimSpace(q)
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, q)
	rows, err := r.DB.Query(ctx, `select `+cols+` from public.customers
		where business_id = $1 and not walk_in
		  and ($2 = '' or name ilike '%' || $2 || '%' or ($3 <> '' and phone like '%' || $3 || '%'))
		order by (case when $2 = '' then created_at end) desc nulls last, name
		limit $4`, businessID, q, digits, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Customer{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, r.attachDocs(ctx, out)
}

func (r Repo) attachDocs(ctx context.Context, cs []Customer) error {
	if len(cs) == 0 {
		return nil
	}
	ids := make([]string, len(cs))
	idx := map[string]int{}
	for i, c := range cs {
		ids[i] = c.ID
		idx[c.ID] = i
		cs[i].Documents = []Document{}
	}
	rows, err := r.DB.Query(ctx, `select customer_id::text, id::text, doc_type, s3_key, created_at from public.customer_documents
		where customer_id::text = any($1) order by created_at desc`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid string
		var d Document
		if err := rows.Scan(&cid, &d.ID, &d.Type, &d.Key, &d.CreatedAt); err != nil {
			return err
		}
		cs[idx[cid]].Documents = append(cs[idx[cid]].Documents, d)
	}
	return rows.Err()
}

func (r Repo) Get(ctx context.Context, businessID, id string) (Customer, error) {
	c, err := scan(r.DB.QueryRow(ctx, `select `+cols+` from public.customers where business_id = $1 and id = $2`, businessID, id))
	if err != nil {
		return c, err
	}
	list := []Customer{c}
	err = r.attachDocs(ctx, list)
	return list[0], err
}

func (r Repo) Create(ctx context.Context, businessID string, c Customer) (Customer, error) {
	created, err := scan(r.DB.QueryRow(ctx, `insert into public.customers
		(business_id, name, phone, email, address, guarantor_name, guarantor_phone, trusted, notes, privacy_accepted_at, privacy_version)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) returning `+cols,
		businessID, c.Name, c.Phone, c.Email, c.Address, c.GuarantorName, c.GuarantorPhone, c.Trusted, c.Notes, c.PrivacyAcceptedAt, c.PrivacyVersion))
	created.Documents = []Document{}
	return created, err
}

// IDByPhone finds who already has that phone (to point at the existing profile on a duplicate).
func (r Repo) IDByPhone(ctx context.Context, businessID, phone string) (string, error) {
	var id string
	err := r.DB.QueryRow(ctx, `select id::text from public.customers where business_id = $1 and phone = $2`, businessID, phone).Scan(&id)
	return id, err
}

func (r Repo) Update(ctx context.Context, businessID string, c Customer) (Customer, error) {
	if _, err := scan(r.DB.QueryRow(ctx, `update public.customers set name = $3, phone = $4, email = $5, address = $6,
		guarantor_name = $7, guarantor_phone = $8, trusted = $9, notes = $10, privacy_accepted_at = $11, privacy_version = $12, updated_at = now()
		where business_id = $1 and id = $2 returning `+cols,
		businessID, c.ID, c.Name, c.Phone, c.Email, c.Address, c.GuarantorName, c.GuarantorPhone, c.Trusted, c.Notes, c.PrivacyAcceptedAt, c.PrivacyVersion)); err != nil {
		return Customer{}, err
	}
	return r.Get(ctx, businessID, c.ID)
}

// AddDocuments records uploaded files (already checked to exist) for a document type.
func (r Repo) AddDocuments(ctx context.Context, businessID, customerID, docType string, keys []string, actor string) (Customer, error) {
	for _, k := range keys {
		if _, err := r.DB.Exec(ctx, `insert into public.customer_documents (business_id, customer_id, doc_type, s3_key, uploaded_by)
			values ($1, $2, $3, $4, nullif($5, '')::uuid)`, businessID, customerID, docType, k, actor); err != nil {
			return Customer{}, err
		}
	}
	return r.Get(ctx, businessID, customerID)
}

func (r Repo) DeleteDocument(ctx context.Context, businessID, customerID, docID string) (string, error) {
	var key string
	err := r.DB.QueryRow(ctx, `delete from public.customer_documents where business_id = $1 and customer_id = $2 and id = $3 returning s3_key`,
		businessID, customerID, docID).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return key, err
}

// Rules of the business: which customer fields are required, from its active features.
func (r Repo) Rules(ctx context.Context, businessID string) (Rules, error) {
	var vertical string
	var settings, grants []byte
	var plan string
	err := r.DB.QueryRow(ctx, `select vertical, settings, feature_grants, plan from public.businesses where id = $1`, businessID).
		Scan(&vertical, &settings, &grants, &plan)
	if err != nil {
		return Rules{}, err
	}
	var s struct {
		Features map[string]bool `json:"features"`
	}
	_ = json.Unmarshal(settings, &s)
	active := features.Active(vertical, features.ParseForPlan(grants, plan), s.Features)
	return Rules{RequireAddress: slices.Contains(active, "customerAddress"), RequireGuarantor: slices.Contains(active, "guarantor")}, nil
}
