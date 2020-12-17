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
	exitCodeOK = 0
)

// This command fixes migrates thread previews and populates the UpdatedAt field
// to support sorting by UpdatedAt.
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

	log.Printf("About to migrate threads with db=%s, dry-run=%v", projectID, isDryRun)
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
		op            = errors.Op("run")
		count     int = 0
		flushSize int = 100
		queue     []*model.Thread
		urlPrefix string = "https://storage.googleapis.com/convo-photos/"
		// t0 is when first thread messages were removed.
		t0 = time.Date(2020, time.Month(11), 23, 0, 0, 0, 0, time.UTC)
	)

	flush := func() error {
		log.Printf("Flushing-> len(queue)=%d", len(queue))

		keys := make([]*datastore.Key, len(queue))
		for i := range queue {
			keys[i] = queue[i].Key
		}

		if !isDryRun {
			log.Printf("Flushing-> putting %d threads", len(keys))

			_, err := dbClient.PutMulti(ctx, keys, queue)
			if err != nil {
				return errors.E(errors.Op("flush"), err)
			}
		}

		log.Print("Flushing-> done putting threads")

		queue = queue[:0]

		log.Printf("Flushing-> len(queue)=%d", len(queue))

		return nil
	}

	iter := dbClient.Run(ctx, datastore.NewQuery("Thread").Order("CreatedAt"))

	log.Print("Starting loop...")

	for {
		count++

		thread := new(model.Thread)
		_, err := iter.Next(thread)

		if errors.Is(err, iterator.Done) {
			log.Print("Done")

			return flush()
		}

		if err != nil {
			return errors.E(op, err)
		}

		log.Printf("Count=%d, ThreadID=%d", count, thread.Key.ID)

		if len(thread.Photos) > 0 {
			for i := range thread.Photos {
				if !strings.HasPrefix(thread.Photos[i], "https://") {
					newURL := urlPrefix + thread.Photos[i]
					thread.Photos[i] = newURL
					log.Printf("Updating photo URL: %s", newURL)
				}
			}
		}

		if thread.UpdatedAt.IsZero() {
			thread.UpdatedAt = thread.CreatedAt
		}

		if thread.CreatedAt.Before(t0) && thread.ResponseCount > 0 {
			thread.ResponseCount--
			log.Printf("Decrementing thread response count to %d", thread.ResponseCount)
		}

		queue = append(queue, thread)

		if len(queue) >= flushSize {
			if err := flush(); err != nil {
				return errors.E(op, err)
			}
		}
	}
}
