package secrets

import (
	"context"
	"os"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
)

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
	if _, err := db.GetAll(ctx, q, &s); err != nil {
		panic(errors.E(errors.Op("secrets.NewClient()"), err))
	}

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
		s = os.Getenv(id)
	}

	if s == "" {
		log.Printf("secrets.Get(id=%s): using fallback", id)
		return fallback
	}

	return s
}
