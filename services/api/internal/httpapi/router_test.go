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
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
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
		Settings:       &fakeSettings{r: settings.DefaultRentals()},
		Customers:      &fakeCustomers{},
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
	NewRouter(Deps{Authenticate: fakeAuth, Store: fakeStore{}, Catalog: fakeCatalog{}}).ServeHTTP(rec, req)
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
	r    settings.Rentals
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

func (f *fakeSettings) Rentals(context.Context, string) (settings.Rentals, error) { return f.r, nil }
func (f *fakeSettings) SaveRentals(_ context.Context, _ string, r settings.Rentals) error {
	f.r = r
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
}

const customerID = "c0c0c0c0-c0c0-c0c0-c0c0-c0c0c0c0c0c0"

type fakeCustomers struct{ saved customers.Customer }

func (*fakeCustomers) Vertical(context.Context, string) (string, error) { return "tool_rental", nil }
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
