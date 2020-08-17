package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewClient(ctx context.Context, uri string) (*mongo.Client, func()) {
	client, err := mongo.NewClient(options.Client().ApplyURI(fmt.Sprintf("mongodb://%s", uri)))
	if err != nil {
		panic(err)
	}

	if err := client.Connect(ctx); err != nil {
		panic(err)
	}

	closer := func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}

	return client, closer
}
