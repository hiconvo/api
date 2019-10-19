package models

import (
	"context"
	"sort"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
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
}

func (m *Message) LoadKey(k *datastore.Key) error {
	m.Key = k

	// Add URL safe key
	m.ID = k.Encode()

	return nil
}

func (m *Message) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(m)
}

func (m *Message) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(m, ps); err != nil {
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if mismatch.FieldName != "ThreadKey" {
				return err
			}
		} else {
			return err
		}
	}

	for _, p := range ps {
		if p.Name == "ThreadKey" || p.Name == "ParentKey" {
			k := p.Value.(*datastore.Key)
			m.ParentKey = k
			m.ParentID = k.Encode()
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

func (m *Message) Commit(ctx context.Context) error {
	key, kErr := db.Client.Put(ctx, m.Key, m)
	if kErr != nil {
		return kErr
	}
	m.ID = key.Encode()
	m.Key = key
	return nil
}

func NewThreadMessage(u *User, t *Thread, body string) (Message, error) {
	ts := time.Now()

	message := Message{
		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToUserPartial(u),
		ParentKey: t.Key,
		ParentID:  t.ID,
		Body:      body,
		Timestamp: ts,
	}

	t.Preview = &Preview{
		Body:      body,
		Sender:    MapUserToUserPartial(u),
		Timestamp: ts,
	}

	ClearReads(t)
	MarkAsRead(t, u.Key)

	return message, nil
}

func NewEventMessage(u *User, e *Event, body string) (Message, error) {
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

	ClearReads(e)
	MarkAsRead(e, u.Key)

	return message, nil
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
	// TODO: Paginate to avoid potential memory overflow.
	if _, err := db.Client.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	userKeys := make([]*datastore.Key, len(messages))
	for i := range messages {
		userKeys[i] = messages[i].UserKey
	}
	users := make([]*User, len(userKeys))
	if err := db.Client.GetMulti(ctx, userKeys, users); err != nil {
		return messages, err
	}

	for i := range messages {
		messages[i].User = MapUserToUserPartial(users[i])
	}

	// TODO: Get Query#Order to work above.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	return messages, nil
}

func GetUnhydratedMessagesByUser(ctx context.Context, u *User) ([]*Message, error) {
	var messages []*Message
	q := datastore.NewQuery("Message").Filter("UserKey =", u.Key)
	if _, err := db.Client.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	return messages, nil
}
