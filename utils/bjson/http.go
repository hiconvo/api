package bjson

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/getsentry/raven-go"
)

var decodeErrResp []byte

func init() {
	encoded, err := json.Marshal(map[string]string{
		"message": "Could not encode JSON",
	})

	if err != nil {
		panic("Could not encode default error response")
	} else {
		decodeErrResp = encoded
	}
}

// WriteJSON writes the given interface to the response. If the interface
// cannot be marshaled, a 500 error is written instead.
func WriteJSON(w http.ResponseWriter, b interface{}, status int) {
	encoded, err := json.Marshal(b)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(decodeErrResp)
	} else {
		w.WriteHeader(status)
		w.Write(encoded)
	}
}

// HandleInternalServerError writes the given error to stderr and returns a
// 500 response with the given payload.
func HandleInternalServerError(w http.ResponseWriter, e error, b interface{}) {
	raven.CaptureError(e, nil)
	fmt.Fprintln(os.Stderr, e.Error())
	WriteJSON(w, b, http.StatusInternalServerError)
}

// WithJSON is middleware that ensures that a content-type of "application/json"
// is set on all requests and reponses. Rejects requests without this header.
func WithJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			WriteJSON(w, map[string]string{
				"message": "Unsupported content-type",
			}, http.StatusUnsupportedMediaType)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type bodyContextKey string

const key bodyContextKey = "body"

// BodyFromContext retuns the decoded JSON payload that was added to the
// context via WithJSONReqBody middleware.
func BodyFromContext(ctx context.Context) map[string]interface{} {
	return ctx.Value(key).(map[string]interface{})
}

// WithJSONReqBody decodes the bodies of incoming POST, PUT, and PATCH requests
// and adds the result to the request context. If the body cannot be decoded,
// then a 500 error is returned
func WithJSONReqBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" || r.Method == "DELETE" {
			decoder := json.NewDecoder(r.Body)

			var body map[string]interface{}

			if decodeErr := decoder.Decode(&body); decodeErr != nil {
				WriteJSON(w, map[string]string{
					"message": "Could not decode JSON",
				}, http.StatusUnsupportedMediaType)
				return
			}

			ctx := context.WithValue(r.Context(), key, body)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
