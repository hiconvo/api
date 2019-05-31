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
	User      *Contact       `json:"user"     datastore:"-"`
	ThreadKey *datastore.Key `json:"-"`
	ThreadID  string         `json:"threadId" datastore:"-"`
	Body      string         `json:"body"     datastore:",noindex"`
	Timestamp time.Time      `json:"timestamp"`
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
		return err
	}

	for _, p := range ps {
		if p.Name == "ThreadKey" {
			k := p.Value.(*datastore.Key)
			m.ThreadID = k.Encode()
			break
		}
	}

	return nil
}

func NewMessage(u *User, t *Thread, body string) (Message, error) {
	return Message{
		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToContact(u),
		ThreadKey: t.Key,
		ThreadID:  t.ID,
		Body:      body,
		Timestamp: time.Now(),
	}, nil
}

func GetMessagesByThread(ctx context.Context, t *Thread) ([]*Message, error) {
	var messages []*Message
	q := datastore.NewQuery("Message").Filter("ThreadKey =", t.Key)
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
		messages[i].User = MapUserToContact(users[i])
	}

	// TODO: Get Query#Order to work above.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	return messages, nil
}
