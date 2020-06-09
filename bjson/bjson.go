// Package bjson is better json. It provides helpers for working with JSON in http handlers.
package bjson

import (
	"encoding/json"
	"net/http"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
)

// nolint
var encodedErrResp []byte = json.RawMessage(`{"message":"There was an internal server error while processing the request"}`)

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

		log.Printf("Client Error: %v", e)

		WriteJSON(w, r.ClientReport(), code)

		return
	}

	handleInternalServerError(w, e)
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

// handleInternalServerError writes the given error to stderr and returns a
// 500 response with a default message.
func handleInternalServerError(w http.ResponseWriter, e error) {
	log.Alarm(e)
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(encodedErrResp)
}
