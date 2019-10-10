package thelpers

import (
	"context"

	"cloud.google.com/go/datastore"
)

func CreateTestContext() context.Context {
	return context.Background()
}

func CreateTestDatastoreClient(ctx context.Context) *datastore.Client {
	client, err := datastore.NewClient(ctx, "convo-api")
	if err != nil {
		panic(err)
	}
	return client
}

func ClearDatastore(ctx context.Context, client *datastore.Client) {
	for _, tp := range []string{"User", "Thread", "Event", "Message"} {
		q := datastore.NewQuery(tp).KeysOnly()
		keys, err := client.GetAll(ctx, q, nil)
		if err != nil {
			panic(err)
		}
		derr := client.DeleteMulti(ctx, keys)
		if err != nil {
			panic(derr)
		}
	}
}
