package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/getsentry/raven-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
)

type contextKey int

const (
	userKey contextKey = iota
	threadKey
	eventKey
	noteKey
)

// WithLogging logs requests to stdout.
func WithLogging(next http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, next)
}

// WithErrorReporting reports errors to Sentry.
func WithErrorReporting(next http.Handler) http.Handler {
	return raven.Recoverer(next)
}

// nolint
var corsHandler = handlers.CORS(
	handlers.AllowedOrigins([]string{"*"}),
	handlers.AllowedMethods([]string{"GET", "PATCH", "POST", "DELETE"}),
	handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
)

// WithCORS adds OPTIONS endpoints and validates CORS permissions and validation.
func WithCORS(next http.Handler) http.Handler {
	return corsHandler(next)
}

// WithJSONRequests is middleware that ensures that a content-type of "application/json"
// is set on all write POST, PUT, and PATCH requets.
func WithJSONRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWriteRequest(r.Method) {
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				bjson.HandleError(w, errors.E(
					errors.Op("bjson.WithJSONRequests"),
					errors.Str("correct header not present"),
					http.StatusUnsupportedMediaType))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func isWriteRequest(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

// UserFromContext returns the User object that was added to the context via
// WithUser middleware.
func UserFromContext(ctx context.Context) *model.User {
	return ctx.Value(userKey).(*model.User)
}

// WithUser adds the authenticated user to the context. If the user cannot be
// found, then a 401 unauthorized response is returned.
func WithUser(s model.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var op errors.Op = "middleware.WithUser"

			if token, ok := GetAuthToken(r.Header); ok {
				ctx := r.Context()
				user, ok, err := s.GetUserByToken(ctx, token)
				if err != nil {
					bjson.HandleError(w, errors.E(op, err))
					return
				}

				if ok {
					next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, userKey, user)))
					return
				}
			}

			bjson.HandleError(w, errors.E(op, http.StatusUnauthorized, errors.Str("no token")))
		})
	}
}

// ThreadFromContext returns the Thread object that was added to the context via
// WithThread middleware.
func ThreadFromContext(ctx context.Context) *model.Thread {
	return ctx.Value(threadKey).(*model.Thread)
}

// WithThread adds the thread indicated in the url to the context. If the thread
// cannot be found, then a 404 response is returned.
func WithThread(s model.ThreadStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			vars := mux.Vars(r)
			id := vars["threadID"]

			thread, err := s.GetThreadByID(ctx, id)
			if err != nil {
				bjson.HandleError(w, errors.E(errors.Op("middleware.WithThread"), http.StatusNotFound, err))
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, threadKey, thread)))
		})
	}
}

// EventFromContext returns the Event object that was added to the context via
// WithEvent middleware.
func EventFromContext(ctx context.Context) *model.Event {
	return ctx.Value(eventKey).(*model.Event)
}

// WithEvent adds the thread indicated in the url to the context. If the thread
// cannot be found, then a 404 response is returned.
func WithEvent(s model.EventStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			vars := mux.Vars(r)
			id := vars["eventID"]

			event, err := s.GetEventByID(ctx, id)
			if err != nil {
				bjson.HandleError(w, errors.E(errors.Op("middleware.WithEvent"), http.StatusNotFound, err))
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, eventKey, event)))
		})
	}
}

// NoteFromContext returns the Note object that was added to the context via
// WithNote middleware.
func NoteFromContext(ctx context.Context) *model.Note {
	return ctx.Value(noteKey).(*model.Note)
}

// WithEvent adds the thread indicated in the url to the context. If the thread
// cannot be found, then a 404 response is returned.
func WithNote(s model.NoteStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op := errors.Op("middleware.WithNote")
			ctx := r.Context()
			u := UserFromContext(ctx)
			vars := mux.Vars(r)
			id := vars["noteID"]

			n, err := s.GetNoteByID(ctx, id)
			if err != nil {
				bjson.HandleError(w, errors.E(op, http.StatusNotFound, err))
				return
			}

			if !n.OwnerKey.Equal(u.Key) {
				bjson.HandleError(w, errors.E(
					op, errors.Str("no permission"), http.StatusNotFound))
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, noteKey, n)))
		})
	}
}

// TransactionFromContext extracts a transaction from the given
// context is one is present.
func TransactionFromContext(ctx context.Context) (db.Transaction, bool) {
	return db.TransactionFromContext(ctx)
}

// AddTransactionToContext returns a new context with a transaction added.
func AddTransactionToContext(ctx context.Context, c db.Client) (context.Context, db.Transaction, error) {
	return db.AddTransactionToContext(ctx, c)
}

// WithTransaction is middleware that adds a transaction to the request context.
func WithTransaction(c db.Client) func(http.Handler) http.Handler {
	return db.WithTransaction(c)
}

// GetAuthToken extracts the Authorization Bearer token from request
// headers if present.
func GetAuthToken(h http.Header) (string, bool) {
	if val := h.Get("Authorization"); val != "" && len(val) >= 7 {
		if strings.ToLower(val[:7]) == "bearer " {
			return val[7:], true
		}
	}

	return "", false
}
