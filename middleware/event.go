package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

type eventContextKey string

const eventKey eventContextKey = "event"

// EventFromContext retuns the Event object that was added to the context via
// WithEvent middleware.
func EventFromContext(ctx context.Context) models.Event {
	return ctx.Value(eventKey).(models.Event)
}

// WithEvent adds the event indicated in the url to the context. If the event
// cannot be found, then a 404 reponse is returned.
func WithEvent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		octx := r.Context()
		vars := mux.Vars(r)
		id := vars["eventID"]

		event, err := models.GetEventByID(octx, id)
		if err != nil {
			bjson.WriteJSON(w, map[string]string{
				"message": "Could not get event",
			}, http.StatusNotFound)
			return
		}

		nctx := context.WithValue(octx, eventKey, event)
		next.ServeHTTP(w, r.WithContext(nctx))
		return
	})
}
