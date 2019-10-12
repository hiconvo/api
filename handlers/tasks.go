package handlers

import (
	"fmt"
	"net/http"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
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
