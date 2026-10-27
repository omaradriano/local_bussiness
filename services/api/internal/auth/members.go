package auth

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"slices"

	"github.com/go-chi/chi/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
)

// Members answers which role a user has in a business ("" when not a member).
type Members interface {
	Role(ctx context.Context, businessID, userID string) (string, error)
}

type Member struct {
	BusinessID string
	UserID     string
	Role       string
}

func MemberFrom(ctx context.Context) (Member, bool) {
	m, ok := ctx.Value(memberKey).(Member)
	return m, ok
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsUUID(s string) bool { return uuidRe.MatchString(s) }

// RequireMember resolves {businessId} for the authenticated user. With roles, only those may pass.
// Non-members get 403 (not 404) so the API never confirms which businesses exist.
func RequireMember(members Members, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserID(r.Context())
			if !ok {
				apierr.Write(w, http.StatusUnauthorized, "unauthorized", "Falta el token de sesión.")
				return
			}
			business := chi.URLParam(r, "businessId")
			if !uuidRe.MatchString(business) {
				apierr.Write(w, http.StatusBadRequest, "invalid_business_id", "El id del negocio no es válido.")
				return
			}
			role, err := members.Role(r.Context(), business, user)
			if err != nil {
				slog.ErrorContext(r.Context(), "resolve member", "err", err)
				apierr.Write(w, http.StatusInternalServerError, "internal", "Error interno.")
				return
			}
			if role == "" || (len(roles) > 0 && !slices.Contains(roles, role)) {
				apierr.Write(w, http.StatusForbidden, "forbidden", "No tienes acceso a este negocio.")
				return
			}
			m := Member{BusinessID: business, UserID: user, Role: role}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), memberKey, m)))
		})
	}
}

// RequireRole restricts a route inside a RequireMember group to the given roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, ok := MemberFrom(r.Context())
			if !ok || !slices.Contains(roles, m.Role) {
				apierr.Write(w, http.StatusForbidden, "forbidden", "Tu rol no permite esta acción.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
