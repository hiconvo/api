package search

import (
	"fmt"
	"time"

	"github.com/olivere/elastic/v7"

	"github.com/hiconvo/api/log"
)

type Client interface {
	Update() *elastic.UpdateService
	Delete() *elastic.DeleteService
	Search(indicies ...string) *elastic.SearchService
}

func NewClient(hostname string) Client {
	var (
		client *elastic.Client
		err    error
	)

	for {
		client, err = elastic.NewClient(
			elastic.SetSniff(false),
			elastic.SetURL(fmt.Sprintf("http://%s:9200", hostname)),
		)
		if err != nil {
			log.Printf("Failed to initialize elasticsearch; will retry in three seconds.\n%s\n", err)
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}

	return client
}
