package handlers

import (
	"context"
	"net/http"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

func MarkThreadAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if !(thread.OwnerIs(&user) || thread.HasUser(&user)) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	if err := markMessagesAsRead(ctx, &thread, &user, thread.Key); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	models.MarkAsRead(&thread, user.Key)
	thread.UserReads = models.MapReadsToUserPartials(&thread, thread.Users)

	if err := thread.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

func MarkEventAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !(event.OwnerIs(&user) || event.HasUser(&user)) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	if err := markMessagesAsRead(ctx, &event, &user, event.Key); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	models.MarkAsRead(&event, user.Key)
	event.UserReads = models.MapReadsToUserPartials(&event, event.Users)

	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

func markMessagesAsRead(
	ctx context.Context,
	readable models.Readable,
	user *models.User,
	key *datastore.Key,
) error {
	messages, err := models.GetMessagesByKey(ctx, key)
	if err != nil {
		return err
	}

	messageKeys := make([]*datastore.Key, len(messages))
	for i := range messages {
		models.MarkAsRead(messages[i], user.Key)
		messageKeys[i] = messages[i].Key
	}

	if _, err := db.Client.PutMulti(ctx, messageKeys, messages); err != nil {
		return err
	}

	return nil
}
