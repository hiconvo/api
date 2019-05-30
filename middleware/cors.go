package middleware

import (
	"net/http"

	"github.com/gorilla/handlers"
)

func WithCORS(next http.Handler) http.Handler {
	return handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "PATCH", "POST", "DELETE"}),
	)(next)
}
