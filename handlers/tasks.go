package handlers

import (
	"encoding/json"
	"net/http"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/queue"
	"github.com/hiconvo/api/utils/bjson"
)

func CreateDigest(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.CreateDigest")

	if val := r.Header.Get("X-Appengine-Cron"); val != "true" {
		bjson.WriteJSON(w, map[string]string{
			"message": "Not found",
		}, http.StatusNotFound)
		return
	}

	ctx := r.Context()
	query := datastore.NewQuery("User")
	iter := db.Client.Run(ctx, query)

	for {
		var user models.User
		_, err := iter.Next(&user)
		if err == iterator.Done {
			break
		}
		if err != nil {
			bjson.HandleError(w, errors.E(op, err))
			return
		}

		if err := user.SendDigest(ctx); err != nil {
			log.Alarm(errors.E(op, errors.Errorf("could not send digest for user='%v': %v", user.ID, err)))
		}
	}

	bjson.WriteJSON(w, map[string]string{"message": "pass"}, http.StatusOK)
}

func SendEmailsAsync(w http.ResponseWriter, r *http.Request) {
	var op errors.Op = "handlers.SendEmailsAsync"

	if val := r.Header.Get("X-Appengine-QueueName"); val != "convo-emails" {
		bjson.WriteJSON(w, map[string]string{
			"message": "Not found",
		}, http.StatusNotFound)
		return
	}

	ctx := r.Context()
	decoder := json.NewDecoder(r.Body)

	var payload queue.EmailPayload

	if decodeErr := decoder.Decode(&payload); decodeErr != nil {
		bjson.WriteJSON(w, map[string]string{
			"message": "Could not decode JSON",
		}, http.StatusUnsupportedMediaType)
		return
	}

	for i := range payload.IDs {
		switch payload.Type {
		case queue.User:
			u, err := models.GetUserByID(ctx, payload.IDs[i])
			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}

			if payload.Action == queue.SendWelcome {
				u.Welcome(ctx)
			}
		case queue.Event:
			e, err := models.GetEventByID(ctx, payload.IDs[i])
			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}

			if payload.Action == queue.SendInvites {
				e.SendInvites(ctx)
			} else if payload.Action == queue.SendUpdatedInvites {
				e.SendUpdatedInvites(ctx)
			}
		case queue.Thread:
			t, err := models.GetThreadByID(ctx, payload.IDs[i])
			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}

			if payload.Action == queue.SendThread {
				if err := t.Send(ctx); err != nil {
					log.Alarm(errors.E(op, err))
				}
			}
		}
	}

	bjson.WriteJSON(w, map[string]string{"message": "pass"}, http.StatusOK)
}
