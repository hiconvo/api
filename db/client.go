package db

import (
	"context"
	"os"

	"cloud.google.com/go/datastore"
)

var Client *datastore.Client

func init() {
	ctx := context.Background()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "local-convo-api"
	}

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		panic(err)
	}

	Client = client
}
