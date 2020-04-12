package notifications

import (
	"fmt"

	"cloud.google.com/go/datastore"
	"gopkg.in/GetStream/stream-go2.v1"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/utils/secrets"
)

var DefaultClient Client

func init() {
	streamKey := secrets.Get("STREAM_API_KEY", "streamKey")
	streamSecret := secrets.Get("STREAM_API_SECRET", "streamSecret")
	DefaultClient = NewClient(streamKey, streamSecret, "us-east")
}

func Put(n Notification) error {
	return DefaultClient.Put(n)
}

func GenerateToken(userID string) string {
	return DefaultClient.GenerateToken(userID)
}

type (
	verb   string
	target string
)

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

type Client interface {
	Put(n Notification) error
	GenerateToken(userID string) string
}

type clientImpl struct {
	client *stream.Client
}

func NewClient(apiKey, apiSecret, apiRegion string) Client {
	c, err := stream.NewClient(
		apiKey,
		apiSecret,
		stream.WithAPIRegion(apiRegion))
	if err != nil {
		panic(errors.E(errors.Op("notifications.NewClient"), err))
	}

	return &clientImpl{
		client: c,
	}
}

// put is something like an adapter for stream.io. It takes the incoming notification
// and dispatches it in the appropriate way.
func (c *clientImpl) put(n Notification, userID string) error {
	feed := c.client.NotificationFeed("notification", userID)

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
func (c *clientImpl) Put(n Notification) error {
	for _, key := range n.UserKeys {
		if err := c.put(n, key.Encode()); err != nil {
			return err
		}
	}

	return nil
}

// GenerateToken generates a token for use on the frontend to retireve notifications.
func (c *clientImpl) GenerateToken(userID string) string {
	feed := c.client.NotificationFeed("notification", userID)
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
