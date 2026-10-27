package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testIssuer = "https://test.supabase.co/auth/v1"

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func sign(t *testing.T, k *ecdsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(k)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims(now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"sub":  "11111111-1111-1111-1111-111111111111",
		"iss":  testIssuer,
		"aud":  "authenticated",
		"role": "authenticated",
		"exp":  now.Add(time.Hour).Unix(),
		"iat":  now.Unix(),
	}
}

// serve runs the middleware and returns the status plus the user id the handler saw.
func serve(t *testing.T, mw func(http.Handler) http.Handler, header string) (int, string) {
	t.Helper()
	var seen string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = UserID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, seen
}

func TestF1_3_ApiValidaJWTDeSupabase(t *testing.T) {
	key := newKey(t)
	other := newKey(t)
	now := time.Now()
	mw := Authenticate(StaticKey(&key.PublicKey), testIssuer)

	with := func(mut func(jwt.MapClaims)) string {
		c := validClaims(now)
		mut(c)
		return "Bearer " + sign(t, key, c)
	}

	cases := []struct {
		name     string
		header   string
		wantCode int
		wantUser string
	}{
		{"token válido", with(func(jwt.MapClaims) {}), http.StatusOK, "11111111-1111-1111-1111-111111111111"},
		{"sin header", "", http.StatusUnauthorized, ""},
		{"sin Bearer", sign(t, key, validClaims(now)), http.StatusUnauthorized, ""},
		{"basura", "Bearer abc.def.ghi", http.StatusUnauthorized, ""},
		{"expirado", with(func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Minute).Unix() }), http.StatusUnauthorized, ""},
		{"otro emisor", with(func(c jwt.MapClaims) { c["iss"] = "https://otro.supabase.co/auth/v1" }), http.StatusUnauthorized, ""},
		{"audiencia anon", with(func(c jwt.MapClaims) { c["aud"] = "anon" }), http.StatusUnauthorized, ""},
		{"sin sub", with(func(c jwt.MapClaims) { delete(c, "sub") }), http.StatusUnauthorized, ""},
		{"firmado con otra llave", "Bearer " + sign(t, other, validClaims(now)), http.StatusUnauthorized, ""},
		{"alg HS256 (confusión de algoritmo)", "Bearer " + func() string {
			s, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(now)).SignedString([]byte("secreto"))
			return s
		}(), http.StatusUnauthorized, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, user := serve(t, mw, tc.header)
			if code != tc.wantCode || user != tc.wantUser {
				t.Fatalf("got (%d, %q), want (%d, %q)", code, user, tc.wantCode, tc.wantUser)
			}
		})
	}
}
