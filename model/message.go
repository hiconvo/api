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
	PhotoKeys []string       `json:"photos"`
	Link      *og.LinkData   `json:"link"     datastore:",noindex"`
}

type GetMessagesOption func(m map[string]interface{})

type MessageStore interface {
	GetMessageByID(ctx context.Context, id string) (*Message, error)
	GetMessagesByKey(ctx context.Context,
		k *datastore.Key, p *Pagination, o ...GetMessagesOption) ([]*Message, error)
	GetMessagesByThread(ctx context.Context,
		t *Thread, p *Pagination, o ...GetMessagesOption) ([]*Message, error)
	GetMessagesByEvent(ctx context.Context,
		t *Event, p *Pagination, o ...GetMessagesOption) ([]*Message, error)
	GetUnhydratedMessagesByUser(ctx context.Context,
		u *User, p *Pagination, o ...GetMessagesOption) ([]*Message, error)
	Commit(ctx context.Context, t *Message) error
	CommitMulti(ctx context.Context, messages []*Message) error
	Delete(ctx context.Context, t *Message) error
}

type NewMessageInput struct {
	User   *User
	Parent *datastore.Key
	Body   string
	Blob   string
}

func NewMessage(
	ctx context.Context,
	sclient *storage.Client,
	ogclient og.Client,
	input *NewMessageInput,
) (*Message, error) {
	var (
		op       = errors.Op("model.NewThreadMessage")
		ts       = time.Now()
		photoURL string
		err      error
	)

	link, photoURL, err := handleLinkAndPhoto(
		ctx, sclient, ogclient, input.Parent, input.Body, input.Blob)
	if err != nil {
		return nil, errors.E(op, err)
	}

	message := Message{
		Key:       datastore.IncompleteKey("Message", nil),
		UserKey:   input.User.Key,
		User:      MapUserToUserPartial(input.User),
		ParentKey: input.Parent,
		ParentID:  input.Parent.Encode(),
		Body:      removeLink(input.Body, link),
		CreatedAt: ts,
		Link:      link,
	}

	if photoURL != "" {
		message.PhotoKeys = []string{photoURL}
	}

	MarkAsRead(&message, input.User.Key)

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
		return errors.E(op, err)
	}

	for _, p := range ps {
		if p.Name == "ParentKey" {
			k, ok := p.Value.(*datastore.Key)
			if !ok {
				return errors.E(op, errors.Errorf("could not load parent key into message='%v'", m.Key.ID))
			}
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

func MarkMessagesAsRead(
	ctx context.Context,
	s MessageStore,
	u *User,
	parentKey *datastore.Key,
) error {
	op := errors.Op("model.MarkMessagesAsRead")

	messages, err := s.GetMessagesByKey(ctx, parentKey, &Pagination{Size: 50})
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
	if strings.Contains(body, fmt.Sprintf("[%s]", linkPtr.Original)) {
		return body
	}

	return strings.Replace(body, linkPtr.Original, "", 1)
}

func handleLinkAndPhoto(
	ctx context.Context,
	sclient *storage.Client,
	ogclient og.Client,
	key *datastore.Key,
	body, blob string,
) (*og.LinkData, string, error) {
	var (
		op       = errors.Op("model.handleLinkAndPhoto")
		photoURL string
		err      error
	)

	if blob != "" {
		photoURL, err = sclient.PutPhotoFromBlob(ctx, key.Encode(), blob)
		if err != nil {
			return nil, "", errors.E(op, err)
		}
	}

	link := ogclient.Extract(ctx, body)

	return link, photoURL, nil
}
