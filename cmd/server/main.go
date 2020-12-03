package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/getsentry/raven-go"

	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	sender "github.com/hiconvo/api/clients/mail"
	"github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/oauth"
	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/places"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/clients/secrets"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/handler"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/template"
	"github.com/hiconvo/api/welcome"
)

const (
	exitCodeOK        = 0
	exitCodeInterrupt = 2
)

func main() {
	ctx := context.Background()
	projectID := getenv("GOOGLE_CLOUD_PROJECT", "local-convo-api")
	signalChan := make(chan os.Signal, 1)

	dbClient := dbc.NewClient(ctx, projectID)
	defer dbClient.Close()

	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	go func() {
		<-signalChan // first signal: clean up and exit gracefully
		log.Print("Signal detected, cleaning up")
		dbClient.Close() // close the db conn when ctl+c
		os.Exit(exitCodeOK)
		<-signalChan // second signal: hard exit
		os.Exit(exitCodeInterrupt)
	}()

	sc := secrets.NewClient(ctx, dbClient)

	raven.SetDSN(sc.Get("SENTRY_DSN", ""))
	raven.SetRelease(getenv("GAE_VERSION", "dev"))

	var (
		// clients
		notifClient   = notification.NewClient(sc.Get("STREAM_API_KEY", "streamKey"), sc.Get("STREAM_API_SECRET", "streamSecret"), "us-east")
		mailClient    = mail.New(sender.NewClient(sc.Get("SENDGRID_API_KEY", "")), template.NewClient())
		searchClient  = search.NewClient(sc.Get("ELASTICSEARCH_HOST", "elasticsearch"))
		storageClient = storage.NewClient(sc.Get("AVATAR_BUCKET_NAME", ""), sc.Get("PHOTO_BUCKET_NAME", ""))
		placesClient  = places.NewClient(sc.Get("GOOGLE_MAPS_API_KEY", ""))
		magicClient   = magic.NewClient(sc.Get("APP_SECRET", ""))
		queueClient   = queue.NewClient(ctx, projectID)
		oauthClient   = oauth.NewClient(sc.Get("GOOGLE_OAUTH_KEY", ""))
		ogClient      = opengraph.NewClient()

		// stores
		userStore    = &db.UserStore{DB: dbClient, Notif: notifClient, S: searchClient, Queue: queueClient}
		threadStore  = &db.ThreadStore{DB: dbClient}
		eventStore   = &db.EventStore{DB: dbClient}
		messageStore = &db.MessageStore{DB: dbClient}
		noteStore    = &db.NoteStore{DB: dbClient, S: searchClient}

		// welcomer
		welcomer = welcome.New(ctx, userStore, sc.Get("SUPPORT_PASSWORD", "support"))
	)

	h := handler.New(&handler.Config{
		Transacter:    dbClient,
		UserStore:     userStore,
		ThreadStore:   threadStore,
		EventStore:    eventStore,
		MessageStore:  messageStore,
		NoteStore:     noteStore,
		Welcome:       welcomer,
		TxnMiddleware: dbc.WithTransaction(dbClient),
		Mail:          mailClient,
		Magic:         magicClient,
		OAuth:         oauthClient,
		OG:            ogClient,
		Storage:       storageClient,
		Notif:         notifClient,
		Places:        placesClient,
		Queue:         queueClient,
	})

	port := getenv("PORT", "8080")
	log.Printf("Listening on port :%s", port)
	log.Panic(http.ListenAndServe(fmt.Sprintf(":%s", port), h))
}

func getenv(name, fallback string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return fallback
}
