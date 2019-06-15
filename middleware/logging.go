package middleware

import (
	"net/http"
	"os"

	"github.com/gorilla/handlers"
)

// WithLogging logs requests to stdout
func WithLogging(next http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, next)
}
