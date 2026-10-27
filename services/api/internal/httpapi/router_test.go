package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
	"github.com/omaradriano/local_bussiness/services/api/internal/staff"
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

func (fakeCatalog) CreateUnit(_ context.Context, _ string, u catalog.Unit) (catalog.Unit, error) {
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
