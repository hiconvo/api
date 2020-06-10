package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

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
	"github.com/hiconvo/api/digest"
	"github.com/hiconvo/api/handler"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/template"
	"github.com/hiconvo/api/welcome"
)

const (
	readTimeout  = 5
	writeTimeout = 60
)

func main() {
	ctx := context.Background()
	projectID := getenv("GOOGLE_CLOUD_PROJECT", "local-convo-api")
	port := getenv("PORT", "8080")

	dbClient := dbc.NewClient(ctx, projectID)
	defer dbClient.Close()

	sc := secrets.NewClient(ctx, dbClient)

	raven.SetDSN(sc.Get("SENTRY_DSN", ""))
	raven.SetRelease(getenv("GAE_VERSION", "dev"))

	notifClient := notification.NewClient(
		sc.Get("STREAM_API_KEY", "streamKey"),
		sc.Get("STREAM_API_SECRET", "streamSecret"),
		"us-east")
	mailClient := mail.New(
		sender.NewClient(sc.Get("SENDGRID_API_KEY", "")),
		template.NewClient(),
	)
	searchClient := search.NewClient(sc.Get("ELASTICSEARCH_HOST", "elasticsearch"))
	storageClient := storage.NewClient(
		sc.Get("AVATAR_BUCKET_NAME", ""),
		sc.Get("PHOTO_BUCKET_NAME", ""))
	placesClient := places.NewClient(sc.Get("GOOGLE_MAPS_API_KEY", ""))
	magicClient := magic.NewClient(sc.Get("APP_SECRET", ""))

	var queueClient queue.Client
	if projectID == "local-convo-api" {
		queueClient = queue.NewLogger()
	} else {
		queueClient = queue.NewClient(ctx, projectID)
	}

	userStore := &db.UserStore{DB: dbClient, Notif: notifClient, S: searchClient, Queue: queueClient}
	threadStore := &db.ThreadStore{DB: dbClient, Storage: storageClient}
	eventStore := &db.EventStore{DB: dbClient}
	messageStore := &db.MessageStore{DB: dbClient, Storage: storageClient}

	welcomer, err := welcome.New(ctx, userStore, sc.Get("SUPPORT_PASSWORD", "support"))
	if err != nil {
		log.Fatal(err)
	}

	h := handler.New(&handler.Config{
		Transacter:    dbClient,
		UserStore:     userStore,
		ThreadStore:   threadStore,
		EventStore:    eventStore,
		MessageStore:  messageStore,
		Welcome:       welcomer,
		TxnMiddleware: dbc.WithTransaction(dbClient),
		Mail:          mailClient,
		Magic:         magicClient,
		OAuth:         oauth.NewClient(sc.Get("GOOGLE_OAUTH_KEY", "")),
		OG:            opengraph.NewClient(),
		Storage:       storageClient,
		Notif:         notifClient,
		Places:        placesClient,
		Queue:         queueClient,
		Digest: digest.New(&digest.Config{
			DB:           dbClient,
			UserStore:    userStore,
			EventStore:   eventStore,
			ThreadStore:  threadStore,
			MessageStore: messageStore,
			Mail:         mailClient,
			Magic:        magicClient,
		}),
	})

	srv := http.Server{
		Handler: h,
		Addr:    fmt.Sprintf(":%s", port),
	}

	log.Printf("Listening on port :%s", port)
	log.Fatal(srv.ListenAndServe())
}

func getenv(name, fallback string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return fallback
}
