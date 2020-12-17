package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/datastore"
	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
	"google.golang.org/api/iterator"
)

const (
	exitCodeOK = 0
)

// This command deletes the first message M of every
// thread T if M's body is identical to T's preview body.
func main() {
	var (
		isDryRun    bool
		projectID   string
		sleepTime   int = 3
		ctx, cancel     = context.WithCancel(context.Background())
		signalChan      = make(chan os.Signal, 1)
	)

	flag.BoolVar(&isDryRun, "dry-run", false, "if passed, nothing is mutated.")
	flag.StringVar(&projectID, "project-id", "local-convo-api", "overrides the default project ID.")
	flag.Parse()

	log.Printf("About to migrate messages with db=%s, dry-run=%v", projectID, isDryRun)
	log.Printf("You have %d seconds to ctl+c if this is incorrect", sleepTime)
	time.Sleep(time.Duration(sleepTime) * time.Second)

	dbClient := dbc.NewClient(ctx, projectID)
	defer dbClient.Close()

	signal.Notify(signalChan, os.Interrupt)
	defer signal.Stop(signalChan)

	go func() {
		<-signalChan // first signal: clean up and exit gracefully
		log.Print("Ctl+C detected, cleaning up")
		cancel()
		dbClient.Close() // close the db conn when ctl+c
		os.Exit(exitCodeOK)
	}()

	if err := run(ctx, dbClient, isDryRun); err != nil {
		log.Panic(err)
	}
}

func run(ctx context.Context, dbClient dbc.Client, isDryRun bool) error {
	var (
		op               = errors.Op("run")
		count        int = 0
		messageStore     = &db.MessageStore{DB: dbClient}
	)

	iter := dbClient.Run(ctx, datastore.NewQuery("Thread"))

	log.Print("Starting loop...")

	for {
		count++

		thread := new(model.Thread)
		_, err := iter.Next(&thread)

		if errors.Is(err, iterator.Done) {
			log.Printf("Done")

			return nil
		}

		if err != nil {
			return errors.E(op, err)
		}

		log.Printf("Count=%d, ThreadID=%d, Subject=%s", count, thread.Key.ID, thread.Subject)

		messages, err := messageStore.GetMessagesByThread(ctx, thread, &model.Pagination{})
		if err != nil {
			log.Panicf(err.Error())
		}

		if len(messages) == 0 {
			log.Print("CleaningCrew-> no messages in thread")
			log.Print("CleaningCrew-> skipping")
			continue
		}

		firstMessage := messages[0]

		if thread.Body != "" && firstMessage.Body == thread.Body {
			log.Print("CleaningCrew-> message has same body as thread preview")

			if isDryRun {
				log.Print("CleaningCrew-> skipping since this is a dry run")
			} else if err := messageStore.Delete(ctx, firstMessage); err != nil {
				return errors.E(op, err)
			}

			log.Printf("CleaningCrew-> deleted message id=%s", firstMessage.ID)
		} else {
			log.Print("CleaningCrew-> message and thread preview are not identical")
			log.Print("CleaningCrew-> skipping")
		}
	}
}
