// Package monitor reports errors to Sentry when SENTRY_DSN is set; without it every call is a no-op.
// Only the error and its technical context leave: request bodies, query strings, cookies,
// credentials and user data are removed (the API handles customers' personal data).
package monitor

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

var enabled bool

func Enabled() bool { return enabled }

// Init configures reporting for one service ("api" or "worker"); flush waits for queued events.
func Init(service string) (flush func(), err error) {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		enabled = false
		return func() {}, nil
	}
	err = sentry.Init(sentry.ClientOptions{
		Dsn:            dsn,
		Environment:    envOr("APP_ENV", "staging"),
		Release:        os.Getenv("RAILWAY_GIT_COMMIT_SHA"),
		ServerName:     service,
		SendDefaultPII: false,
		BeforeSend:     scrub,
	})
	enabled = err == nil
	return func() { sentry.Flush(2 * time.Second) }, err
}

// Capture reports err (what says where it happened). Uses the request's hub when there is one.
func Capture(ctx context.Context, err error, what string) {
	if !enabled || err == nil {
		return
	}
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}
	hub.WithScope(func(s *sentry.Scope) {
		s.SetTag("where", what)
		hub.CaptureException(err)
	})
}

// Middleware gives each request its own hub and reports panics (then re-panics for the recoverer).
func Middleware(next http.Handler) http.Handler {
	if !enabled {
		return next
	}
	return sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(next)
}

func scrub(e *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	e.User = sentry.User{}
	if e.Request != nil {
		e.Request.Data, e.Request.Cookies, e.Request.QueryString = "", "", ""
		for k := range e.Request.Headers {
			switch k {
			case "Authorization", "Cookie", "Idempotency-Key", "X-Forwarded-For", "X-Real-Ip":
				delete(e.Request.Headers, k)
			}
		}
	}
	return e
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
