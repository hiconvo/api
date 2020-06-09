package notification

import (
	"fmt"
	"log"

	"cloud.google.com/go/datastore"
	"gopkg.in/GetStream/stream-go2.v1"

	"github.com/hiconvo/api/errors"
)

type (
	verb   string
	target string
)

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
	Put(n *Notification) error
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
		panic(errors.E(errors.Op("notification.NewClient"), err))
	}

	return &clientImpl{
		client: c,
	}
}

// put is something like an adapter for stream.io. It takes the incoming notification
// and dispatches it in the appropriate way.
func (c *clientImpl) put(n *Notification, userID string) error {
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
func (c *clientImpl) Put(n *Notification) error {
	for _, key := range n.UserKeys {
		if err := c.put(n, key.Encode()); err != nil {
			return err
		}
	}

	return nil
}

// GenerateToken generates a token for use on the frontend to retrieve notifications.
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

type logger struct{}

func NewLogger() Client {
	log.Print("notification.NewLogger: USING NOTIFICATION LOGGER FOR LOCAL DEVELOPMENT")
	return &logger{}
}

func (l *logger) Put(n *Notification) error {
	log.Printf("notification.Put(Actor=%s, Verb=%s, Target=%s, TargetID=%s)",
		n.Actor, n.Verb, string(n.Target), n.TargetID)
	return nil
}

func (l *logger) GenerateToken(userID string) string {
	return "nullToken"
}
