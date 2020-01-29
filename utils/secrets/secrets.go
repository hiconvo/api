package secrets

import (
	"context"
	"os"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/log"
)

type secret struct {
	Name  string
	Value string
}

var secrets map[string]string

func init() {
	ctx := context.Background()
	var s []secret
	q := datastore.NewQuery("Secret")
	db.Client.GetAll(ctx, q, &s)

	secretMap := make(map[string]string, len(s))
	for i := range s {
		secretMap[s[i].Name] = s[i].Value
	}

	secrets = secretMap
}

func Get(id, fallback string) string {
	s := secrets[id]
	if s == "" {
		log.Printf("secrets.Get: '%s' is empty, trying to read from environment...\n", id)
		s = os.Getenv(id)
	}
	if s == "" {
		log.Printf("secrets.Get: '%s' is not defined in the environment either, using fallback\n", id)
		return fallback
	}
	return s
}
