package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakeMembers map[[2]string]string // (business, user) -> role

func (f fakeMembers) Role(_ context.Context, businessID, userID string) (string, error) {
	if businessID == bizBoom {
		return "", errors.New("db down")
	}
	return f[[2]string{businessID, userID}], nil
}

const (
	bizA    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	bizB    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	bizBoom = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee" // the fake store fails for this one
	owner   = "11111111-1111-1111-1111-111111111111"
	staff   = "22222222-2222-2222-2222-222222222222"
)

func TestF1_3_ResuelveNegocioYRol(t *testing.T) {
	members := fakeMembers{{bizA, owner}: "owner", {bizA, staff}: "staff"}

	route := func(roles ...string) http.Handler {
		r := chi.NewRouter()
		r.Route("/v1/b/{businessId}", func(r chi.Router) {
			r.Use(RequireMember(members, roles...))
			r.Get("/x", func(w http.ResponseWriter, r *http.Request) {
				m, _ := MemberFrom(r.Context())
				w.Write([]byte(m.BusinessID + ":" + m.Role))
			})
		})
		return r
	}

	cases := []struct {
		name     string
		roles    []string
		user     string
		business string
		wantCode int
		wantBody string
	}{
		{"dueño en su negocio", nil, owner, bizA, http.StatusOK, bizA + ":owner"},
		{"empleada en su negocio", nil, staff, bizA, http.StatusOK, bizA + ":staff"},
		{"dueño en negocio ajeno", nil, owner, bizB, http.StatusForbidden, ""},
		{"empleada en ruta solo para dueño", []string{"owner"}, staff, bizA, http.StatusForbidden, ""},
		{"dueño en ruta solo para dueño", []string{"owner"}, owner, bizA, http.StatusOK, bizA + ":owner"},
		{"id de negocio inválido", nil, owner, "no-es-uuid", http.StatusBadRequest, ""},
		{"error de base", nil, owner, bizBoom, http.StatusInternalServerError, ""},
		{"sin usuario autenticado", nil, "", bizA, http.StatusUnauthorized, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/b/"+tc.business+"/x", nil)
			if tc.user != "" {
				req = req.WithContext(WithUserID(req.Context(), tc.user))
			}
			rec := httptest.NewRecorder()
			route(tc.roles...).ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantCode, rec.Body)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body, tc.wantBody)
			}
		})
	}
}
