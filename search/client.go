package search

import (
	"fmt"
	"os"
	"time"

	"github.com/olivere/elastic/v7"

	"github.com/hiconvo/api/utils/secrets"
)

var Client *elastic.Client

func init() {
	esHost := secrets.Get("ELASTICSEARCH_HOST", "elasticsearch")

	var err error
	for {
		Client, err = elastic.NewClient(
			elastic.SetSniff(false),
			elastic.SetURL(fmt.Sprintf("http://%s:9200", esHost)),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize elasticsearch; will retry in three seconds.\n%s\n", err)
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}
}
