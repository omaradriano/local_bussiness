package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/bookings"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/exports"
	"github.com/omaradriano/local_bussiness/services/api/internal/features"
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/finances"
	"github.com/omaradriano/local_bussiness/services/api/internal/imports"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
	"github.com/omaradriano/local_bussiness/services/api/internal/plans"
	"github.com/omaradriano/local_bussiness/services/api/internal/platform"
	"github.com/omaradriano/local_bussiness/services/api/internal/products"
	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
	"github.com/omaradriano/local_bussiness/services/api/internal/staff"
	"github.com/omaradriano/local_bussiness/services/api/internal/steps"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

const (
	user      = "11111111-1111-1111-1111-111111111111"
	staffUser = "22222222-2222-2222-2222-222222222222"
	itemID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	bizA      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	bizB      = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	// The owner's other business, whose account ran out (read-only); expiredUser's own trial ended.
	bizExpired  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	expiredUser = "33333333-3333-3333-3333-333333333333"
	// A Lite business of the owner (at its limits), and a Lite owner who already has their one business.
	bizLite  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	liteUser = "44444444-4444-4444-4444-444444444444"
)

type fakeStore struct{}

func (fakeStore) Role(_ context.Context, businessID, userID string) (string, error) {
	if businessID == bizA && userID == user {
		return "owner", nil
	}
	if businessID == bizA && userID == staffUser {
		return "staff", nil
	}
	if (businessID == bizExpired || businessID == bizLite) && userID == user {
		return "owner", nil
	}
	return "", nil
}

func (fakeStore) Memberships(_ context.Context, userID string) ([]store.Membership, error) {
	if userID != user {
		return nil, nil
	}
	return []store.Membership{{BusinessID: bizA, Slug: "demo-unas", Name: "Uñas", Vertical: "nails", Role: "owner",
		Grants: []byte(`{"customerDocuments": true}`), Settings: []byte(`{}`)}}, nil
}

// The owner ("Bearer ok") also administers the platform.
func (fakeStore) IsPlatformAdmin(_ context.Context, userID string) (bool, error) {
	return userID == user, nil
}

// fakeAuth: "Bearer ok" is the owner of bizA, "Bearer staff" its employee; anything else is 401.
func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := map[string]string{"Bearer ok": user, "Bearer staff": staffUser, "Bearer vencido": expiredUser, "Bearer lite": liteUser}[r.Header.Get("Authorization")]
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

const unitID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

func (fakeCatalog) ListUnits(_ context.Context, _, item string) ([]catalog.Unit, error) {
	if item != itemID {
		return nil, catalog.ErrNotFound
	}
	return []catalog.Unit{{ID: unitID, ItemID: itemID, Code: "RM-01", Status: "available"}}, nil
}

func (fakeCatalog) GetUnit(_ context.Context, _, item, unit string) (catalog.Unit, error) {
	if item != itemID || unit != unitID {
		return catalog.Unit{}, catalog.ErrNotFound
	}
	return catalog.Unit{ID: unitID, ItemID: itemID, Code: "RM-01", Status: "available"}, nil
}

var createdUnits []string

func (fakeCatalog) CreateUnit(_ context.Context, _ string, u catalog.Unit) (catalog.Unit, error) {
	createdUnits = append(createdUnits, u.Code)
	if u.ItemID != itemID {
		return catalog.Unit{}, catalog.ErrNotFound
	}
	if u.Code == "RM-01" {
		return catalog.Unit{}, catalog.ErrCodeTaken
	}
	u.ID = unitID
	return u, nil
}

func (fakeCatalog) UpdateUnit(_ context.Context, _ string, u catalog.Unit) (catalog.Unit, error) {
	return u, nil
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
		Staff:          fakeStaff{},
		Availability:   fakeAvailability{},
		Bookings:       fakeBookings{},
		Settings:       &fakeSettings{s: settings.From([]byte(`{}`)).WithBusiness("barbershop", nil)},
		Customers:      &fakeCustomers{},
		Platform:       fakePlatform{},
		Finances:       fakeFinances{},
		Products:       fakeProducts{},
		Plans:          fakePlans{},
		DeleteBusiness: fakeDeleteBusiness,
		Exports:        fakeExports{},
		Imports:        fakeImports{},
		Now:            func() time.Time { return fixedNow },
		Photos:         &Photos{Media: fakeMedia{}, Files: fakeFiles{}, EnqueueImage: fakeEnqueue, CDNBase: "https://cdn.test"},
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
	body := rec.Body.String()
	// Active features: the giro's (staff) plus the one granted to this business.
	if rec.Code != http.StatusOK || !strings.Contains(body, `"slug":"demo-unas"`) || !strings.Contains(body, `"role":"owner"`) ||
		!strings.Contains(body, `"features":["customerDocuments","staff","startService"]`) || !strings.Contains(body, `"platform_admin":true`) {
		t.Fatalf("got %d %s", rec.Code, body)
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

// Every method the API exposes must pass the browser preflight (a missing PUT broke saving hours).
func TestCORSPreflightAllowsEveryMethodWeUse(t *testing.T) {
	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := do(http.MethodOptions, "/v1/b/"+bizA+"/hours", "", "Origin", "http://localhost:8082",
			"Access-Control-Request-Method", m, "Access-Control-Request-Headers", "authorization,content-type")
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8082" {
			t.Errorf("%s preflight: allow-origin = %q", m, got)
		}
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

func TestUnitRoutes(t *testing.T) {
	units := "/v1/b/" + bizA + "/catalog/" + itemID + "/units"
	cases := []struct {
		name, method, path, token, body string
		wantCode                        int
		wantIn                          string
	}{
		{"empleada lista unidades", http.MethodGet, units, "staff", "", http.StatusOK, `"code":"RM-01"`},
		{"artículo inexistente", http.MethodGet, "/v1/b/" + bizA + "/catalog/dddddddd-dddd-dddd-dddd-dddddddddddd/units", "ok", "", http.StatusNotFound, `"not_found"`},
		{"dueño agrega (clave en mayúsculas)", http.MethodPost, units, "ok", `{"code":" rm-02 "}`, http.StatusCreated, `"code":"RM-02"`},
		{"clave repetida", http.MethodPost, units, "ok", `{"code":"RM-01"}`, http.StatusConflict, `"code_taken"`},
		{"clave vacía", http.MethodPost, units, "ok", `{"code":""}`, http.StatusUnprocessableEntity, `"code"`},
		{"empleada no agrega", http.MethodPost, units, "staff", `{"code":"RM-03"}`, http.StatusForbidden, `"forbidden"`},
		{"a mantenimiento", http.MethodPatch, units + "/" + unitID, "ok", `{"status":"maintenance"}`, http.StatusOK, `"status":"maintenance"`},
		{"estado inválido", http.MethodPatch, units + "/" + unitID, "ok", `{"status":"perdida"}`, http.StatusUnprocessableEntity, `"status"`},
		{"unidad inexistente", http.MethodPatch, units + "/ffffffff-ffff-ffff-ffff-ffffffffffff", "ok", `{"status":"retired"}`, http.StatusNotFound, `"not_found"`},
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

const staffID = "abababab-abab-abab-abab-abababababab"

type fakeStaff struct{}

func (fakeStaff) List(context.Context, string) ([]staff.Member, error) {
	return []staff.Member{{ID: staffID, Name: "Yo", Active: true, IsOwner: true, ServiceIDs: []string{}, Hours: []staff.Hours{}}}, nil
}
func (f fakeStaff) Get(ctx context.Context, b, id string) (staff.Member, error) {
	if id != staffID {
		return staff.Member{}, staff.ErrNotFound
	}
	l, _ := f.List(ctx, b)
	return l[0], nil
}
func (fakeStaff) Create(_ context.Context, _ string, m staff.Member) (staff.Member, error) {
	m.ID, m.Active = staffID, true
	return m, nil
}
func (fakeStaff) Update(context.Context, string, staff.Member) error { return nil }
func (fakeStaff) SetServices(_ context.Context, _, id string, items []string) error {
	if id != staffID {
		return staff.ErrNotFound
	}
	if len(items) > 0 && items[0] == bizB {
		return staff.ErrNotAService
	}
	return nil
}
func (fakeStaff) SetHours(_ context.Context, _, id string, _ []staff.Hours) error {
	if id != "" && id != staffID {
		return staff.ErrNotFound
	}
	return nil
}
func (fakeStaff) BusinessHours(context.Context, string) ([]staff.Hours, error) {
	return []staff.Hours{{Weekday: 1, OpensAt: "10:00", ClosesAt: "19:00"}}, nil
}

func TestStaffAndHoursRoutes(t *testing.T) {
	b := "/v1/b/" + bizA
	one := b + "/staff/" + staffID
	cases := []struct {
		name, method, path, token, body string
		wantCode                        int
		wantIn                          string
	}{
		{"empleada ve al equipo", http.MethodGet, b + "/staff", "staff", "", http.StatusOK, `"is_owner":true`},
		{"dueño agrega", http.MethodPost, b + "/staff", "ok", `{"name":" Karla ","color":"#E91E63"}`, http.StatusCreated, `"name":"Karla"`},
		{"sin nombre", http.MethodPost, b + "/staff", "ok", `{"name":""}`, http.StatusUnprocessableEntity, `"name"`},
		{"empleada no agrega", http.MethodPost, b + "/staff", "staff", `{"name":"X"}`, http.StatusForbidden, `"forbidden"`},
		{"renombrar al dueño", http.MethodPatch, one + "/", "ok", `{"name":"Lupita"}`, http.StatusOK, `"name":"Lupita"`},
		{"persona inexistente", http.MethodPatch, b + "/staff/cdcdcdcd-cdcd-cdcd-cdcd-cdcdcdcdcdcd/", "ok", `{"name":"X"}`, http.StatusNotFound, `"not_found"`},
		{"id no uuid", http.MethodPut, b + "/staff/abc/hours", "ok", `{"hours":[]}`, http.StatusNotFound, `"not_found"`},
		{"asignar servicios", http.MethodPut, one + "/services", "ok", `{"service_ids":["` + itemID + `"]}`, http.StatusNoContent, ""},
		{"servicio de otro tipo", http.MethodPut, one + "/services", "ok", `{"service_ids":["` + bizB + `"]}`, http.StatusUnprocessableEntity, `"service_ids"`},
		{"id de servicio inválido", http.MethodPut, one + "/services", "ok", `{"service_ids":["x"]}`, http.StatusUnprocessableEntity, `"service_ids"`},
		{"horario partido", http.MethodPut, one + "/hours", "ok", `{"hours":[{"weekday":1,"opens_at":"09:00","closes_at":"14:00"},{"weekday":1,"opens_at":"16:00","closes_at":"20:00"}]}`, http.StatusNoContent, ""},
		{"horario encimado", http.MethodPut, one + "/hours", "ok", `{"hours":[{"weekday":1,"opens_at":"09:00","closes_at":"14:00"},{"weekday":1,"opens_at":"13:00","closes_at":"20:00"}]}`, http.StatusUnprocessableEntity, `"hours"`},
		{"empleada no cambia horarios", http.MethodPut, one + "/hours", "staff", `{"hours":[]}`, http.StatusForbidden, `"forbidden"`},
		{"horario general visible", http.MethodGet, b + "/hours", "staff", "", http.StatusOK, `"opens_at":"10:00"`},
		{"dueño cambia horario general", http.MethodPut, b + "/hours", "ok", `{"hours":[{"weekday":0,"opens_at":"09:00","closes_at":"20:00"}]}`, http.StatusNoContent, ""},
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

const imageID = "99999999-9999-9999-9999-999999999999"

type fakeMedia struct{}

func (fakeMedia) CreatePending(_ context.Context, _, item, _ string) (media.Image, error) {
	if item != itemID {
		return media.Image{}, media.ErrNotFound
	}
	return media.Image{ID: imageID, ItemID: itemID, Status: "processing"}, nil
}
func (fakeMedia) Get(_ context.Context, _, item, img string) (media.Image, error) {
	if item != itemID || img != imageID {
		return media.Image{}, media.ErrNotFound
	}
	return media.Image{ID: imageID, ItemID: itemID, Status: "processing"}, nil
}
func (fakeMedia) ListForBusiness(context.Context, string) (map[string][]media.Image, error) {
	return map[string][]media.Image{itemID: {{ID: imageID, ItemID: itemID, Status: "ready"}}}, nil
}
func (f fakeMedia) Delete(ctx context.Context, b, item, img string) (media.Image, error) {
	return f.Get(ctx, b, item, img)
}

// fakeFiles: the object "exists" unless the test turned uploaded off.
type fakeFiles struct{}

var uploaded = true

func (fakeFiles) PresignPut(_ context.Context, bucket, key, _ string, _ int64) (string, error) {
	return "https://s3.test/" + bucket + "/" + key + "?sig=x", nil
}
func (fakeFiles) Head(context.Context, string, string) (int64, string, error) {
	if !uploaded {
		return 0, "", files.ErrNoObject
	}
	return 500_000, "image/jpeg", nil
}
func (fakeFiles) Delete(context.Context, string, string) error { return nil }
func (fakeFiles) PresignGet(_ context.Context, bucket, key string) (string, error) {
	return "https://s3.test/" + bucket + "/" + key + "?get", nil
}

var enqueued []string

func fakeEnqueue(_ context.Context, id string) error {
	enqueued = append(enqueued, id)
	return nil
}

func TestCatalogImageRoutes(t *testing.T) {
	base := "/v1/b/" + bizA + "/catalog/" + itemID + "/images/"
	jpg := `{"content_type":"image/jpeg","size":800000}`
	cases := []struct {
		name, method, path, token, body string
		uploaded                        bool
		wantCode                        int
		wantIn                          string
	}{
		{"pedir subida", http.MethodPost, base, "ok", jpg, true, http.StatusCreated, `"upload_url":"https://s3.test/`},
		{"tipo no permitido", http.MethodPost, base, "ok", `{"content_type":"image/gif","size":1000}`, true, http.StatusUnprocessableEntity, `"file"`},
		{"muy pesada", http.MethodPost, base, "ok", `{"content_type":"image/jpeg","size":6000000}`, true, http.StatusUnprocessableEntity, `"file"`},
		{"artículo de otro negocio o lleno", http.MethodPost, "/v1/b/" + bizA + "/catalog/dddddddd-dddd-dddd-dddd-dddddddddddd/images/", "ok", jpg, true, http.StatusUnprocessableEntity, `"file"`},
		{"empleada no sube", http.MethodPost, base, "staff", jpg, true, http.StatusForbidden, `"forbidden"`},
		{"confirmar sin archivo", http.MethodPost, base + imageID + "/complete", "ok", "", false, http.StatusUnprocessableEntity, `no se terminó de subir`},
		{"confirmar subida", http.MethodPost, base + imageID + "/complete", "ok", "", true, http.StatusAccepted, `"status":"processing"`},
		{"foto inexistente", http.MethodPost, base + "abc/complete", "ok", "", true, http.StatusNotFound, `"not_found"`},
		{"borrar", http.MethodDelete, base + imageID, "ok", "", true, http.StatusNoContent, ""},
		{"borrar inexistente", http.MethodDelete, base + "88888888-8888-8888-8888-888888888888", "ok", "", true, http.StatusNotFound, `"not_found"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uploaded, enqueued = tc.uploaded, nil
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
			if tc.name == "confirmar subida" && (len(enqueued) != 1 || enqueued[0] != imageID) {
				t.Fatalf("job not enqueued: %v", enqueued)
			}
		})
	}
}

func TestCatalogListIncludesImages(t *testing.T) {
	rec := do(http.MethodGet, "/v1/b/"+bizA+"/catalog", "staff")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"images":[{"id":"`+imageID) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestPhotosDisabledAnswers503(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/b/"+bizA+"/catalog/"+itemID+"/images/", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer ok")
	rec := httptest.NewRecorder()
	NewRouter(Deps{Authenticate: fakeAuth, Store: fakeStore{}, Catalog: fakeCatalog{}, Plans: fakePlans{}}).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

var fixedNow = time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)

type fakeAvailability struct{}

// The fake echoes what reached it, so the test can see the parsed request and the injected clock.
func (fakeAvailability) Answer(_ context.Context, _ string, req availability.Request, now time.Time) (availability.Response, error) {
	switch {
	case req.ItemID != itemID:
		return availability.Response{}, availability.ErrItemNotFound
	case req.Date == "mal":
		return availability.Response{}, availability.BadRequest{Field: "date", Message: "Elige un día (AAAA-MM-DD)."}
	case !now.Equal(fixedNow):
		return availability.Response{}, availability.BadRequest{Field: "now", Message: "clock not injected"}
	}
	return availability.Response{Kind: "rental", Available: req.Quantity == 2, FreeUnits: []string{"u1", "u2"}}, nil
}

func TestAvailabilityRoute(t *testing.T) {
	base := "/v1/b/" + bizA + "/availability?item_id="
	cases := []struct {
		name, path, token string
		wantCode          int
		wantIn            string
	}{
		{"empleada consulta", base + itemID + "&from=2026-11-03T15:00:00Z&to=2026-11-05T15:00:00Z&quantity=2", "staff", http.StatusOK, `"available":true`},
		{"sin artículo", "/v1/b/" + bizA + "/availability", "ok", http.StatusUnprocessableEntity, `"item_id"`},
		{"cantidad inválida", base + itemID + "&quantity=0", "ok", http.StatusUnprocessableEntity, `"quantity"`},
		{"staff_id inválido", base + itemID + "&staff_id=x", "ok", http.StatusUnprocessableEntity, `"item_id"`},
		{"artículo inexistente", base + "dddddddd-dddd-dddd-dddd-dddddddddddd", "ok", http.StatusNotFound, `"not_found"`},
		{"fecha mal escrita", base + itemID + "&date=mal", "ok", http.StatusUnprocessableEntity, `"date"`},
		{"negocio ajeno", "/v1/b/" + bizB + "/availability?item_id=" + itemID, "ok", http.StatusForbidden, `"forbidden"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(http.MethodGet, tc.path, tc.token)
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

const bookingID = "b0b0b0b0-b0b0-b0b0-b0b0-b0b0b0b0b0b0"

type fakeBookings struct{}

func (fakeBookings) Create(_ context.Context, _ string, in bookings.CreateInput, now time.Time) (bookings.Booking, bool, error) {
	if err := in.Validate(); err != nil {
		return bookings.Booking{}, false, err
	}
	switch {
	case in.Notes == "ocupado":
		return bookings.Booking{}, false, bookings.ErrSlotTaken
	case in.Source != "admin" || (in.CreatedBy != user && in.CreatedBy != staffUser) || !now.Equal(fixedNow):
		return bookings.Booking{}, false, errors.New("source, actor or clock not set by the handler")
	}
	return bookings.Booking{ID: bookingID, Status: "pending_deposit"}, in.IdempotencyKey == "repetida-123", nil
}

func (fakeBookings) List(_ context.Context, _ string, f bookings.ListFilter) ([]bookings.Summary, error) {
	return []bookings.Summary{{ID: bookingID, Kind: f.Kind, Status: map[bool]string{true: "active", false: "all"}[f.OnlyActive]}}, nil
}

func (fakeBookings) Transition(_ context.Context, _, id, to, actor, _ string) (bookings.Booking, error) {
	switch {
	case id != bookingID:
		return bookings.Booking{}, bookings.ErrNotFound
	case to == "completed":
		return bookings.Booking{}, bookings.StepsPending{Labels: []string{"Foto del INE"}}
	case to == "ready":
		return bookings.Booking{}, bookings.Invalid{Field: "status", Message: "No se puede."}
	case actor == "":
		return bookings.Booking{}, errors.New("actor not passed")
	}
	return bookings.Booking{ID: id, Status: to}, nil
}

func (fakeBookings) AddPayment(_ context.Context, _, id string, p bookings.Payment, actor string, now time.Time, _ bookings.FileExists) (bookings.Booking, error) {
	if err := bookings.ValidatePayment(p, bookings.Balance{Due: 10000}); err != nil {
		return bookings.Booking{}, err
	}
	if actor == "" || !now.Equal(fixedNow) {
		return bookings.Booking{}, errors.New("actor or clock not passed")
	}
	return bookings.Booking{ID: id, Status: "confirmed", Balance: bookings.Balance{Paid: p.Amount, Due: 10000 - p.Amount}}, nil
}

const stepID = "57575757-5757-5757-5757-575757575757"

func (fakeBookings) StepUpload(_ context.Context, _, _, step string) (string, error) {
	if step != stepID {
		return "", bookings.ErrNotFound
	}
	return "private-docs", nil
}

func (fakeBookings) CompleteStep(ctx context.Context, _, id, step string, sub steps.Submission, _ string, _ time.Time, exists bookings.FileExists) (bookings.Booking, error) {
	if step != stepID {
		return bookings.Booking{}, bookings.ErrNotFound
	}
	for _, k := range sub.Files {
		if err := exists(ctx, "private-docs", k); err != nil {
			return bookings.Booking{}, bookings.Invalid{Field: "files", Message: "no se terminó de subir"}
		}
	}
	return bookings.Booking{ID: id, Steps: []bookings.Step{{ID: step, Status: "done"}}}, nil
}

func (fakeBookings) Checkout(_ context.Context, _, id, condition, _ string, _ time.Time) (bookings.Booking, error) {
	if condition == "sin ine" {
		return bookings.Booking{}, bookings.StepsPending{Labels: []string{"Foto del INE"}}
	}
	return bookings.Booking{ID: id, Status: "checked_out"}, nil
}

func (fakeBookings) Return(_ context.Context, _, id, _, _ string, _ time.Time) (bookings.Booking, error) {
	return bookings.Booking{ID: id, Status: "returned", Rental: &bookings.RentalInfo{LateDays: 2, LateFeeSuggested: 60000}}, nil
}

func (fakeBookings) SetDiscount(_ context.Context, _, id string, _ bookings.Discount, _ string) (bookings.Booking, error) {
	return bookings.Booking{ID: id}, nil
}

func (fakeBookings) Extend(_ context.Context, _, id string, newEnd time.Time, _ string, _ time.Time) (bookings.Booking, error) {
	if newEnd.Year() == 2030 {
		return bookings.Booking{}, bookings.ErrSlotTaken
	}
	return bookings.Booking{ID: id, Status: "checked_out", EndsAt: &newEnd}, nil
}

func (fakeBookings) RemoveStepFile(_ context.Context, _, id, _, key string) (bookings.Booking, error) {
	if key == "" {
		return bookings.Booking{}, bookings.ErrNotFound
	}
	return bookings.Booking{ID: id, Status: "confirmed"}, nil
}

func (fakeBookings) AddDocument(_ context.Context, _, id, name string, keys []string, _ string, _ bookings.FileExists) (bookings.Booking, error) {
	if name == "" {
		return bookings.Booking{}, bookings.Invalid{Field: "name", Message: "nombre"}
	}
	return bookings.Booking{ID: id, Documents: []bookings.Document{{Name: name, Files: keys}}}, nil
}

func (fakeBookings) RemoveDocument(_ context.Context, _, id, _ string) (bookings.Booking, []string, error) {
	return bookings.Booking{ID: id, Documents: []bookings.Document{}}, []string{"k"}, nil
}

func (fakeBookings) Inventory(context.Context, string) ([]bookings.InventoryRow, error) {
	return []bookings.InventoryRow{{State: "available", UnitCode: "RM-01"}}, nil
}

func (fakeBookings) Quote(_ context.Context, _ string, in bookings.QuoteRequest) (bookings.QuoteResult, error) {
	if in.EndsAt != nil && in.EndsAt.Weekday() == time.Sunday {
		return bookings.QuoteResult{}, bookings.Invalid{Field: "ends_at", Message: "cerrado"}
	}
	return bookings.QuoteResult{Quote: bookings.Quote{Total: 60000}, Periods: 2}, nil
}

func TestQuoteRoute(t *testing.T) {
	base := "/v1/b/" + bizA + "/quote?item_id=" + itemID + "&quantity=1&starts_at=2026-11-02T15:00:00Z"
	cases := []struct {
		name, query string
		code        int
		want        string
	}{
		{"cotiza", "&ends_at=2026-11-04T15:00:00Z", http.StatusOK, `"total":60000`},
		{"regreso en domingo", "&ends_at=2026-11-08T15:00:00Z", http.StatusUnprocessableEntity, `"ends_at"`},
		{"fecha inválida", "&ends_at=mañana", http.StatusUnprocessableEntity, `"ends_at"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(http.MethodGet, base+tc.query, "staff"); rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func (fakeBookings) History(_ context.Context, _ string, f bookings.HistoryFilter) ([]bookings.Summary, error) {
	return []bookings.Summary{{ID: bookingID, Status: "completed", CustomerName: "Ana " + f.Group}}, nil
}

func TestHistoryRoute(t *testing.T) {
	base := "/v1/b/" + bizA + "/bookings/history"
	cases := []struct {
		name, query string
		code        int
		want        string
	}{
		{"terminadas", "?group=finished", http.StatusOK, `"customer_name":"Ana finished"`},
		{"todas", "", http.StatusOK, `"status":"completed"`},
		{"grupo inválido", "?group=raro", http.StatusUnprocessableEntity, `"group"`},
		{"cliente inválido", "?customer_id=x", http.StatusUnprocessableEntity, `"group"`},
		{"cursor inválido", "?before=ayer", http.StatusUnprocessableEntity, `"before"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(http.MethodGet, base+tc.query, "staff"); rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestInventoryRoute(t *testing.T) {
	if rec := do(http.MethodGet, "/v1/b/"+bizA+"/inventory", "staff"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"unit_code":"RM-01"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func (fakeBookings) Get(_ context.Context, _, id string) (bookings.Booking, error) {
	if id != bookingID {
		return bookings.Booking{}, bookings.ErrNotFound
	}
	return bookings.Booking{ID: bookingID}, nil
}

func TestBookingRoutes(t *testing.T) {
	base := "/v1/b/" + bizA + "/bookings"
	ok := `{"customer":{"name":"Ana","phone":"6141234567"},"item_id":"` + itemID + `","starts_at":"2026-11-02T17:00:00Z"`
	cases := []struct {
		name, method, path, token, body, key string
		wantCode                             int
		wantIn                               string
	}{
		{"empleada captura en mostrador", http.MethodPost, base, "staff", ok + `}`, "", http.StatusCreated, `"status":"pending_deposit"`},
		{"reintento con la misma clave", http.MethodPost, base, "ok", ok + `}`, "repetida-123", http.StatusOK, bookingID},
		{"clave inválida", http.MethodPost, base, "ok", ok + `}`, "corta", http.StatusUnprocessableEntity, `"idempotency_key"`},
		{"teléfono inválido", http.MethodPost, base, "ok", `{"customer":{"name":"Ana","phone":"123"},"item_id":"` + itemID + `","starts_at":"2026-11-02T17:00:00Z"}`, "", http.StatusUnprocessableEntity, `"customer.phone"`},
		{"sin fecha", http.MethodPost, base, "ok", `{"customer":{"name":"Ana","phone":"6141234567"},"item_id":"` + itemID + `"}`, "", http.StatusUnprocessableEntity, `"starts_at"`},
		{"artículo inválido", http.MethodPost, base, "ok", `{"item_id":"x"}`, "", http.StatusUnprocessableEntity, `"item_id"`},
		{"choque", http.MethodPost, base, "ok", ok + `,"notes":"ocupado"}`, "", http.StatusConflict, `"slot_taken"`},
		{"negocio ajeno", http.MethodPost, "/v1/b/" + bizB + "/bookings", "ok", ok + `}`, "", http.StatusForbidden, `"forbidden"`},
		{"detalle", http.MethodGet, base + "/" + bookingID + "/", "staff", "", "", http.StatusOK, bookingID},
		{"detalle inexistente", http.MethodGet, base + "/eeeeeeee-0000-0000-0000-000000000000/", "ok", "", "", http.StatusNotFound, `"not_found"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var headers []string
			if tc.key != "" {
				headers = []string{"Idempotency-Key", tc.key}
			}
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body), headers...)
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestListBookingsRoute(t *testing.T) {
	base := "/v1/b/" + bizA + "/bookings?from=2026-11-01T00:00:00Z&to=2026-12-01T00:00:00Z"
	cases := []struct {
		name, path string
		wantCode   int
		wantIn     string
	}{
		{"activas por defecto", base + "&kind=rental", http.StatusOK, `"status":"active"`},
		{"todas", base + "&status=all", http.StatusOK, `"status":"all"`},
		{"tipo inválido", base + "&kind=x", http.StatusUnprocessableEntity, `"kind"`},
		{"rango invertido", "/v1/b/" + bizA + "/bookings?from=2026-12-01T00:00:00Z&to=2026-11-01T00:00:00Z", http.StatusUnprocessableEntity, `"from"`},
		{"rango de un año", "/v1/b/" + bizA + "/bookings?from=2026-01-01T00:00:00Z&to=2027-01-01T00:00:00Z", http.StatusUnprocessableEntity, `"from"`},
		{"sin rango", "/v1/b/" + bizA + "/bookings", http.StatusUnprocessableEntity, `"from"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(http.MethodGet, tc.path, "staff")
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestCreateRentalWithUnits(t *testing.T) {
	createdUnits = nil
	body := `{"kind":"rental","name":"Generador","price":40000,"price_unit":"day","security_deposit":0,"late_fee_per_day":0,"units":3}`
	rec := doBody(http.MethodPost, "/v1/b/"+bizA+"/catalog", "ok", strings.NewReader(body))
	if rec.Code != http.StatusCreated || strings.Join(createdUnits, ",") != "GEN-01,GEN-02,GEN-03" {
		t.Fatalf("got %d %s units=%v", rec.Code, rec.Body, createdUnits)
	}
	createdUnits = nil
	rec = doBody(http.MethodPost, "/v1/b/"+bizA+"/catalog", "ok", strings.NewReader(`{"kind":"service","name":"X","price":1,"duration_minutes":30,"units":3}`))
	if rec.Code != http.StatusCreated || len(createdUnits) != 0 {
		t.Fatalf("units for a service: %d %v", rec.Code, createdUnits)
	}
	if rec := doBody(http.MethodPost, "/v1/b/"+bizA+"/catalog", "ok", strings.NewReader(`{"kind":"rental","name":"X","price":1,"units":99}`)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("99 units: %d", rec.Code)
	}
}

func TestBookingOperationRoutes(t *testing.T) {
	one := "/v1/b/" + bizA + "/bookings/" + bookingID
	cases := []struct {
		name, path, token, body string
		wantCode                int
		wantIn                  string
	}{
		{"empleada cambia estado", one + "/transition", "staff", `{"to":"in_progress"}`, http.StatusOK, `"status":"in_progress"`},
		{"transición inválida", one + "/transition", "ok", `{"to":"ready"}`, http.StatusUnprocessableEntity, `"status"`},
		{"pasos pendientes", one + "/transition", "ok", `{"to":"completed"}`, http.StatusUnprocessableEntity, `"steps_pending"`},
		{"reserva inexistente", "/v1/b/" + bizA + "/bookings/eeeeeeee-0000-0000-0000-000000000000/transition", "ok", `{"to":"cancelled"}`, http.StatusNotFound, `"not_found"`},
		{"id que no es uuid", "/v1/b/" + bizA + "/bookings/abc/transition", "ok", `{"to":"cancelled"}`, http.StatusNotFound, `"not_found"`},
		{"pago", one + "/payments", "staff", `{"type":"advance","method":"cash","amount":3000}`, http.StatusOK, `"due":7000`},
		{"pago mayor al saldo", one + "/payments", "ok", `{"type":"balance","method":"cash","amount":20000}`, http.StatusUnprocessableEntity, `"amount"`},
		{"método inválido", one + "/payments", "ok", `{"type":"balance","method":"cheque","amount":100}`, http.StatusUnprocessableEntity, `"method"`},
		{"negocio ajeno", "/v1/b/" + bizB + "/bookings/" + bookingID + "/payments", "ok", `{}`, http.StatusForbidden, `"forbidden"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(http.MethodPost, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestBookingDetailWithoutTrailingSlash(t *testing.T) {
	if rec := do(http.MethodGet, "/v1/b/"+bizA+"/bookings/"+bookingID, "ok"); rec.Code != http.StatusOK {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestRentalFlowRoutes(t *testing.T) {
	one := "/v1/b/" + bizA + "/bookings/" + bookingID
	prefix := bizA + "/bookings/" + bookingID + "/"
	cases := []struct {
		name, method, path, body string
		uploaded                 bool
		wantCode                 int
		wantIn                   string
	}{
		{"subir INE", http.MethodPost, one + "/uploads", `{"step_id":"` + stepID + `","content_type":"image/jpeg","size":900000}`, true, http.StatusCreated, `"key":"` + prefix},
		{"paso sin archivos o ajeno", http.MethodPost, one + "/uploads", `{"step_id":"eeeeeeee-0000-0000-0000-000000000000","content_type":"image/jpeg","size":1}`, true, http.StatusNotFound, `"not_found"`},
		{"tipo de archivo inválido", http.MethodPost, one + "/uploads", `{"step_id":"` + stepID + `","content_type":"text/plain","size":1}`, true, http.StatusUnprocessableEntity, `"file"`},
		{"completar paso", http.MethodPost, one + "/steps/" + stepID, `{"files":["` + prefix + `a.jpg"]}`, true, http.StatusOK, `"status":"done"`},
		{"completar con foto sin subir", http.MethodPost, one + "/steps/" + stepID, `{"files":["` + prefix + `a.jpg"]}`, false, http.StatusUnprocessableEntity, `"files"`},
		{"ver archivo propio", http.MethodGet, one + "/files?bucket=private-docs&key=" + prefix + "a.jpg", "", true, http.StatusOK, `"url":"https://s3.test/private-docs/`},
		{"archivo de otra reserva", http.MethodGet, one + "/files?bucket=private-docs&key=" + bizA + "/bookings/otra/a.jpg", "", true, http.StatusNotFound, `"not_found"`},
		{"bucket público no", http.MethodGet, one + "/files?bucket=catalog&key=" + prefix + "a.jpg", "", true, http.StatusNotFound, `"not_found"`},
		{"salida con pasos pendientes", http.MethodPost, one + "/checkout", `{"condition":"sin ine"}`, true, http.StatusUnprocessableEntity, `"steps_pending"`},
		{"salida", http.MethodPost, one + "/checkout", `{"condition":"ok"}`, true, http.StatusOK, `"status":"checked_out"`},
		{"devolución con recargo sugerido", http.MethodPost, one + "/return", `{}`, true, http.StatusOK, `"late_fee_suggested":60000`},
		{"quitar foto de un paso", http.MethodDelete, one + "/steps/" + stepID + "/files?key=" + prefix + "a.jpg", "", true, http.StatusOK, `"status":"confirmed"`},
		{"quitar foto inexistente", http.MethodDelete, one + "/steps/" + stepID + "/files", "", true, http.StatusNotFound, `"not_found"`},
		{"subir documento PDF", http.MethodPost, one + "/uploads", `{"purpose":"document","content_type":"application/pdf","size":2000000}`, true, http.StatusCreated, `.pdf"`},
		{"PDF en evidencia de pago no", http.MethodPost, one + "/uploads", `{"purpose":"payment","content_type":"application/pdf","size":1000}`, true, http.StatusUnprocessableEntity, `"file"`},
		{"PDF en paso privado", http.MethodPost, one + "/uploads", `{"step_id":"` + stepID + `","content_type":"application/pdf","size":1000}`, true, http.StatusCreated, `.pdf"`},
		{"agregar documento", http.MethodPost, one + "/documents", `{"name":"Contrato","files":["` + prefix + `documents/a.pdf"]}`, true, http.StatusOK, `"name":"Contrato"`},
		{"documento sin nombre", http.MethodPost, one + "/documents", `{"name":"","files":[]}`, true, http.StatusUnprocessableEntity, `"name"`},
		{"quitar documento", http.MethodDelete, one + "/documents/" + stepID, "", true, http.StatusOK, `"documents":[]`},
		{"subir comprobante de pago", http.MethodPost, one + "/uploads", `{"purpose":"payment","content_type":"image/jpeg","size":1000}`, true, http.StatusCreated, `"key":"` + prefix + `payments/`},
		{"extender", http.MethodPost, one + "/extend", `{"ends_at":"2026-11-10T16:00:00Z"}`, true, http.StatusOK, `"ends_at":"2026-11-10T16:00:00Z"`},
		{"extender sin fecha", http.MethodPost, one + "/extend", `{}`, true, http.StatusUnprocessableEntity, `"ends_at"`},
		{"extender sobre otra renta", http.MethodPost, one + "/extend", `{"ends_at":"2030-01-10T16:00:00Z"}`, true, http.StatusConflict, `"slot_taken"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uploaded = tc.uploaded
			rec := doBody(tc.method, tc.path, "staff", strings.NewReader(tc.body))
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
	uploaded = true
}

type fakeSettings struct {
	s    settings.Settings
	docs []settings.DocumentType
}

func (f *fakeSettings) DocumentTypes(context.Context, string) ([]settings.DocumentType, error) {
	return f.docs, nil
}
func (f *fakeSettings) DocumentType(_ context.Context, _, id string) (settings.DocumentType, error) {
	if id != customerID { // the fake router is rebuilt per request: one known document type
		return settings.DocumentType{}, settings.ErrNotFound
	}
	return settings.DocumentType{ID: id, Name: "Pagaré", Active: true}, nil
}
func (f *fakeSettings) CreateDocumentType(_ context.Context, _ string, d settings.DocumentType) (settings.DocumentType, error) {
	d.ID = customerID
	f.docs = append(f.docs, d)
	return d, nil
}
func (f *fakeSettings) UpdateDocumentType(_ context.Context, _ string, d settings.DocumentType) (settings.DocumentType, error) {
	return d, nil
}

func TestDocumentTypeRoutes(t *testing.T) {
	base := "/v1/b/" + bizA + "/settings/documents"
	if rec := doBody(http.MethodPost, base, "staff", strings.NewReader(`{"name":"Pagaré"}`)); rec.Code != http.StatusForbidden {
		t.Fatalf("staff create: %d", rec.Code)
	}
	if rec := doBody(http.MethodPost, base, "ok", strings.NewReader(`{"name":"  "}`)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("blank name: %d", rec.Code)
	}
	rec := doBody(http.MethodPost, base, "ok", strings.NewReader(`{"name":" Pagaré ","required":true}`))
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"name":"Pagaré"`) || !strings.Contains(rec.Body.String(), `"active":true`) {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec := doBody(http.MethodPatch, base+"/"+customerID, "ok", strings.NewReader(`{"active":false}`)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"active":false`) {
		t.Fatalf("deactivate: %d %s", rec.Code, rec.Body)
	}
	if rec := doBody(http.MethodPatch, base+"/eeeeeeee-0000-0000-0000-000000000000", "ok", strings.NewReader(`{}`)); rec.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", rec.Code)
	}
	if rec := do(http.MethodGet, base, "staff"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"documents"`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
}

func (f *fakeSettings) Get(context.Context, string) (settings.Settings, error) { return f.s, nil }
func (f *fakeSettings) Save(_ context.Context, _ string, s settings.Settings) error {
	f.s = s
	return nil
}

func TestSettingsRoutes(t *testing.T) {
	path := "/v1/b/" + bizA + "/settings"
	if rec := do(http.MethodGet, path, "staff"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"late_fee_multiplier":1`) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	if rec := doBody(http.MethodPatch, path, "staff", strings.NewReader(`{"rentals":{"late_fee_multiplier":2}}`)); rec.Code != http.StatusForbidden {
		t.Fatalf("staff patch: %d", rec.Code)
	}
	rec := doBody(http.MethodPatch, path, "ok", strings.NewReader(`{"rentals":{"late_fee_multiplier":2,"security_deposit_enabled":false}}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"late_fee_multiplier":2`) || !strings.Contains(rec.Body.String(), `"security_deposit_enabled":false`) || !strings.Contains(rec.Body.String(), `"rentals_span_closed_days":true`) {
		t.Fatalf("owner patch: %d %s", rec.Code, rec.Body)
	}
	if rec := doBody(http.MethodPatch, path, "ok", strings.NewReader(`{"rentals":{"late_fee_multiplier":20}}`)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid multiplier: %d", rec.Code)
	}
	rec = doBody(http.MethodPatch, path, "ok", strings.NewReader(`{"general":{"deposit_percent":30},"delivery":{"delivery_zones":[{"name":"Centro","fee":5000}]}}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"deposit_percent":30`) || !strings.Contains(rec.Body.String(), `"name":"Centro"`) {
		t.Fatalf("general + delivery: %d %s", rec.Code, rec.Body)
	}
	if rec := doBody(http.MethodPatch, path, "ok", strings.NewReader(`{"general":{"hold_hours":0}}`)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"hold_hours"`) {
		t.Fatalf("invalid hold: %d %s", rec.Code, rec.Body)
	}
}

const customerID = "c0c0c0c0-c0c0-c0c0-c0c0-c0c0c0c0c0c0"

type fakeCustomers struct{ saved customers.Customer }

func (*fakeCustomers) Rules(context.Context, string) (customers.Rules, error) {
	return customers.Rules{RequireAddress: true, RequireGuarantor: true}, nil
}
func (*fakeCustomers) Search(_ context.Context, _, q string, _ int) ([]customers.Customer, error) {
	return []customers.Customer{{ID: customerID, Name: "Ana " + q}}, nil
}
func (*fakeCustomers) Get(_ context.Context, _, id string) (customers.Customer, error) {
	if id != customerID {
		return customers.Customer{}, customers.ErrNotFound
	}
	a, g, gp := "Calle 1", "Pedro", "+526147654321"
	return customers.Customer{ID: id, Name: "Ana", Phone: "+526141234567", Address: &a, GuarantorName: &g, GuarantorPhone: &gp}, nil
}
func (f *fakeCustomers) Create(_ context.Context, _ string, c customers.Customer) (customers.Customer, error) {
	if c.Phone == "+526140000000" {
		return customers.Customer{}, customers.ErrPhoneTaken
	}
	c.ID = customerID
	f.saved = c
	return c, nil
}
func (*fakeCustomers) IDByPhone(context.Context, string, string) (string, error) {
	return customerID, nil
}
func (f *fakeCustomers) Update(_ context.Context, _ string, c customers.Customer) (customers.Customer, error) {
	f.saved = c
	return c, nil
}
func (*fakeCustomers) AddDocuments(_ context.Context, _, id, docType string, keys []string, _ string) (customers.Customer, error) {
	return customers.Customer{ID: id, Documents: []customers.Document{{Type: docType, Key: keys[0]}}}, nil
}
func (*fakeCustomers) DeleteDocument(context.Context, string, string, string) (string, error) {
	return "k", nil
}

func TestCustomerRoutes(t *testing.T) {
	base := "/v1/b/" + bizA + "/customers"
	one := base + "/" + customerID
	prefix := bizA + "/customers/" + customerID + "/"
	full := `{"name":"Ana","phone":"614 123 4567","address":"Calle 1","guarantor_name":"Pedro","guarantor_phone":"6147654321"`
	cases := []struct {
		name, method, path, token, body string
		wantCode                        int
		wantIn                          string
	}{
		{"buscar", http.MethodGet, base + "?q=López", "staff", "", http.StatusOK, `"name":"Ana López"`},
		{"alta completa (teléfono normalizado)", http.MethodPost, base, "staff", full + `}`, http.StatusCreated, `"phone":"+526141234567"`},
		{"renta exige aval", http.MethodPost, base, "ok", `{"name":"Ana","phone":"6141234567","address":"Calle 1"}`, http.StatusUnprocessableEntity, `"guarantor_name"`},
		{"teléfono repetido", http.MethodPost, base, "ok", strings.Replace(full, "614 123 4567", "6140000000", 1) + `}`, http.StatusConflict, `"customer_id":"` + customerID},
		{"empleada no marca de confianza", http.MethodPatch, one + "/", "staff", `{"trusted":true}`, http.StatusOK, `"trusted":false`},
		{"dueño marca de confianza", http.MethodPatch, one + "/", "ok", `{"trusted":true}`, http.StatusOK, `"trusted":true`},
		{"cliente inexistente", http.MethodGet, base + "/eeeeeeee-0000-0000-0000-000000000000/", "ok", "", http.StatusNotFound, `"not_found"`},
		{"subir INE", http.MethodPost, one + "/uploads", "staff", `{"doc_type":"ine","content_type":"image/jpeg","size":500000}`, http.StatusCreated, `"key":"` + prefix},
		{"tipo de documento inválido", http.MethodPost, one + "/uploads", "staff", `{"doc_type":"pasaporte","content_type":"image/jpeg","size":1}`, http.StatusUnprocessableEntity, `"doc_type"`},
		{"registrar documento", http.MethodPost, one + "/documents", "staff", `{"doc_type":"proof_of_address","keys":["` + prefix + `a.jpg"]}`, http.StatusOK, `"doc_type":"proof_of_address"`},
		{"documento de otro cliente", http.MethodPost, one + "/documents", "staff", `{"doc_type":"ine","keys":["` + bizA + `/customers/otro/a.jpg"]}`, http.StatusUnprocessableEntity, `"keys"`},
		{"ver documento", http.MethodGet, one + "/files?key=" + prefix + "a.jpg", "staff", "", http.StatusOK, `"url":"https://s3.test/private-docs/`},
		{"documento de otro negocio", http.MethodGet, one + "/files?key=" + bizB + "/x.jpg", "staff", "", http.StatusNotFound, `"not_found"`},
		{"empleada no borra documentos", http.MethodDelete, one + "/documents/" + customerID, "staff", "", http.StatusForbidden, `"forbidden"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uploaded = true
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantIn) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

type fakePlatform struct{}

// Giros in the fake: rentals open to everyone, nails and flowers only for the platform admin, barbershop off.
var fakeVerticals = []platform.Vertical{{Vertical: "barbershop", Status: platform.Off}, {Vertical: "flowers", Status: platform.AdminOnly},
	{Vertical: "nails", Status: platform.AdminOnly}, {Vertical: "tool_rental", Status: platform.Public}}

func (fakePlatform) Verticals(context.Context) ([]platform.Vertical, error) {
	return fakeVerticals, nil
}
func (fakePlatform) VerticalStatus(_ context.Context, vertical string) (string, error) {
	for _, v := range fakeVerticals {
		if v.Vertical == vertical {
			return v.Status, nil
		}
	}
	return "", platform.ErrUnknownVertical
}
func (fakePlatform) SetVerticalStatus(_ context.Context, vertical, status string) (platform.Vertical, error) {
	if vertical == "limos" {
		return platform.Vertical{}, platform.ErrUnknownVertical
	}
	if status == "maybe" {
		return platform.Vertical{}, platform.ErrBadStatus
	}
	return platform.Vertical{Vertical: vertical, Status: status}, nil
}

func TestVerticalAvailability(t *testing.T) {
	cases := []struct {
		name, method, path, token, body string
		code                            int
		want, notWant                   string
	}{
		{"el cliente solo ve los públicos", http.MethodGet, "/v1/verticals", "staff", "", http.StatusOK, `"tool_rental"`, `"nails"`},
		{"el superadmin ve también los de prueba", http.MethodGet, "/v1/verticals", "ok", "", http.StatusOK, `{"vertical":"nails","status":"admin"}`, `"barbershop"`},
		{"cliente no abre un giro de prueba", http.MethodPost, "/v1/businesses", "staff", `{"name":"Uñas","slug":"unas-x","vertical":"nails"}`, http.StatusUnprocessableEntity, `"vertical"`, ""},
		{"cliente abre un giro público", http.MethodPost, "/v1/businesses", "staff", `{"name":"Renta","slug":"renta-x","vertical":"tool_rental"}`, http.StatusCreated, `"role":"owner"`, ""},
		{"nadie abre un giro apagado", http.MethodPost, "/v1/businesses", "ok", `{"name":"Barbería","slug":"barber-x","vertical":"barbershop"}`, http.StatusUnprocessableEntity, `"vertical"`, ""},
		{"superadmin lista los giros", http.MethodGet, "/v1/admin/verticals", "ok", "", http.StatusOK, `{"vertical":"barbershop","status":"off"}`, ""},
		{"superadmin publica un giro", http.MethodPut, "/v1/admin/verticals/nails", "ok", `{"status":"public"}`, http.StatusOK, `"status":"public"`, ""},
		{"estado desconocido", http.MethodPut, "/v1/admin/verticals/nails", "ok", `{"status":"maybe"}`, http.StatusUnprocessableEntity, `"status"`, ""},
		{"giro desconocido", http.MethodPut, "/v1/admin/verticals/limos", "ok", `{"status":"public"}`, http.StatusNotFound, `"not_found"`, ""},
		{"solo el superadmin cambia giros", http.MethodPut, "/v1/admin/verticals/nails", "staff", `{"status":"public"}`, http.StatusForbidden, `"forbidden"`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			body := rec.Body.String()
			if rec.Code != tc.code || !strings.Contains(body, tc.want) || (tc.notWant != "" && strings.Contains(body, tc.notWant)) {
				t.Fatalf("got %d %s", rec.Code, body)
			}
		})
	}
}

func (fakePlatform) Businesses(context.Context) ([]platform.Business, error) {
	return []platform.Business{{ID: bizA, Name: "Uñas", Vertical: "nails"}}, nil
}
func (fakePlatform) SetGrant(_ context.Context, id, feature string, grant *bool) (platform.Business, error) {
	if id != bizA {
		return platform.Business{}, platform.ErrNotFound
	}
	if feature == "inventory" {
		return platform.Business{}, platform.ErrNotForBusiness
	}
	return platform.Business{ID: id, Features: []features.State{{Key: feature, Grant: grant}}}, nil
}

func TestPlatformAdminRoutes(t *testing.T) {
	if rec := do(http.MethodGet, "/v1/admin/businesses", "staff"); rec.Code != http.StatusForbidden {
		t.Fatalf("not an admin: %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/v1/admin/businesses", "ok"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"vertical":"nails"`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	base := "/v1/admin/businesses/" + bizA + "/features/"
	cases := []struct {
		name, path, body string
		code             int
		want             string
	}{
		{"dar", base + "guarantor", `{"grant":true}`, http.StatusOK, `"grant":true`},
		{"volver al giro", base + "guarantor", `{"grant":null}`, http.StatusOK, `"grant":null`},
		{"no aplica al motor", base + "inventory", `{"grant":true}`, http.StatusUnprocessableEntity, `"feature"`},
		{"negocio inexistente", "/v1/admin/businesses/" + bizB + "/features/guarantor", `{"grant":true}`, http.StatusNotFound, `"not_found"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := doBody(http.MethodPut, tc.path, "ok", strings.NewReader(tc.body)); rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

type fakeFinances struct{}

func (fakeFinances) Today(context.Context, string, time.Time) (finances.Day, error) {
	return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), nil
}
func (fakeFinances) Summary(_ context.Context, _ string, from, to finances.Day, _ time.Time) (finances.Summary, error) {
	return finances.Summary{From: from, To: to, Profit: 1000}, nil
}
func (fakeFinances) Expenses(context.Context, string, finances.Day, finances.Day) ([]finances.Expense, error) {
	return []finances.Expense{}, nil
}
func (fakeFinances) AddExpense(_ context.Context, _ string, e finances.Expense, _ string) (finances.Expense, error) {
	e.ID = bookingID
	return e, nil
}
func (fakeFinances) DeleteExpense(_ context.Context, _, id string) error {
	if id != bookingID {
		return finances.ErrNotFound
	}
	return nil
}
func (fakeFinances) Plans(context.Context, string) ([]finances.Plan, error) {
	return []finances.Plan{}, nil
}
func (fakeFinances) AddPlan(_ context.Context, _ string, p finances.Plan, _ string) (finances.Plan, error) {
	return p, nil
}
func (fakeFinances) DeletePlan(context.Context, string, string) error { return nil }
func (fakeFinances) Pays(context.Context, string) ([]finances.Pay, error) {
	return []finances.Pay{}, nil
}
func (fakeFinances) SetPay(_ context.Context, _ string, p finances.Pay) (finances.Pay, error) {
	return p, nil
}

func (fakeFinances) Recurring(context.Context, string, finances.Day) ([]finances.RecurringStatus, error) {
	return []finances.RecurringStatus{}, nil
}
func (fakeFinances) GetRecurring(_ context.Context, _, id string) (finances.Recurring, error) {
	if id != itemID {
		return finances.Recurring{}, finances.ErrNotFound
	}
	return finances.Recurring{ID: id, Description: "Renta", Category: "rent", Amount: 800000, Frequency: "monthly",
		StartsOn: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}, nil
}
func (fakeFinances) AddRecurring(_ context.Context, _ string, r finances.Recurring, _ string) (finances.Recurring, error) {
	r.ID = itemID
	return r, nil
}
func (fakeFinances) DeleteRecurring(context.Context, string, string) error { return nil }
func (fakeFinances) PayCharge(_ context.Context, _ string, r finances.Recurring, due, spentOn finances.Day, amount int64, _ string) (finances.Expense, error) {
	if due.Month() == time.September { // already answered
		return finances.Expense{}, finances.ErrAnswered
	}
	return finances.Expense{ID: bookingID, SpentOn: spentOn, Category: r.Category, Amount: amount, RecurringID: &r.ID}, nil
}
func (fakeFinances) SkipCharge(context.Context, string, finances.Recurring, finances.Day) error {
	return nil
}

func TestFinanceRoutes(t *testing.T) {
	base := "/v1/b/" + bizA + "/finances"
	cases := []struct {
		name, method, path, token, body string
		code                            int
		want                            string
	}{
		{"empleado no ve finanzas", http.MethodGet, base + "/summary", "staff", "", http.StatusForbidden, `"forbidden"`},
		{"mes actual por defecto", http.MethodGet, base + "/summary", "ok", "", http.StatusOK, `"from":"2026-11-01T00:00:00Z","to":"2026-11-30T00:00:00Z"`},
		{"rango", http.MethodGet, base + "/summary?from=2026-01-01&to=2026-12-31", "ok", "", http.StatusOK, `"profit":1000`},
		{"rango al revés", http.MethodGet, base + "/summary?from=2026-12-01&to=2026-01-01", "ok", "", http.StatusUnprocessableEntity, `"from"`},
		{"gasto", http.MethodPost, base + "/expenses", "ok", `{"spent_on":"2026-11-03","category":"transport","amount":50000,"description":"Gasolina"}`, http.StatusCreated, `"category":"transport"`},
		{"gasto sin categoría", http.MethodPost, base + "/expenses", "ok", `{"spent_on":"2026-11-03","category":"x","amount":50000}`, http.StatusUnprocessableEntity, `"category"`},
		{"gasto en cero", http.MethodPost, base + "/expenses", "ok", `{"spent_on":"2026-11-03","category":"transport","amount":0}`, http.StatusUnprocessableEntity, `"amount"`},
		{"borrar gasto", http.MethodDelete, base + "/expenses/" + bookingID, "ok", "", http.StatusNoContent, ""},
		{"borrar gasto ajeno", http.MethodDelete, base + "/expenses/" + stepID, "ok", "", http.StatusNotFound, `"not_found"`},
		{"compra a plazos", http.MethodPost, base + "/plans", "ok", `{"description":"Camioneta","category":"asset","purchased_on":"2026-11-01","down_payment":5000000,"installment_amount":800000,"installments":24,"first_due":"2026-12-01"}`, http.StatusCreated, `"installments":24`},
		{"primer pago antes de la compra", http.MethodPost, base + "/plans", "ok", `{"description":"Camioneta","category":"asset","purchased_on":"2026-11-01","installment_amount":800000,"installments":24,"first_due":"2026-10-01"}`, http.StatusUnprocessableEntity, `"first_due"`},
		{"sueldo y comisión", http.MethodPut, base + "/pay/" + stepID, "ok", `{"salary":1200000,"salary_period":"monthly","commission_percent":40}`, http.StatusOK, `"commission_percent":40`},
		{"gasto recurrente", http.MethodPost, base + "/recurring", "ok", `{"description":"Renta del local","category":"rent","amount":800000,"frequency":"monthly","starts_on":"2026-08-01"}`, http.StatusCreated, `"frequency":"monthly"`},
		{"recurrente sin frecuencia", http.MethodPost, base + "/recurring", "ok", `{"description":"Renta","category":"rent","amount":800000,"starts_on":"2026-08-01"}`, http.StatusUnprocessableEntity, `"frequency"`},
		{"recurrentes", http.MethodGet, base + "/recurring", "ok", "", http.StatusOK, `"recurring":[]`},
		{"confirmar cobro con monto real", http.MethodPost, base + "/recurring/" + itemID + "/charges", "ok", `{"due_on":"2026-10-01","amount":812000}`, http.StatusCreated, `"amount":812000`},
		{"pagar por adelantado el siguiente", http.MethodPost, base + "/recurring/" + itemID + "/charges", "ok", `{"due_on":"2026-12-01","amount":800000,"spent_on":"2026-11-28"}`, http.StatusCreated, `"spent_on":"2026-11-28`},
		{"fecha que no es cobro", http.MethodPost, base + "/recurring/" + itemID + "/charges", "ok", `{"due_on":"2026-10-02","amount":800000}`, http.StatusUnprocessableEntity, `"due_on"`},
		{"cobro lejano", http.MethodPost, base + "/recurring/" + itemID + "/charges", "ok", `{"due_on":"2027-01-01","amount":800000}`, http.StatusUnprocessableEntity, `"due_on"`},
		{"cobro en cero", http.MethodPost, base + "/recurring/" + itemID + "/charges", "ok", `{"due_on":"2026-10-01","amount":0}`, http.StatusUnprocessableEntity, `"amount"`},
		{"cobro ya contestado", http.MethodPost, base + "/recurring/" + itemID + "/charges", "ok", `{"due_on":"2026-09-01","amount":800000}`, http.StatusConflict, `"already_answered"`},
		{"no se pagó", http.MethodPost, base + "/recurring/" + itemID + "/skips", "ok", `{"due_on":"2026-10-01"}`, http.StatusNoContent, ""},
		{"recurrente ajeno", http.MethodPost, base + "/recurring/" + stepID + "/skips", "ok", `{"due_on":"2026-10-01"}`, http.StatusNotFound, `"not_found"`},
		{"dejar de pagar", http.MethodDelete, base + "/recurring/" + itemID, "ok", "", http.StatusNoContent, ""},
		{"comisión > 100", http.MethodPut, base + "/pay/" + stepID, "ok", `{"salary":0,"salary_period":"monthly","commission_percent":140}`, http.StatusUnprocessableEntity, `"commission_percent"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

type fakeProducts struct{}

func (fakeProducts) Products(_ context.Context, _ string, withCost bool) ([]products.Product, error) {
	p := products.Product{ID: itemID, Name: "Cera", Price: 15000, Stock: 3, Active: true}
	if withCost {
		c := int64(7000)
		p.Cost = &c
	}
	return []products.Product{p}, nil
}
func (fakeProducts) Product(_ context.Context, _, id string) (products.Product, error) {
	c := int64(7000)
	return products.Product{ID: id, Name: "Cera", Price: 15000, Cost: &c, Stock: 3, Active: true}, nil
}
func (fakeProducts) Create(_ context.Context, _ string, p products.Product) (products.Product, error) {
	p.ID = itemID
	return p, nil
}
func (fakeProducts) Update(_ context.Context, _ string, p products.Product) (products.Product, error) {
	return p, nil
}
func (fakeProducts) Purchase(_ context.Context, _, id string, q int, _ int64, _ time.Time, _ string) (products.Product, error) {
	return products.Product{ID: id, Stock: 3 + q}, nil
}
func (fakeProducts) Sell(_ context.Context, _ string, in []products.LineInput, method, _, _ string, _ time.Time) (products.Sale, error) {
	if len(in) == 0 {
		return products.Sale{}, products.Invalid{"lines": "Agrega productos."}
	}
	return products.Sale{ID: bookingID, Method: method, Total: 15000}, nil
}
func (fakeProducts) CancelSale(context.Context, string, string) error { return nil }
func (fakeProducts) Sales(context.Context, string, time.Time, time.Time) ([]products.Sale, error) {
	return []products.Sale{}, nil
}
func (fakeProducts) StatsFor(context.Context, string, time.Time, time.Time, time.Time, time.Time) (products.Stats, error) {
	return products.Stats{Revenue: 15000, Profit: 8000}, nil
}
func (fakeProducts) Bounds(_ context.Context, _ string, from, to time.Time) (time.Time, time.Time, error) {
	return from, to.AddDate(0, 0, 1), nil
}

func TestProductRoutes(t *testing.T) {
	base := "/v1/b/" + bizA
	cases := []struct {
		name, method, path, token, body string
		code                            int
		want                            string
	}{
		{"empleado ve productos sin costo", http.MethodGet, base + "/products", "staff", "", http.StatusOK, `"cost":null`},
		{"dueño ve el costo", http.MethodGet, base + "/products", "ok", "", http.StatusOK, `"cost":7000`},
		{"empleado no crea productos", http.MethodPost, base + "/products", "staff", `{"name":"Cera","price":15000}`, http.StatusForbidden, `"forbidden"`},
		{"crear", http.MethodPost, base + "/products", "ok", `{"name":" Cera ","price":15000,"cost":7000,"stock":10}`, http.StatusCreated, `"name":"Cera"`},
		{"sin nombre", http.MethodPost, base + "/products", "ok", `{"name":"","price":1}`, http.StatusUnprocessableEntity, `"name"`},
		{"compra de mercancía", http.MethodPost, base + "/products/" + itemID + "/purchases", "ok", `{"quantity":5,"unit_cost":6500,"purchased_on":"2026-11-03"}`, http.StatusOK, `"stock":8`},
		{"compra sin fecha", http.MethodPost, base + "/products/" + itemID + "/purchases", "ok", `{"quantity":5,"unit_cost":6500}`, http.StatusUnprocessableEntity, `"purchased_on"`},
		{"empleado vende", http.MethodPost, base + "/sales", "staff", `{"lines":[{"product_id":"` + itemID + `","quantity":1}],"method":"cash"}`, http.StatusCreated, `"total":15000`},
		{"venta vacía", http.MethodPost, base + "/sales", "staff", `{"lines":[],"method":"cash"}`, http.StatusUnprocessableEntity, `"lines"`},
		{"empleado no cancela ventas", http.MethodDelete, base + "/sales/" + bookingID, "staff", "", http.StatusForbidden, `"forbidden"`},
		{"dueño cancela", http.MethodDelete, base + "/sales/" + bookingID, "ok", "", http.StatusNoContent, ""},
		{"estadísticas solo dueño", http.MethodGet, base + "/products/stats", "staff", "", http.StatusForbidden, `"forbidden"`},
		{"estadísticas", http.MethodGet, base + "/products/stats?from=2026-11-01&to=2026-11-30", "ok", "", http.StatusOK, `"profit":8000`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func (f *fakeSettings) Notice(_ context.Context, slug string) (settings.Notice, error) {
	if slug != "barber360" {
		return settings.Notice{}, settings.ErrNotFound
	}
	return settings.NoticeFrom("Barber360", nil, []byte(`{"privacy_email":"hola@barber.mx"}`)), nil
}

func TestPublicPrivacyNoticeNeedsNoSession(t *testing.T) {
	rec := do(http.MethodGet, "/v1/public/businesses/barber360/privacy", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"legal_name":"Barber360"`) || !strings.Contains(rec.Body.String(), `"email":"hola@barber.mx"`) {
		t.Fatalf("notice: %d %s", rec.Code, rec.Body)
	}
	if rec := do(http.MethodGet, "/v1/public/businesses/nadie/privacy", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown: %d", rec.Code)
	}
	// The rest of /v1 still requires a session.
	if rec := do(http.MethodGet, "/v1/me", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me without session: %d", rec.Code)
	}
}

type fakePlans struct{}

func ended() plans.Account {
	end := fixedNow.AddDate(0, 0, -1)
	return plans.Account{Plan: plans.Trial, EndsAt: &end}
}
func (fakePlans) ForUser(_ context.Context, userID string) (plans.Account, bool, error) {
	switch userID {
	case user:
		return plans.Account{Plan: plans.Pro}, true, nil
	case expiredUser:
		return ended(), true, nil
	case liteUser:
		return plans.Account{Plan: plans.Lite}, true, nil
	}
	return plans.Account{}, false, nil // never opened a business
}
func (fakePlans) ForBusiness(_ context.Context, businessID string) (plans.Account, error) {
	if businessID == bizExpired {
		return ended(), nil
	}
	if businessID == bizLite {
		return plans.Account{Plan: plans.Lite}, nil
	}
	return plans.Account{Plan: plans.Pro}, nil
}
func (fakePlans) OwnerOf(_ context.Context, businessID string) (string, error) {
	if businessID != bizA {
		return "", plans.ErrNoOwner
	}
	return user, nil
}
func (fakePlans) Set(context.Context, string, plans.Account, string) error { return nil }

// The Lite business is full: 30 items and its one person; the Lite owner already has a business.
func (fakePlans) Usage(_ context.Context, businessID, what, _ string) (int, error) {
	if businessID != bizLite {
		return 0, nil
	}
	return map[string]int{"catalog_items": 30, "staff": 1, "rental_units": 3, "photos_per_item": 3}[what], nil
}
func (fakePlans) OwnedBusinesses(_ context.Context, userID string) (int, error) {
	return map[string]int{user: 6, liteUser: 1}[userID], nil
}

func TestPlanLimits(t *testing.T) {
	lite := "/v1/b/" + bizLite
	cases := []struct {
		name, method, path, token, body string
		code                            int
		want                            string
	}{
		{"Lite: catálogo lleno", http.MethodPost, lite + "/catalog", "ok", `{"kind":"service","name":"Corte","price":1}`, http.StatusForbidden, `Tu plan Lite permite hasta 30 artículos en el catálogo.`},
		{"Lite: solo una persona atiende", http.MethodPost, lite + "/staff", "ok", `{"name":"Beto"}`, http.StatusForbidden, `"plan_limit"`},
		{"Lite: fotos por artículo", http.MethodPost, lite + "/catalog/" + itemID + "/images", "ok", `{"content_type":"image/jpeg","size":1000}`, http.StatusForbidden, `3 fotos por artículo`},
		{"Lite: sin gastos recurrentes", http.MethodGet, lite + "/finances/recurring", "ok", "", http.StatusForbidden, `"plan_capability"`},
		{"Lite: sin compras a plazos", http.MethodPost, lite + "/finances/plans", "ok", `{}`, http.StatusForbidden, `"plan_capability"`},
		{"Lite: sin sueldos", http.MethodGet, lite + "/finances/pay", "ok", "", http.StatusForbidden, `"plan_capability"`},
		{"Lite: gastos simples sí", http.MethodPost, lite + "/finances/expenses", "ok", `{"spent_on":"2026-11-03","category":"transport","amount":50000}`, http.StatusCreated, `"amount":50000`},
		{"Lite: resumen de un mes", http.MethodGet, lite + "/finances/summary?from=2026-10-01&to=2026-10-31", "ok", "", http.StatusOK, `"profit"`},
		{"Lite: resumen de un año no", http.MethodGet, lite + "/finances/summary?from=2026-01-01&to=2026-12-31", "ok", "", http.StatusForbidden, `"plan_capability"`},
		{"Pro: resumen de un año", http.MethodGet, "/v1/b/" + bizA + "/finances/summary?from=2026-01-01&to=2026-12-31", "ok", "", http.StatusOK, `"profit"`},
		{"Lite: un solo negocio", http.MethodPost, "/v1/businesses", "lite", `{"name":"Otra","slug":"otra-lite","vertical":"tool_rental"}`, http.StatusForbidden, `Tu plan Lite permite hasta 1 negocio.`},
		{"el superadmin no tiene tope de negocios", http.MethodPost, "/v1/businesses", "ok", `{"name":"Prueba","slug":"prueba-admin","vertical":"tool_rental"}`, http.StatusCreated, `"role":"owner"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestPlans(t *testing.T) {
	cases := []struct {
		name, method, path, token, body string
		code                            int
		want                            string
	}{
		{"cuenta vencida: se puede consultar", http.MethodGet, "/v1/b/" + bizExpired + "/catalog", "ok", "", http.StatusOK, `"items"`},
		{"cuenta vencida: no se registra nada", http.MethodPost, "/v1/b/" + bizExpired + "/catalog", "ok", `{"kind":"service","name":"Corte","price":1}`, http.StatusForbidden, `"plan_expired"`},
		{"cuenta vigente sí registra", http.MethodPost, "/v1/b/" + bizA + "/finances/expenses", "ok", `{"spent_on":"2026-11-03","category":"transport","amount":50000}`, http.StatusCreated, `"amount":50000`},
		{"vencido no abre otro negocio", http.MethodPost, "/v1/businesses", "vencido", `{"name":"Otra","slug":"otra-x","vertical":"tool_rental"}`, http.StatusForbidden, `"plan_expired"`},
		{"me trae el plan de cada negocio", http.MethodGet, "/v1/me", "ok", "", http.StatusOK, `"account":{"plan":"pro","status":"active","ends_at":null,"days_left":-1}`},
		{"superadmin asigna Lite hasta una fecha", http.MethodPut, "/v1/admin/businesses/" + bizA + "/plan", "ok", `{"plan":"lite","ends_at":"2026-12-31","note":"Piloto"}`, http.StatusOK, `"plan":"lite","status":"active","ends_at":"2027-01-01T00:00:00Z"`},
		{"superadmin reinicia la prueba", http.MethodPut, "/v1/admin/businesses/" + bizA + "/plan", "ok", `{"plan":"trial"}`, http.StatusOK, `"status":"trial","ends_at":"2026-11-15T18:00:00Z","days_left":14`},
		{"plan gratis no existe", http.MethodPut, "/v1/admin/businesses/" + bizA + "/plan", "ok", `{"plan":"free"}`, http.StatusUnprocessableEntity, `"plan"`},
		{"fecha inválida", http.MethodPut, "/v1/admin/businesses/" + bizA + "/plan", "ok", `{"plan":"pro","ends_at":"mañana"}`, http.StatusUnprocessableEntity, `"ends_at"`},
		{"negocio sin dueño", http.MethodPut, "/v1/admin/businesses/" + bizB + "/plan", "ok", `{"plan":"pro"}`, http.StatusNotFound, `"not_found"`},
		{"solo superadmin asigna planes", http.MethodPut, "/v1/admin/businesses/" + bizA + "/plan", "staff", `{"plan":"business"}`, http.StatusForbidden, `"forbidden"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

// staffID is the owner's own person; customerID stands for another person of the team.
func (fakeStaff) Delete(_ context.Context, _, id string) error {
	switch id {
	case staffID:
		return staff.ErrOwnerStaff
	case customerID:
		return nil
	}
	return staff.ErrNotFound
}

func (fakePlatform) Business(_ context.Context, id string) (platform.Business, error) {
	switch id {
	case bizA:
		return platform.Business{ID: id, Name: "Uñas Bellas"}, nil
	case bizExpired:
		return platform.Business{ID: id, Name: "Vencido"}, nil
	}
	return platform.Business{}, platform.ErrNotFound
}

func fakeDeleteBusiness(_ context.Context, id string) error {
	if id != bizA && id != bizExpired {
		return businesses.ErrNotFound
	}
	return nil
}

func TestDeleting(t *testing.T) {
	cases := []struct {
		name, method, path, token, body string
		code                            int
		want                            string
	}{
		{"borrar a una persona del equipo", http.MethodDelete, "/v1/b/" + bizA + "/staff/" + customerID, "ok", "", http.StatusNoContent, ""},
		{"no se borra al dueño", http.MethodDelete, "/v1/b/" + bizA + "/staff/" + staffID, "ok", "", http.StatusConflict, `"owner_staff"`},
		{"persona que no existe", http.MethodDelete, "/v1/b/" + bizA + "/staff/" + bookingID, "ok", "", http.StatusNotFound, `"not_found"`},
		{"el empleado no borra personas", http.MethodDelete, "/v1/b/" + bizA + "/staff/" + customerID, "staff", "", http.StatusForbidden, `"forbidden"`},
		{"borrar negocio pide su nombre", http.MethodDelete, "/v1/b/" + bizA, "ok", `{"confirm":"otro"}`, http.StatusUnprocessableEntity, `"confirm"`},
		{"el dueño borra su negocio", http.MethodDelete, "/v1/b/" + bizA, "ok", `{"confirm":"  uñas   bellas "}`, http.StatusNoContent, ""},
		{"el empleado no borra el negocio", http.MethodDelete, "/v1/b/" + bizA, "staff", `{"confirm":"Uñas Bellas"}`, http.StatusForbidden, `"forbidden"`},
		{"con la cuenta vencida también se puede borrar", http.MethodDelete, "/v1/b/" + bizExpired, "ok", `{"confirm":"Vencido"}`, http.StatusNoContent, ""},
		{"superadmin borra un negocio", http.MethodDelete, "/v1/admin/businesses/" + bizA, "ok", `{"confirm":"Uñas Bellas"}`, http.StatusNoContent, ""},
		{"superadmin: negocio que no existe", http.MethodDelete, "/v1/admin/businesses/" + bizB, "ok", `{"confirm":"x"}`, http.StatusNotFound, `"not_found"`},
		{"superadmin: nombre equivocado", http.MethodDelete, "/v1/admin/businesses/" + bizA, "ok", `{"confirm":"Uñas"}`, http.StatusUnprocessableEntity, `"confirm"`},
		{"solo el superadmin borra desde la plataforma", http.MethodDelete, "/v1/admin/businesses/" + bizA, "staff", `{"confirm":"Uñas Bellas"}`, http.StatusForbidden, `"forbidden"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(tc.method, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

type fakeExports struct{}

func (fakeExports) Timezone(context.Context, string) (string, error) { return "America/Chihuahua", nil }
func (fakeExports) Rows(_ context.Context, _, kind string, p *exports.Period) (any, error) {
	if kind == "customers" {
		return []exports.Customer{{Name: "Ana", Phone: "+526141234567"}}, nil
	}
	if p != nil {
		return []string{p.From.Format("2006-01-02"), p.To.Format("2006-01-02")}, nil
	}
	return []string{"all"}, nil
}

func TestExports(t *testing.T) {
	base := "/v1/b/" + bizA + "/export/"
	cases := []struct {
		name, path, token string
		code              int
		want              string
	}{
		{"clientes", base + "customers", "ok", http.StatusOK, `"rows":[{"name":"Ana"`},
		{"reservas de un periodo", base + "bookings?from=2026-10-01&to=2026-10-31", "ok", http.StatusOK, `"rows":["2026-10-01","2026-10-31"]`},
		{"sin periodo es todo", base + "expenses", "ok", http.StatusOK, `"rows":["all"]`},
		{"periodo al revés", base + "sales?from=2026-12-01&to=2026-01-01", "ok", http.StatusUnprocessableEntity, `"from"`},
		{"tipo desconocido", base + "passwords", "ok", http.StatusNotFound, `"not_found"`},
		{"solo el dueño exporta", base + "customers", "staff", http.StatusForbidden, `"forbidden"`},
		{"con la cuenta vencida también", "/v1/b/" + bizExpired + "/export/customers", "ok", http.StatusOK, `"Ana"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(http.MethodGet, tc.path, tc.token)
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

type fakeImports struct{}

func fakeSummary(n int, apply bool) imports.Summary {
	return imports.Summary{Results: []imports.Result{}, Created: n, Applied: apply}
}
func (fakeImports) Customers(_ context.Context, _ string, rows []customers.Input, _ time.Time, apply bool) (imports.Summary, error) {
	return fakeSummary(len(rows), apply), nil
}
func (fakeImports) Catalog(_ context.Context, _ string, rows []imports.CatalogRow, apply bool) (imports.Summary, error) {
	return fakeSummary(len(rows), apply), nil
}
func (fakeImports) Products(_ context.Context, _ string, rows []products.ProductInput, apply bool) (imports.Summary, error) {
	return fakeSummary(len(rows), apply), nil
}

func TestImports(t *testing.T) {
	base := "/v1/b/" + bizA + "/import/"
	many := `{"rows":[` + strings.Repeat(`{"name":"x"},`, imports.MaxRows) + `{"name":"x"}]}`
	cases := []struct {
		name, path, token, body string
		code                    int
		want                    string
	}{
		{"vista previa de clientes", base + "customers", "ok", `{"rows":[{"name":"Ana","phone":"6141234567"}]}`, http.StatusOK, `"created":1,"updated":0,"failed":0,"applied":false`},
		{"aplicar catálogo", base + "catalog?apply=true", "ok", `{"rows":[{"name":"Corte","price":15000},{"name":"Barba","price":10000}]}`, http.StatusOK, `"created":2,"updated":0,"failed":0,"applied":true`},
		{"productos", base + "products", "ok", `{"rows":[{"name":"Cera","price":15000}]}`, http.StatusOK, `"created":1`},
		{"sin filas", base + "customers", "ok", `{"rows":[]}`, http.StatusUnprocessableEntity, `"rows"`},
		{"demasiadas filas", base + "customers", "ok", many, http.StatusUnprocessableEntity, `"rows"`},
		{"tipo desconocido", base + "bookings", "ok", `{"rows":[{}]}`, http.StatusNotFound, `"not_found"`},
		{"solo el dueño importa", base + "customers", "staff", `{"rows":[{"name":"Ana"}]}`, http.StatusForbidden, `"forbidden"`},
		{"cuenta vencida no importa", "/v1/b/" + bizExpired + "/import/customers", "ok", `{"rows":[{"name":"Ana"}]}`, http.StatusForbidden, `"plan_expired"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doBody(http.MethodPost, tc.path, tc.token, strings.NewReader(tc.body))
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}
