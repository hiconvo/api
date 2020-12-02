package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
	"google.golang.org/api/iterator"
)

const (
	exitCodeOK        = 0
	exitCodeInterrupt = 2
)

// This command fixes photos stored in the old way on messages and
// brings all fields up to date on outdated messages.
func main() {
	var (
		isDryRun   bool
		projectID  string
		sleepTime  int = 3
		ctx            = context.Background()
		signalChan     = make(chan os.Signal, 1)
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
		dbClient.Close() // close the db conn when ctl+c
		os.Exit(exitCodeOK)
		<-signalChan // second signal: hard exit
		os.Exit(exitCodeInterrupt)
	}()

	if err := run(ctx, dbClient, isDryRun); err != nil {
		log.Panic(err)
	}
}

func run(ctx context.Context, dbClient dbc.Client, isDryRun bool) error {
	var (
		op            = errors.Op("run")
		count     int = 0
		flushSize int = 20
		queue     []*model.Message
		urlPrefix string = "https://storage.googleapis.com/convo-photos/"
	)

	flush := func() error {
		log.Printf("Flushing-> len(queue)=%d", len(queue))

		keys := make([]*datastore.Key, len(queue))
		for i := range queue {
			keys[i] = queue[i].Key
		}

		if !isDryRun {
			log.Printf("Flushing-> putting %d messages", len(keys))

			_, err := dbClient.PutMulti(ctx, keys, queue)
			if err != nil {
				return errors.E(errors.Op("flush"), err)
			}
		}

		log.Print("Flushing-> done putting messages")

		queue = queue[:0]

		log.Printf("Flushing-> len(queue)=%d", len(queue))

		return nil
	}

	iter := dbClient.Run(ctx, datastore.NewQuery("Message").Order("Timestamp"))

	log.Print("Starting loop...")

	for {
		count++

		message := new(model.Message)
		_, err := iter.Next(message)

		if errors.Is(err, iterator.Done) {
			log.Print("Done")

			return flush()
		}

		if err != nil {
			return errors.E(op, err)
		}

		log.Printf("Count=%d, MessageID=%d", count, message.Key.ID)

		if len(message.PhotoKeys) > 0 {
			for i := range message.PhotoKeys {
				if !strings.HasPrefix(message.PhotoKeys[i], "https://") {
					newURL := urlPrefix + message.PhotoKeys[i]
					message.PhotoKeys[i] = newURL
					log.Printf("Updating photo URL: %s", newURL)
				}
			}
		}

		queue = append(queue, message)

		if len(queue) >= flushSize {
			if err := flush(); err != nil {
				return errors.E(op, err)
			}
		}
	}
}
