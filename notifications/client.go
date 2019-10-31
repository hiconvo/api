package notifications

import (
	"fmt"

	"cloud.google.com/go/datastore"
	"gopkg.in/GetStream/stream-go2.v1"

	"github.com/hiconvo/api/utils/secrets"
)

type verb string
type target string

const (
	NewEvent    verb = "NewEvent"
	UpdateEvent verb = "UpdateEvent"
	DeleteEvent verb = "DeleteEvent"
	AddRSVP     verb = "AddRSVP"
	RemoveRSVP  verb = "RemoveRSVP"

	NewMessage verb = "NewMessage"

	Thread target = "thread"
	Event  target = "event"
)

type Notification struct {
	UserKeys   []*datastore.Key
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

func put(n Notification, userID string) error {
	feed := client.NotificationFeed("notification", userID)

	_, err := feed.AddActivity(stream.Activity{
		Actor:  n.Actor,
		Verb:   string(n.Verb),
		Object: fmt.Sprintf("%s:%s", string(n.Target), n.TargetID),
		Target: string(n.Target),
		Extra: map[string]interface{}{
			"targetName": n.TargetName,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func Put(n Notification) error {
	for _, key := range n.UserKeys {
		if err := put(n, key.Encode()); err != nil {
			return err
		}
	}

	return nil
}

func GenerateToken(userID string) string {
	feed := client.NotificationFeed("notification", userID)
	return feed.RealtimeToken(true)
}

func FilterKey(keys []*datastore.Key, toFilter *datastore.Key) []*datastore.Key {
	var filtered []*datastore.Key
	for i := range keys {
		if keys[i].Equal(toFilter) {
			continue
		}

		filtered = append(filtered, keys[i])
	}

	return filtered
}
