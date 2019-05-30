package thelpers

import (
	"context"

	"cloud.google.com/go/datastore"
	"google.golang.org/appengine/aetest"
)

func CreateTestContext() (context.Context, func()) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		panic(err)
	}

	return ctx, done
}

func CreateTestDatastoreClient(ctx context.Context) *datastore.Client {
	client, err := datastore.NewClient(ctx, "")
	if err != nil {
		panic(err)
	}
	return client
}

func ClearDatastore(ctx context.Context, client *datastore.Client) {
	for _, tp := range []string{"User", "Thread", "Message"} {
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
