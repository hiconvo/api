package search

import (
	"fmt"
	"time"

	"github.com/olivere/elastic/v7"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
)

type Client interface {
	Update() *elastic.UpdateService
	Delete() *elastic.DeleteService
	Search(indicies ...string) *elastic.SearchService
}

func NewClient(hostname string) Client {
	var (
		op     = errors.Opf("search.NewClient(hostname=%s)", hostname)
		client *elastic.Client
		err    error
	)

	const (
		maxAttempts = 5
		timeout     = 3
	)

	for i := 1; i <= maxAttempts; i++ {
		client, err = elastic.NewClient(
			elastic.SetSniff(false),
			elastic.SetURL(fmt.Sprintf("http://%s:9200", hostname)),
		)
		if err != nil {
			if i == maxAttempts {
				panic(errors.E(op, errors.Str("Failed to connect to elasticsearch")))
			}

			log.Printf("%s: Failed to connect to elasticsearch; attempt %d/%d; will retry in %d seconds\n%s\n",
				string(op), i, maxAttempts, timeout, err)
			time.Sleep(timeout * time.Second)
		} else {
			log.Printf("%s: Connected to elasticsearch", string(op))

			break
		}
	}

	return client
}
