package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	og "github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/errors"
)

type Message struct {
	Key       *datastore.Key `json:"-"        datastore:"__key__"`
	ID        string         `json:"id"       datastore:"-"`
	UserKey   *datastore.Key `json:"-"`
	User      *UserPartial   `json:"user"     datastore:"-"`
	ParentKey *datastore.Key `json:"-"`
	ParentID  string         `json:"parentId" datastore:"-"`
	Body      string         `json:"body"     datastore:",noindex"`
	CreatedAt time.Time      `json:"createdAt"`
	Reads     []*Read        `json:"-"        datastore:",noindex"`
	PhotoKeys []string       `json:"-"`
	Photos    []string       `json:"photos"   datastore:"-"`
	Link      *og.LinkData   `json:"link"     datastore:",noindex"`
}

type MessageStore interface {
	GetMessageByID(ctx context.Context, id string) (*Message, error)
	GetMessagesByKey(ctx context.Context, k *datastore.Key) ([]*Message, error)
	GetMessagesByThread(ctx context.Context, t *Thread) ([]*Message, error)
	GetMessagesByEvent(ctx context.Context, t *Event) ([]*Message, error)
	GetUnhydratedMessagesByUser(ctx context.Context, u *User, p *Pagination) ([]*Message, error)
	Commit(ctx context.Context, t *Message) error
	CommitMulti(ctx context.Context, messages []*Message) error
	Delete(ctx context.Context, t *Message) error
}

func NewThreadMessage(u *User, t *Thread, body, photoKey string, link *og.LinkData) (*Message, error) {
	ts := time.Now()

	message := Message{
		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToUserPartial(u),
		ParentKey: t.Key,
		ParentID:  t.ID,
		Body:      removeLink(body, link),
		CreatedAt: ts,
		Link:      link,
	}

	if photoKey != "" {
		message.PhotoKeys = []string{photoKey}
		message.Photos = []string{photoKey}
	}

	if t.Preview == nil {
		t.Preview = &message
	}

	t.IncRespCount()

	ClearReads(t)
	MarkAsRead(t, u.Key)

	return &message, nil
}

func NewEventMessage(u *User, e *Event, body, photoKey string, link *og.LinkData) (*Message, error) {
	ts := time.Now()

	message := Message{
		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   u.Key,
		User:      MapUserToUserPartial(u),
		ParentKey: e.Key,
		ParentID:  e.ID,
		Body:      removeLink(body, link),
		CreatedAt: ts,
		Link:      link,
	}

	if photoKey != "" {
		message.PhotoKeys = []string{photoKey}
		message.Photos = []string{photoKey}
	}

	ClearReads(e)
	MarkAsRead(e, u.Key)

	return &message, nil
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

		if p.Name == "Timestamp" {
			t, ok := p.Value.(time.Time)
			if !ok {
				return errors.E(op, errors.Errorf("could not load timestamp into message='%v'", m.ID))
			}
			m.CreatedAt = t
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

func (m *Message) RestorePhotoURLs(c *storage.Client) {
	if len(m.PhotoKeys) == 0 {
		return
	}

	photos := make([]string, len(m.PhotoKeys))
	for i := range m.PhotoKeys {
		photos[i] = c.GetPhotoURLFromKey(m.PhotoKeys[i])
	}

	m.Photos = photos
}

func MarkMessagesAsRead(
	ctx context.Context,
	s MessageStore,
	u *User,
	parentKey *datastore.Key,
) error {
	op := errors.Op("model.MarkMessagesAsRead")

	messages, err := s.GetMessagesByKey(ctx, parentKey)
	if err != nil {
		return errors.E(op, err)
	}

	for i := range messages {
		MarkAsRead(messages[i], u.Key)
	}

	err = s.CommitMulti(ctx, messages)
	if err != nil {
		return errors.E(op, err)
	}

	return nil
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
