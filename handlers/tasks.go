package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/queue"
	"github.com/hiconvo/api/utils/bjson"
	"github.com/hiconvo/api/utils/reporter"
)

func CreateDigest(w http.ResponseWriter, r *http.Request) {
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
			bjson.HandleInternalServerError(w, err, map[string]string{
				"message": "Could not send all digests",
			})
			return
		}

		if err := user.SendDigest(ctx); err != nil {
			bjson.HandleInternalServerError(w, err, map[string]string{
				"message": fmt.Sprintf("Could not send digests for user %v", user.ID),
			})
			return
		}
	}

	bjson.WriteJSON(w, map[string]string{"message": "pass"}, http.StatusOK)
}

func SendEmailsAsync(w http.ResponseWriter, r *http.Request) {
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
				reporter.Report(fmt.Errorf("SendEmailsAsync: queue.User: %v", err))
				break
			}

			if payload.Action == queue.SendWelcome {
				u.Welcome(ctx)
			}
		case queue.Event:
			e, err := models.GetEventByID(ctx, payload.IDs[i])
			if err != nil {
				reporter.Report(fmt.Errorf("SendEmailsAsync: queue.Event: %v", err))
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
				reporter.Report(fmt.Errorf("SendEmailsAsync: queue.Thread: %v", err))
				break
			}

			if payload.Action == queue.SendThread {
				t.Send(ctx)
			}
		}
	}

	bjson.WriteJSON(w, map[string]string{"message": "pass"}, http.StatusOK)
}
