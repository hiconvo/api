// Package bjson is better json. It provides helpers for working with JSON in http handlers.
package bjson

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/getsentry/raven-go"

	"github.com/hiconvo/api/errors"
)

var encodedErrResp []byte = json.RawMessage(`{"error":"There was an internal server error while processing the request"}`)

// HandleError writes an appropriate error response to the given response
// writer. If the given error implements ErrorReporter, then the values from
// ErrorReport() and StatusCode() are written to the response, except in
// the case of a 5XX error, where the error is logged and a default message is
// written to the response.
func HandleError(w http.ResponseWriter, e error) {
	if r, ok := e.(errors.ClientReporter); ok {
		code := r.StatusCode()
		if code >= http.StatusInternalServerError {
			handleInternalServerError(w, e)
			return
		}

		WriteJSON(w, r.ClientReport(), code)
		return
	}

	handleInternalServerError(w, e)
}

// HandleInternalServerError provides backwards compatibility.
func HandleInternalServerError(w http.ResponseWriter, e error, message map[string]string) {
	ee := errors.E(errors.Op("unknown handler error"), http.StatusInternalServerError, e, message)
	handleInternalServerError(w, ee)
}

// ReadJSON unmarshals JSON from the incoming request to the given sturct pointer.
func ReadJSON(dst interface{}, r *http.Request) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		return errors.E(errors.Op("bjson.ReadJSON"), http.StatusBadRequest, err,
			map[string]string{"message": "Could not decode JSON"})
	}
	return nil
}

// WriteJSON writes the given interface to the response. If the interface
// cannot be marshaled, a 500 error is written instead.
func WriteJSON(w http.ResponseWriter, payload interface{}, status int) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		handleInternalServerError(w, errors.E(errors.Op("bjson.WriteJSON"), http.StatusInternalServerError, err))
	} else {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(encoded)
	}
}

// WithJSONRequests is middleware that ensures that a content-type of "application/json"
// is set on all write POST, PUT, and PATCH requets.
func WithJSONRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWriteRequest(r.Method) {
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				HandleError(w, errors.E(errors.Op("bjson.WithJSONRequests"), http.StatusUnsupportedMediaType))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func isWriteRequest(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

// handleInternalServerError writes the given error to stderr and returns a
// 500 response with a default message.
func handleInternalServerError(w http.ResponseWriter, e error) {
	log.Printf("Internal Server Error: %v", e)
	raven.CaptureError(e, nil)
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(encodedErrResp)
}

type bodyContextKey int

const key bodyContextKey = iota

// BodyFromContext retuns the decoded JSON payload that was added to the
// context via WithJSONRequestBody middleware.
func BodyFromContext(ctx context.Context) map[string]interface{} {
	return ctx.Value(key).(map[string]interface{})
}

// WithJSONRequestBody decodes the bodies of incoming POST, PUT, and PATCH requests
// and adds the result to the request context. If the body cannot be decoded,
// then a 500 error is returned
func WithJSONRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWriteRequest(r.Method) {
			var body map[string]interface{}

			if err := ReadJSON(&body, r); err != nil {
				HandleError(w, errors.E(errors.Op("bjson.WithJSONRequestBody"), err))
				return
			}

			ctx := context.WithValue(r.Context(), key, body)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
