package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

type userContextKey string

const key userContextKey = "user"

// UserFromContext retuns the User object that was added to the context via
// WithUser middleware.
func UserFromContext(ctx context.Context) models.User {
	return ctx.Value(key).(models.User)
}

// WithUser adds the authenticated user to the context. If the user cannot be
// found, then a 401 unauthorized reponse is returned.
func WithUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var op errors.Op = "middleware.WithUser"

		if token, ok := GetAuthToken(r.Header); ok {
			ctx := r.Context()
			user, ok, err := models.GetUserByToken(ctx, token)
			if err != nil {
				bjson.HandleError(w, errors.E(op, err))
				return
			}

			if ok {
				next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, key, user)))
				return
			}
		}

		bjson.HandleError(w, errors.E(op, http.StatusUnauthorized))
	})
}

// GetAuthToken extracts the Authorization Bearer token from request
// headers if present.
func GetAuthToken(h http.Header) (string, bool) {
	if val := h.Get("Authorization"); val != "" {
		if strings.ToLower(val[:7]) == "bearer " {
			return val[7:], true
		}
	}

	return "", false
}
