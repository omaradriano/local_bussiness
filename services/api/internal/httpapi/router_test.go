package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

const (
	user      = "11111111-1111-1111-1111-111111111111"
	staffUser = "22222222-2222-2222-2222-222222222222"
	itemID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	bizA      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	bizB      = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

type fakeStore struct{}

func (fakeStore) Role(_ context.Context, businessID, userID string) (string, error) {
	if businessID == bizA && userID == user {
		return "owner", nil
	}
	if businessID == bizA && userID == staffUser {
		return "staff", nil
	}
	return "", nil
}

func (fakeStore) Memberships(_ context.Context, userID string) ([]store.Membership, error) {
	if userID != user {
		return nil, nil
	}
	return []store.Membership{{BusinessID: bizA, Slug: "demo-unas", Name: "Uñas", Vertical: "nails", Role: "owner"}}, nil
}

// fakeAuth: "Bearer ok" is the owner of bizA, "Bearer staff" its employee; anything else is 401.
func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := map[string]string{"Bearer ok": user, "Bearer staff": staffUser}[r.Header.Get("Authorization")]
		if id == "" {
			apierr.Write(w, http.StatusUnauthorized, "unauthorized", "x")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), id)))
	})
}

type fakeCatalog struct{}

func (fakeCatalog) List(context.Context, string) ([]catalog.Item, error) {
	return []catalog.Item{{ID: itemID, Kind: "service", Name: "Gelish"}}, nil
}

func (fakeCatalog) Get(_ context.Context, _, id string) (catalog.Item, error) {
	if id != itemID {
		return catalog.Item{}, catalog.ErrNotFound
	}
	d := 60
	return catalog.Item{ID: itemID, Kind: "service", Name: "Gelish", Price: 25000, Active: true, DurationMinutes: &d}, nil
}

func (fakeCatalog) Create(_ context.Context, _ string, it catalog.Item) (catalog.Item, error) {
	it.ID = itemID
	return it, nil
}

func (fakeCatalog) Update(_ context.Context, _ string, it catalog.Item) (catalog.Item, error) {
	return it, nil
}

func fakeCreate(_ context.Context, userID string, in businesses.Input) (businesses.Business, error) {
	if in.Slug == "ocupado" {
		return businesses.Business{}, businesses.ErrSlugTaken
	}
	return businesses.Business{ID: bizA, Slug: in.Slug, Name: in.Name, Vertical: in.Vertical, Role: "owner"}, nil
}

func do(method, path, token string, headers ...string) *httptest.ResponseRecorder {
	return doBody(method, path, token, nil, headers...)
}

func doBody(method, path, token string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	NewRouter(Deps{
		Authenticate:   fakeAuth,
		Store:          fakeStore{},
		CreateBusiness: fakeCreate,
		Catalog:        fakeCatalog{},
		AllowedOrigins: []string{"http://localhost:8082", "https://local-bussiness-staging-*.vercel.app"},
	}).ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	rec := do(http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"status":"ok"}` {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	if rec := do(http.MethodGet, "/nope", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestMeRequiresSession(t *testing.T) {
	rec := do(http.MethodGet, "/v1/me", "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestMeListsBusinesses(t *testing.T) {
	rec := do(http.MethodGet, "/v1/me", "ok")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"slug":"demo-unas"`) || !strings.Contains(rec.Body.String(), `"role":"owner"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestBusinessRouteResolvesRole(t *testing.T) {
	if rec := do(http.MethodGet, "/v1/b/"+bizA+"/", "ok"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"owner"`) {
		t.Fatalf("own business: %d %s", rec.Code, rec.Body)
	}
	if rec := do(http.MethodGet, "/v1/b/"+bizB+"/", "ok"); rec.Code != http.StatusForbidden {
		t.Fatalf("other business: %d %s", rec.Code, rec.Body)
	}
}

func TestCORSAllowsOnlyEnvironmentOrigins(t *testing.T) {
	preflight := func(origin string) string {
		rec := do(http.MethodOptions, "/v1/me", "", "Origin", origin, "Access-Control-Request-Method", "GET")
		return rec.Header().Get("Access-Control-Allow-Origin")
	}
	for _, o := range []string{"http://localhost:8082", "https://local-bussiness-staging-abc123-omaradrianos-projects.vercel.app"} {
		if got := preflight(o); got != o {
			t.Errorf("origin %s: allow-origin = %q", o, got)
		}
	}
	if got := preflight("https://evil.example.com"); got != "" {
		t.Errorf("foreign origin allowed: %q", got)
	}
}

func TestCreateBusiness(t *testing.T) {
	cases := []struct {
		name, token, body string
		wantCode          int
		wantIn            string
	}{
		{"alta correcta", "ok", `{"name":" Uñas Bellas ","slug":"unas-bellas","vertical":"nails"}`, http.StatusCreated, `"role":"owner"`},
		{"sin sesión", "", `{"name":"x","slug":"xyz","vertical":"nails"}`, http.StatusUnauthorized, `"unauthorized"`},
		{"json inválido", "ok", `{nope`, http.StatusBadRequest, `"invalid_json"`},
		{"datos inválidos", "ok", `{"name":"","slug":"A B","vertical":"limos"}`, http.StatusUnprocessableEntity, `"slug":`},
		{"slug ocupado", "ok", `{"name":"Otra","slug":"ocupado","vertical":"flowers"}`, http.StatusConflict, `"slug_taken"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(http.MethodPost, "/v1/businesses", tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestCatalogRoutes(t *testing.T) {
	base := "/v1/b/" + bizA + "/catalog"
	svc := `{"kind":"service","name":"Acrílico","price":45000,"duration_minutes":120}`
	cases := []struct {
		name, method, path, token, body string
		wantCode                        int
		wantIn                          string
	}{
		{"empleada lee el catálogo", http.MethodGet, base, "staff", "", http.StatusOK, `"name":"Gelish"`},
		{"empleada no crea", http.MethodPost, base, "staff", svc, http.StatusForbidden, `"forbidden"`},
		{"dueño crea", http.MethodPost, base, "ok", svc, http.StatusCreated, `"id":"` + itemID + `"`},
		{"dueño con datos inválidos", http.MethodPost, base, "ok", `{"kind":"service","name":"X","price":100}`, http.StatusUnprocessableEntity, `"duration_minutes"`},
		{"negocio ajeno", http.MethodGet, "/v1/b/" + bizB + "/catalog", "ok", "", http.StatusForbidden, `"forbidden"`},
		{"dueño desactiva", http.MethodPatch, base + "/" + itemID, "ok", `{"active":false}`, http.StatusOK, `"active":false`},
		{"patch inválido", http.MethodPatch, base + "/" + itemID, "ok", `{"duration_minutes":1}`, http.StatusUnprocessableEntity, `"duration_minutes"`},
		{"artículo inexistente", http.MethodPatch, base + "/dddddddd-dddd-dddd-dddd-dddddddddddd", "ok", `{"active":false}`, http.StatusNotFound, `"not_found"`},
		{"id que no es uuid", http.MethodPatch, base + "/abc", "ok", `{"active":false}`, http.StatusNotFound, `"not_found"`},
		{"empleada no edita", http.MethodPatch, base + "/" + itemID, "staff", `{"active":false}`, http.StatusForbidden, `"forbidden"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}
