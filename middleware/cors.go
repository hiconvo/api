package middleware

import (
	"net/http"

	"github.com/gorilla/handlers"
)

var corsHandler = handlers.CORS(
	handlers.AllowedOrigins([]string{"*"}),
	handlers.AllowedMethods([]string{"GET", "PATCH", "POST", "DELETE"}),
)

func WithCORS(next http.Handler) http.Handler {
	return corsHandler(next)
}
