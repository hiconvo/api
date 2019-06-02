package middleware

import (
	"net/http"

	"github.com/getsentry/raven-go"
)

func WithErrorReporting(next http.Handler) http.Handler {
	return raven.Recoverer(next)
}
