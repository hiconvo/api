package db

import (
	"context"
	"sort"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
)

var _ model.MessageStore = (*MessageStore)(nil)

type MessageStore struct {
	DB db.Client
}

func (s *MessageStore) GetMessageByID(ctx context.Context, id string) (*model.Message, error) {
	var (
		op      = errors.Opf("models.GetMessageByID(id=%q)", id)
		message = new(model.Message)
	)

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return message, errors.E(op, err)
	}

	err = s.DB.Get(ctx, key, message)
	if err != nil {
		return message, errors.E(op, err)
	}

	return message, nil
}

func (s *MessageStore) GetMessagesByKey(ctx context.Context, k *datastore.Key) ([]*model.Message, error) {
	var messages []*model.Message

	q := datastore.NewQuery("Message").Filter("ParentKey =", k)

	if _, err := s.DB.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	userKeys := make([]*datastore.Key, len(messages))
	for i := range messages {
		userKeys[i] = messages[i].UserKey
	}

	users := make([]*model.User, len(userKeys))
	if err := s.DB.GetMulti(ctx, userKeys, users); err != nil {
		return messages, err
	}

	for i := range messages {
		messages[i].User = model.MapUserToUserPartial(users[i])
	}

	// TODO: Get Query#Order to work above.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

func (s *MessageStore) GetMessagesByThread(ctx context.Context, t *model.Thread) ([]*model.Message, error) {
	return s.GetMessagesByKey(ctx, t.Key)
}

func (s *MessageStore) GetMessagesByEvent(ctx context.Context, e *model.Event) ([]*model.Message, error) {
	return s.GetMessagesByKey(ctx, e.Key)
}

func (s *MessageStore) GetUnhydratedMessagesByUser(
	ctx context.Context,
	u *model.User,
	p *model.Pagination,
) ([]*model.Message, error) {
	var messages []*model.Message

	q := datastore.NewQuery("Message").Filter("UserKey =", u.Key)
	if _, err := s.DB.GetAll(ctx, q, &messages); err != nil {
		return messages, err
	}

	return messages, nil
}

func (s *MessageStore) Commit(ctx context.Context, m *model.Message) error {
	key, err := s.DB.Put(ctx, m.Key, m)
	if err != nil {
		return err
	}

	m.ID = key.Encode()
	m.Key = key

	return nil
}

func (s *MessageStore) CommitMulti(ctx context.Context, messages []*model.Message) error {
	keys := make([]*datastore.Key, len(messages))
	for i := range messages {
		keys[i] = messages[i].Key
	}

	_, err := s.DB.PutMulti(ctx, keys, messages)
	if err != nil {
		return errors.E(errors.Op("MessageStore.CommitMulti"), err)
	}

	return nil
}

func (s *MessageStore) Delete(ctx context.Context, m *model.Message) error {
	if err := s.DB.Delete(ctx, m.Key); err != nil {
		return err
	}

	return nil
}
