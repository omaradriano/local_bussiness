// Package auth validates Supabase JWTs and resolves the caller's business and role.
package auth

import (
	"context"
	"crypto/ecdsa"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
)

type ctxKey int

const (
	userKey ctxKey = iota
	memberKey
)

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userKey, id)
}

func UserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userKey).(string)
	return id, ok && id != ""
}

// JWKS fetches and refreshes the project's signing keys from Supabase.
func JWKS(ctx context.Context, supabaseURL string) (jwt.Keyfunc, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"})
	if err != nil {
		return nil, err
	}
	return k.Keyfunc, nil
}

// StaticKey is for tests: every token is verified against one public key.
func StaticKey(pub *ecdsa.PublicKey) jwt.Keyfunc {
	return func(*jwt.Token) (any, error) { return pub, nil }
}

// Authenticate requires a valid Supabase access token (ES256, our issuer, audience "authenticated")
// and puts the user id (sub) in the request context.
func Authenticate(keys jwt.Keyfunc, issuer string) func(http.Handler) http.Handler {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				apierr.Write(w, http.StatusUnauthorized, "unauthorized", "Falta el token de sesión.")
				return
			}
			tok, err := parser.Parse(raw, keys)
			if err != nil || !tok.Valid {
				apierr.Write(w, http.StatusUnauthorized, "unauthorized", "Sesión inválida o expirada.")
				return
			}
			sub, err := tok.Claims.GetSubject()
			if err != nil || sub == "" {
				apierr.Write(w, http.StatusUnauthorized, "unauthorized", "Sesión inválida o expirada.")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), sub)))
		})
	}
}
