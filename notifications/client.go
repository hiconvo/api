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
	// NewEvent is a notification type that means a new event was created.
	NewEvent verb = "NewEvent"
	// UpdateEvent is a notification type that means an event was updated.
	UpdateEvent verb = "UpdateEvent"
	// DeleteEvent is a notification type that means an event was deleted.
	DeleteEvent verb = "DeleteEvent"
	// AddRSVP is a notification type that means someone RSVP'd to an event.
	AddRSVP verb = "AddRSVP"
	// RemoveRSVP is a notification type that means someone removed their RSVP from an event.
	RemoveRSVP verb = "RemoveRSVP"

	// NewMessage is a notification type that means a new message was sent.
	NewMessage verb = "NewMessage"

	// Thread is a notification target that associates the notification with a thread object.
	Thread target = "thread"
	// Event is a notification target that associates the notification with an event object.
	Event target = "event"
)

// A Notification contains the information needed to dispatch a notification.
type Notification struct {
	UserKeys   []*datastore.Key
	Actor      string
	Verb       verb
	Target     target
	TargetID   string
	TargetName string
}

var client *stream.Client

// Setup the stream.Client.
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

// put is something like an adapter for stream.io. It takes the incoming notification
// and dispatches it in the appropriate way.
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

// Put dispatches a notification.
func Put(n Notification) error {
	for _, key := range n.UserKeys {
		if err := put(n, key.Encode()); err != nil {
			return err
		}
	}

	return nil
}

// GenerateToken generates a token for use on the frontend to retireve notifications.
func GenerateToken(userID string) string {
	feed := client.NotificationFeed("notification", userID)
	return feed.RealtimeToken(true)
}

// FilterKey is a convenience function that filters a specific key from a slice.
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
