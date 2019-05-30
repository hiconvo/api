package db

import (
	"context"

	"cloud.google.com/go/datastore"
)

var Client *datastore.Client

func init() {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, "convo-api")
	if err != nil {
		panic(err)
	}

	Client = client
}
