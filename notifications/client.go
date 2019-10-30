package notifications

import (
	"fmt"

	"gopkg.in/GetStream/stream-go2.v1"

	"github.com/hiconvo/api/utils/secrets"
)

type verb string
type target string

const (
	AddRSVP    verb = "AddRSVP"
	RemoveRSVP verb = "RemoveRSVP"
	NewMessage verb = "NewMessage"
	NewEvent   verb = "NewEvent"

	Thread target = "thread"
	Event  target = "event"
)

type Notification struct {
	UserID     string
	UserIDs    []string
	Actor      string
	Verb       verb
	Target     target
	TargetID   string
	TargetName string
}

var client *stream.Client

func init() {
	streamKey := secrets.Get("STREAM_API_KEY", "streamKey")
	streamSecret := secrets.Get("STREAM_API_SECRET", "streamSecret")

	c, err := stream.NewClient(
		streamKey,
		streamSecret,
		stream.WithAPIRegion("us-east"))
	if err != nil {
		panic(err)
	}

	client = c
}

func Put(n Notification) error {
	feed := client.NotificationFeed("notification", n.UserID)

	_, err := feed.AddActivity(stream.Activity{
		Actor:  n.Actor,
		Verb:   string(n.Verb),
		Object: fmt.Sprintf("%s:%s", string(n.Target), n.TargetID),
		Extra: map[string]interface{}{
			"targetName": n.TargetName,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func PutMulti(n Notification) error {
	for _, id := range n.UserIDs {
		n.UserID = id

		if err := Put(n); err != nil {
			return err
		}
	}

	return nil
}

func GenerateToken(userID string) string {
	feed := client.NotificationFeed("notification", userID)
	return feed.RealtimeToken(true)
}
