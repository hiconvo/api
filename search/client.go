package search

import (
	"fmt"
	"os"
	"time"

	"github.com/olivere/elastic/v7"
)

var Client *elastic.Client

func init() {
	var err error
	for {
		Client, err = elastic.NewClient(
			elastic.SetURL("http://elasticsearch:9200"),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize elasticsearch; will retry in three seconds.\n%s\n", err)
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}
}
