package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/secrets"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/model"
	"google.golang.org/api/iterator"
)

// This command deletes the first message M of every thread T if M's body is identical to T's preview body.
func main() {
	var isDryRun bool
	flag.BoolVar(&isDryRun, "dry-run", false, "if passed, deletions are not done.")
	flag.Parse()

	ctx := context.Background()
	projectID := getenv("GOOGLE_CLOUD_PROJECT", "local-convo-api")
	sleepTime := 3

	log.Printf("about to migrate first messages with db=%s, dry-run=%v", projectID, isDryRun)
	log.Printf("You have %d seconds to ctl+c if this is incorrect", sleepTime)
	time.Sleep(time.Duration(sleepTime) * time.Second)

	dbClient := dbc.NewClient(ctx, projectID)
	defer dbClient.Close()

	sc := secrets.NewClient(ctx, dbClient)
	storageClient := storage.NewClient(sc.Get("AVATAR_BUCKET_NAME", ""), sc.Get("PHOTO_BUCKET_NAME", ""))

	messageStore := &db.MessageStore{DB: dbClient, Storage: storageClient}

	log.Print("starting loop...")
	log.Print("----------")

	count := 0
	iter := dbClient.Run(ctx, datastore.NewQuery("Thread"))

	for {
		count++
		var thread model.Thread
		_, err := iter.Next(&thread)

		if errors.Is(err, iterator.Done) {
			log.Printf("done")

			break
		}

		if err != nil {
			log.Panicf(err.Error())
		}

		log.Printf("count=%d, starting thread id=%d, subject=%s", count, thread.Key.ID, thread.Subject)

		messages, err := messageStore.GetMessagesByThread(ctx, &thread)
		if err != nil {
			log.Panicf(err.Error())
		}

		if len(messages) == 0 {
			log.Print("no messages in thread, continuing...")

			log.Print("----------")
			continue
		}

		firstMessage := messages[0]

		if thread.Preview != nil && firstMessage.Body == thread.Preview.Body {
			log.Printf("message id=%d has same body as thread preview, deleting...", firstMessage.Key.ID)

			if isDryRun {
				log.Print("skipping since this is a dry run")
			} else if err := messageStore.Delete(ctx, firstMessage); err != nil {
				log.Panicf(err.Error())
			}

			log.Printf("deleted message id=%s", firstMessage.ID)
		} else {
			log.Print("message and thread preview are not identical, skipping...")
		}

		log.Print("----------")
	}
}

func getenv(name, fallback string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return fallback
}
