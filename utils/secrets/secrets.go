package secrets

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/datastore"
)

type secret struct {
	Name  string
	Value string
}

var isInitialized = false
var secrets map[string]string

func Init(client *datastore.Client) {
	if isInitialized {
		return
	}
	isInitialized = true

	ctx := context.Background()
	var s []secret
	q := datastore.NewQuery("Secret")
	client.GetAll(ctx, q, &s)

	secretMap := make(map[string]string, len(s))
	for i := range s {
		secretMap[s[i].Name] = s[i].Value
	}

	secrets = secretMap
}

func Get(id string) string {
	s := secrets[id]
	if s == "" {
		fmt.Fprintf(os.Stderr, "Secret '%s' is empty\n", id)
	}
	return s
}
