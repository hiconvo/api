package handlers

import (
	"fmt"
	"html"
	"net/http"
	"os"

	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	notif "github.com/hiconvo/api/notifications"
	"github.com/hiconvo/api/utils/bjson"
	"github.com/hiconvo/api/utils/validate"
)

var (
	errMsgCreateMessage = map[string]string{"message": "Could not create message"}
	errMsgSaveMessage   = map[string]string{"message": "Could not save message"}
	errMsgSendMessage   = map[string]string{"message": "Could not send message"}
	errMsgGetMessages   = map[string]string{"message": "Could not get messages"}
)

// GetMessagesByThread Endpoint: GET /threads/{id}/messages

// GetMessagesByThread gets the messages from the given thread.
func GetMessagesByThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if !(thread.OwnerIs(&u) || thread.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	// TODO: Paginate
	messages, merr := models.GetMessagesByThread(ctx, &thread)
	if merr != nil {
		bjson.HandleInternalServerError(w, merr, errMsgGetMessages)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.Message{"messages": messages}, http.StatusOK)
}

// AddMessageToThread Endpoint: POST /threads/:id/messages
//
// Request payload:
type createMessagePayload struct {
	Body string `validate:"nonzero"`
}

// AddMessageToThread adds a message to the given thread.
func AddMessageToThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	// Validate raw data
	var payload createMessagePayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	// Check permissions
	if !(thread.OwnerIs(&u) || thread.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	message, err := models.NewThreadMessage(&u, &thread, html.UnescapeString(payload.Body))
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreateMessage)
		return
	}

	if err := message.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if err := thread.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if err := thread.Send(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSendMessage)
		return
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}

// GetMessagesByEvent Endpoint: GET /events/{id}/messages

// GetMessagesByEvent gets the messages from the given thread.
func GetMessagesByEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !(event.OwnerIs(&u) || event.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	// TODO: Paginate
	messages, merr := models.GetMessagesByEvent(ctx, &event)
	if merr != nil {
		bjson.HandleInternalServerError(w, merr, errMsgGetMessages)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.Message{"messages": messages}, http.StatusOK)
}

// AddMessageToEvent Endpoint: POST /events/:id/messages

// AddMessageToEvent adds a message to the given thread.
func AddMessageToEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	// Validate raw data
	var payload createMessagePayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	// Check permissions
	if !(event.OwnerIs(&u) || event.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	message, err := models.NewEventMessage(&u, &event, html.UnescapeString(payload.Body))
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreateMessage)
		return
	}

	if err := message.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveMessage)
		return
	}

	if err := notif.Put(notif.Notification{
		UserKeys:   notif.FilterKey(event.UserKeys, u.Key),
		Actor:      u.FullName,
		Verb:       notif.NewMessage,
		Target:     notif.Event,
		TargetID:   event.ID,
		TargetName: event.Name,
	}); err != nil {
		// Log the error but don't fail the request
		fmt.Fprintln(os.Stderr, err)
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}
