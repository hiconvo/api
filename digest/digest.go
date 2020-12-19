package digest

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
)

var NothingToDigestErr = errors.Str("Nothing to digest")

type Digester interface {
	Digest(ctx context.Context) error
}

type Config struct {
	DB           dbc.Client
	UserStore    model.UserStore
	EventStore   model.EventStore
	ThreadStore  model.ThreadStore
	MessageStore model.MessageStore
	Magic        magic.Client
	Mail         *mail.Client
}

type digesterImpl struct {
	*Config
}

func New(c *Config) Digester {
	return &digesterImpl{Config: c}
}

func (d *digesterImpl) Digest(ctx context.Context) error {
	op := errors.Op("Digest")

	keys, err := d.getUserList(ctx)
	if err != nil {
		return errors.E(op, err)
	}

	for i, k := range keys {
		log.Printf("--> digest.sendDigest(count=%d): start", i)

		var user model.User

		err := d.DB.Get(ctx, k, &user)
		if err != nil {
			log.Alarm(errors.E(op, errors.Errorf("digest.sendDigest(count=%d): %v", i, err)))

			continue
		}

		if !user.SendDigest {
			log.Printf("digest.sendDigest(count=%d): skipping digest for user=%q", i, user.Email)

			continue
		}

		if err := d.sendDigest(ctx, &user); err != nil {
			log.Alarm(errors.E(
				op,
				errors.Errorf("digest.sendDigest(count=%d): could not send digest for user=%q: %v",
					i, user.Email, err)))
		}
	}

	return nil
}

func (d *digesterImpl) getUserList(ctx context.Context) ([]*datastore.Key, error) {
	op := errors.Op("getUserList")
	yesterday := time.Now().Add(time.Duration(24) * time.Hour * -1)

	users := map[string]struct{}{}

	q := datastore.NewQuery("Event").Filter("UpdatedAt >", yesterday)
	iter := d.DB.Run(ctx, q)

	for {
		var event model.Event
		_, err := iter.Next(&event)

		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			return nil, errors.E(op, err)
		}

		for _, k := range event.UserKeys {
			users[k.Encode()] = struct{}{}
		}
	}

	q = datastore.NewQuery("Thread").Filter("UpdatedAt >", yesterday)
	iter = d.DB.Run(ctx, q)

	for {
		var thread model.Thread
		_, err := iter.Next(&thread)

		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			return nil, errors.E(op, err)
		}

		for _, k := range thread.UserKeys {
			users[k.Encode()] = struct{}{}
		}
	}

	var keys []*datastore.Key

	for s := range users {
		k, err := datastore.DecodeKey(s)
		if err != nil {
			return nil, errors.E(op, err)
		}

		keys = append(keys, k)
	}

	log.Printf("%v: found %d users who may need digest emails", op, len(keys))

	return keys, nil
}

func (d *digesterImpl) sendDigest(ctx context.Context, u *model.User) error {
	op := errors.Opf("sendDigest(user=%s)", u.Email)

	events, err := d.EventStore.GetEventsByUser(ctx, u, &model.Pagination{})
	if err != nil {
		return errors.E(op, err)
	}

	threads, err := d.ThreadStore.GetThreadsByUser(ctx, u, &model.Pagination{})
	if err != nil {
		return errors.E(op, err)
	}

	var (
		// Filter out already read events and threads
		cleanEvents  []*model.Event
		cleanThreads []*model.Thread
		// Save the upcoming events to a slice at the same time
		upcoming []*model.Event
	)

	for i := range events {
		// Get all of the unread events
		if !model.IsRead(events[i], u.Key) {
			cleanEvents = append(cleanEvents, events[i])
		}

		if events[i].IsUpcoming() {
			upcoming = append(upcoming, events[i])
		}
	}

	for i := range threads {
		// Get all of the unread threads
		if !model.IsRead(threads[i], u.Key) {
			cleanThreads = append(cleanThreads, threads[i])
		}
	}

	digestList, err := generateDigestList(ctx, d.MessageStore, cleanThreads, cleanEvents, u)
	if err != nil {
		return errors.E(op, err)
	}

	if len(digestList) > 0 || len(upcoming) > 0 {
		if err := d.Mail.SendDigest(d.Magic, digestList, upcoming, u); err != nil {
			return errors.E(op, err)
		}

		// TODO: The following two calls are bad because they're exposed to a race condition
		// so all this will have to change if there are ever a decent amount of real users
		if err := markThreadsAsRead(ctx, d.ThreadStore, cleanThreads, u); err != nil {
			return errors.E(op, err)
		}

		if err := markEventsAsRead(ctx, d.EventStore, cleanEvents, u); err != nil {
			return errors.E(op, err)
		}
	}

	log.Printf("%v: processed digest of %d items",
		op, len(digestList)+len(upcoming))

	return nil
}

func generateDigestList(
	ctx context.Context,
	ms model.MessageStore,
	threads []*model.Thread,
	events []*model.Event,
	u *model.User,
) ([]*model.DigestItem, error) {
	var op = errors.Opf("generateDigestList(user=%s)", u.Email)
	var digest []*model.DigestItem

	for i := range events {
		item, err := generateDigestItemFromEvent(ctx, ms, events[i], u)
		if err != nil {
			if errors.Is(err, NothingToDigestErr) {
				continue
			}

			return nil, errors.E(op, err)
		}

		digest = append(digest, item)
	}

	for i := range threads {
		item, err := generateDigestItemFromThread(ctx, ms, threads[i], u)
		if err != nil {
			if errors.Is(err, NothingToDigestErr) {
				continue
			}

			return nil, errors.E(op, err)
		}

		digest = append(digest, item)
	}

	return digest, nil
}

func generateDigestItemFromEvent(
	ctx context.Context,
	ms model.MessageStore,
	e *model.Event,
	u *model.User,
) (*model.DigestItem, error) {
	op := errors.Opf("generateDigestItemFromEvent(user=%s, event=%d)", u.Email, e.Key.ID)

	messages, err := ms.GetMessagesByKey(
		ctx, e.Key, &model.Pagination{Size: 5},
		db.MessagesOrderBy(db.CreatedAtNewestFirst))
	if err != nil {
		return nil, errors.E(op, err)
	}

	if len(messages) > 0 {
		log.Printf("%v: adding event with name=%s", op, e.Name)

		return &model.DigestItem{
			ParentID: e.Key,
			Name:     e.Name,
			Messages: reverse(messages),
		}, nil
	}

	return nil, NothingToDigestErr
}

func generateDigestItemFromThread(
	ctx context.Context,
	ms model.MessageStore,
	t *model.Thread,
	u *model.User,
) (*model.DigestItem, error) {
	op := errors.Opf("generateDigestItemFromThread(user=%s, thread=%d)", u.Email, t.Key.ID)

	// Get the most recent five messages
	messages, err := ms.GetMessagesByKey(
		ctx, t.Key, &model.Pagination{Size: 5},
		db.MessagesOrderBy(db.CreatedAtNewestFirst))
	if err != nil {
		return nil, errors.E(op, err)
	}

	cleanMessages := make([]*model.Message, 0)

	// If there are fewer than five messages, include the info in the thread by
	// creating a pseudo-message that can be used in DigestItem
	if len(messages) < 5 {
		firstMessage := &model.Message{
			Body:      t.Body,
			PhotoKeys: t.Photos,
			Link:      t.Link,
			User:      t.Owner,
			UserKey:   t.OwnerKey,
			ParentKey: t.Key,
			ParentID:  t.Key.Encode(),
			CreatedAt: t.CreatedAt,
		}
		cleanMessages = append(cleanMessages, messages...)
		cleanMessages = append(cleanMessages, firstMessage)
	} else {
		cleanMessages = messages
	}

	log.Printf("%v: adding thread with subject=%s", op, t.Subject)

	return &model.DigestItem{
		ParentID: t.Key,
		Name:     t.Subject,
		Messages: reverse(cleanMessages),
	}, nil
}

func markThreadsAsRead(
	ctx context.Context,
	ts model.ThreadStore,
	threads []*model.Thread,
	user *model.User,
) error {
	op := errors.Opf("markThreadsAsRead(user=%s)", user.Email)

	if len(threads) == 0 {
		return nil
	}

	for i := range threads {
		model.MarkAsRead(threads[i], user.Key)
	}

	err := ts.CommitMulti(ctx, threads)
	if err != nil {
		return errors.E(op, err)
	}

	log.Printf("%v: marked %d thread(s) as read", op, len(threads))

	return nil
}

func markEventsAsRead(
	ctx context.Context,
	es model.EventStore,
	events []*model.Event,
	user *model.User,
) error {
	op := errors.Opf("markEventsAsRead(user=%s)", user.Email)

	if len(events) == 0 {
		return nil
	}

	for i := range events {
		model.MarkAsRead(events[i], user.Key)
	}

	err := es.CommitMulti(ctx, events)
	if err != nil {
		return errors.E(op, err)
	}

	log.Printf("%v: marked %d event(s) as read", op, len(events))

	return nil
}

func reverse(messages []*model.Message) []*model.Message {
	reversedMessages := make([]*model.Message, len(messages))
	for i := range messages {
		reversedMessages[len(messages)-1-i] = messages[i]
	}

	return reversedMessages
}
