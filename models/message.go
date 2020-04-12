package models

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/storage"
	og "github.com/hiconvo/api/utils/opengraph"
)

type Message struct {
	Key       *datastore.Key `json:"-"        datastore:"__key__"`
	ID        string         `json:"id"       datastore:"-"`
	UserKey   *datastore.Key `json:"-"`
	User      *UserPartial   `json:"user"     datastore:"-"`
	ParentKey *datastore.Key `json:"-"`
	ParentID  string         `json:"parentId" datastore:"-"`
	Body      string         `json:"body"     datastore:",noindex"`
	Timestamp time.Time      `json:"timestamp"`
	Reads     []*Read        `json:"-"        datastore:",noindex"`
	PhotoKeys []string       `json:"-"`
	Photos    []string       `json:"photos"   datastore:"-"`
	Link      *og.LinkData   `json:"link"     datastore:",noindex"`
}

func NewThreadMessage(u *User, t *Thread, body, photoKey string, link og.LinkData) (Message, error) {
	ts := time.Now()

	linkPtr := &link
	if link.URL == "" {
		linkPtr = nil
	}

	message := Message{
		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToUserPartial(u),
		ParentKey: t.Key,
		ParentID:  t.ID,
		Body:      removeLink(body, linkPtr),
		Timestamp: ts,
		Link:      linkPtr,
	}

	if photoKey != "" {
		message.PhotoKeys = []string{photoKey}
		message.Photos = []string{storage.DefaultClient.GetPhotoURLFromKey(photoKey)}
	}

	if t.Preview == nil {
		t.Preview = &message
	}

	t.IncRespCount()

	ClearReads(t)
	MarkAsRead(t, u.Key)

	return message, nil
}

func NewEventMessage(u *User, e *Event, body, photoKey string) (Message, error) {
	ts := time.Now()

	message := Message{
		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToUserPartial(u),
		ParentKey: e.Key,
		ParentID:  e.ID,
		Body:      body,
		Timestamp: ts,
	}

	if photoKey != "" {
		message.PhotoKeys = []string{photoKey}
		message.Photos = []string{storage.DefaultClient.GetPhotoURLFromKey(photoKey)}
	}

	ClearReads(e)
	MarkAsRead(e, u.Key)

	return message, nil
}

func (m *Message) LoadKey(k *datastore.Key) error {
	m.Key = k

	// Add URL safe key
	if k != nil {
		m.ID = k.Encode()
	}

	return nil
}

func (m *Message) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(m)
}

func (m *Message) Load(ps []datastore.Property) error {
	op := errors.Op("message.Load")

	if err := datastore.LoadStruct(m, ps); err != nil {
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if mismatch.FieldName != "ThreadKey" {
				return errors.E(op, err)
			}
		} else {
			return errors.E(op, err)
		}
	}

	for _, p := range ps {
		if p.Name == "ThreadKey" || p.Name == "ParentKey" {
			k, ok := p.Value.(*datastore.Key)
			if !ok {
				return errors.E(op, errors.Errorf("could not load parent key into message='%v'", m.ID))
			}
			m.ParentKey = k
			m.ParentID = k.Encode()
		}

		// Convert photoKeys into full URLs
		if p.Name == "PhotoKeys" {
			photoKeys, ok := p.Value.([]interface{})
			if ok {
				photos := make([]string, len(photoKeys))
				for i := range photoKeys {
					photoKey, ok := photoKeys[i].(string)
					if ok {
						photos[i] = storage.DefaultClient.GetPhotoURLFromKey(photoKey)
					}
				}

				m.Photos = photos
			}
		}
	}

	return nil
}

func (m *Message) GetReads() []*Read {
	return m.Reads
}

func (m *Message) SetReads(newReads []*Read) {
	m.Reads = newReads
}

func (m *Message) HasPhoto() bool {
	return len(m.PhotoKeys) > 0
}

func (m *Message) HasLink() bool {
	return m.Link != nil
}

func (m *Message) OwnerIs(u *User) bool {
	return m.UserKey.Equal(u.Key)
}

func (m *Message) HasPhotoKey(key string) bool {
	for i := range m.PhotoKeys {
		if m.PhotoKeys[i] == key {
			return true
		}
	}

	return false
}

// DeletePhoto deletes the given photo by key. In order to handle
// concurrent requests, or cases where photo deletion succeeds but
// updating the message fails, etc., it does not return an if the
// photo has already been deleted.
func (m *Message) DeletePhoto(ctx context.Context, key string) error {
	if m.HasPhotoKey(key) {
		for i := range m.PhotoKeys {
			if m.PhotoKeys[i] == key {
				m.PhotoKeys[i] = m.PhotoKeys[len(m.PhotoKeys)-1]
				m.PhotoKeys = m.PhotoKeys[:len(m.PhotoKeys)-1]
				break
			}
		}

		for i := range m.Photos {
			if strings.HasSuffix(m.Photos[i], key) {
				m.Photos[i] = m.Photos[len(m.Photos)-1]
				m.Photos = m.Photos[:len(m.Photos)-1]
				break
			}
		}

		if err := storage.DefaultClient.DeletePhoto(ctx, key); err != nil {
			log.Alarm(errors.E(errors.Op("models.DeletePhoto"), err))
		}
	}

	return nil
}

func (m *Message) Commit(ctx context.Context) error {
	key, err := db.DefaultClient.Put(ctx, m.Key, m)
	if err != nil {
		return err
	}

	m.ID = key.Encode()
	m.Key = key

	return nil
}

func (m *Message) Delete(ctx context.Context) error {
	if err := db.DefaultClient.Delete(ctx, m.Key); err != nil {
		return err
	}
	return nil
}

func GetMessagesByThread(ctx context.Context, t *Thread) ([]*Message, error) {
	return GetMessagesByKey(ctx, t.Key)
}

func GetMessagesByEvent(ctx context.Context, e *Event) ([]*Message, error) {
	return GetMessagesByKey(ctx, e.Key)
}

func GetMessagesByKey(ctx context.Context, k *datastore.Key) ([]*Message, error) {
	var messages []*Message

	q := datastore.NewQuery("Message").Filter("ParentKey =", k)

	if _, err := db.DefaultClient.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	userKeys := make([]*datastore.Key, len(messages))
	for i := range messages {
		userKeys[i] = messages[i].UserKey
	}
	users := make([]*User, len(userKeys))
	if err := db.DefaultClient.GetMulti(ctx, userKeys, users); err != nil {
		return messages, err
	}

	for i := range messages {
		messages[i].User = MapUserToUserPartial(users[i])
	}

	// TODO: Get Query#Order to work above.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

func GetUnhydratedMessagesByUser(ctx context.Context, u *User) ([]*Message, error) {
	var messages []*Message
	q := datastore.NewQuery("Message").Filter("UserKey =", u.Key)
	if _, err := db.DefaultClient.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	return messages, nil
}

func GetMessageByID(ctx context.Context, id string) (Message, error) {
	var op errors.Op = "models.GetMessageByID"
	var message Message

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return message, errors.E(op, err)
	}

	err = db.DefaultClient.Get(ctx, key, &message)
	if err != nil {
		return message, errors.E(op, err)
	}

	return message, nil
}

func removeLink(body string, linkPtr *og.LinkData) string {
	if linkPtr == nil {
		return body
	}

	// If this is a markdown formatted link, leave it. Otherwise, remove the link.
	// This isn't a perfect test, but it gets the job done and I'm lazy.
	if strings.Contains(body, fmt.Sprintf("[%s]", linkPtr.URL)) {
		return body
	}

	return strings.Replace(body, linkPtr.URL, "", 1)
}
