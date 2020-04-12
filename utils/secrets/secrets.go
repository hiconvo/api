package secrets

import (
	"context"
	"os"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/log"
)

var DefaultClient Client

func init() {
	DefaultClient = NewClient(context.Background(), db.DefaultClient)
}

func Get(id, fallback string) string {
	return DefaultClient.Get(id, fallback)
}

type Client interface {
	Get(id, fallback string) string
}

type clientImpl struct {
	secrets map[string]string
}

func NewClient(ctx context.Context, db db.Client) Client {
	var s []struct {
		Name  string
		Value string
	}

	q := datastore.NewQuery("Secret")
	db.GetAll(ctx, q, &s)

	secretMap := make(map[string]string, len(s))
	for i := range s {
		secretMap[s[i].Name] = s[i].Value
	}

	return &clientImpl{
		secrets: secretMap,
	}
}

func (c *clientImpl) Get(id, fallback string) string {
	s := c.secrets[id]
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
