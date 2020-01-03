package db

import (
	"context"
	"os"
)

// Client is a wrappedClientImpl instance for interacting with
// Google Datastore.
var Client wrappedClient

func init() {
	ctx := context.Background()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "local-convo-api"
	}

	Client = newClient(ctx, projectID)
}
