package monitor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestOffWithoutDSN(t *testing.T) {
	t.Setenv("SENTRY_DSN", "")
	flush, err := Init("api")
	if err != nil || Enabled() {
		t.Fatalf("init without dsn: enabled=%v err=%v", Enabled(), err)
	}
	flush()
	Capture(context.Background(), errors.New("boom"), "test") // no-op, no panic
	called := false
	Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !called {
		t.Fatal("middleware must pass through when off")
	}
}

func TestScrubDropsPersonalData(t *testing.T) {
	e := &sentry.Event{
		User: sentry.User{ID: "u1", Email: "a@b.mx"},
		Request: &sentry.Request{URL: "https://api/v1/b/x/customers", Method: "POST", Data: `{"name":"Ana","phone":"+52..."}`,
			Cookies: "sb=1", QueryString: "q=ana", Headers: map[string]string{"Authorization": "Bearer x", "User-Agent": "ua"}},
	}
	got := scrub(e, nil)
	if got.User.ID != "" || got.User.Email != "" || got.Request.Data != "" || got.Request.Cookies != "" || got.Request.QueryString != "" {
		t.Fatalf("personal data kept: %+v %+v", got.User, got.Request)
	}
	if _, ok := got.Request.Headers["Authorization"]; ok || got.Request.Headers["User-Agent"] != "ua" || got.Request.Method != "POST" {
		t.Fatalf("headers: %+v", got.Request.Headers)
	}
}
