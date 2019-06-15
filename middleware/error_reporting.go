package middleware

import (
	"net/http"

	"github.com/getsentry/raven-go"
)

// WithErrorReporting reports errors to Sentry
func WithErrorReporting(next http.Handler) http.Handler {
	return raven.Recoverer(next)
}
