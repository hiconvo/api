package handlers

import (
	"net/http"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
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

	if !(thread.OwnerIs(&u) || thread.HasUser(&u)) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	message, merr := models.NewMessage(&u, &thread, payload.Body)
	if merr != nil {
		bjson.HandleInternalServerError(w, merr, errMsgCreateMessage)
		return
	}

	key, kErr := db.Client.Put(ctx, message.Key, &message)
	if kErr != nil {
		bjson.HandleInternalServerError(w, kErr, errMsgSaveMessage)
		return
	}
	message.ID = key.Encode()
	message.Key = key

	if err := thread.Send(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSendMessage)
		return
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}
