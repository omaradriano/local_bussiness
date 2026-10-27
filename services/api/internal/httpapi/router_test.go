package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

const (
	user = "11111111-1111-1111-1111-111111111111"
	bizA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	bizB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

type fakeStore struct{}

func (fakeStore) Role(_ context.Context, businessID, userID string) (string, error) {
	if businessID == bizA && userID == user {
		return "owner", nil
	}
	return "", nil
}

func (fakeStore) Memberships(_ context.Context, userID string) ([]store.Membership, error) {
	if userID != user {
		return nil, nil
	}
	return []store.Membership{{BusinessID: bizA, Slug: "demo-unas", Name: "Uñas", Vertical: "nails", Role: "owner"}}, nil
}

// fakeAuth accepts "Bearer ok" as the test user; anything else is 401.
func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ok" {
			apierr.Write(w, http.StatusUnauthorized, "unauthorized", "x")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), user)))
	})
}

func do(method, path, token string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
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
