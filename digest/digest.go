package digest

import (
	"context"

	"google.golang.org/api/iterator"

	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
)

type Digester interface {
	Digest(ctx context.Context) error
}

type Config struct {
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
	// TODO: Rewrite this whole thing so it's not garbage.
	op := errors.Op("digest.Digest")
	iter := d.UserStore.IterAll(ctx)

	for {
		var user model.User
		_, err := iter.Next(&user)

		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			return errors.E(op, err)
		}

		if !user.SendDigest {
			log.Printf("digest.sendDigest: skipping digest for user=%q", user.Email)
			continue
		}

		if err := d.sendDigest(ctx, &user); err != nil {
			log.Alarm(errors.E(
				op,
				errors.Errorf("digest.sendDigest: could not send digest for user=%q: %v", user.Email, err)))
		}
	}

	return nil
}

func (d *digesterImpl) sendDigest(ctx context.Context, u *model.User) error {
	// TODO: Optimize these queries
	events, err := d.EventStore.GetEventsByUser(ctx, u, &model.Pagination{})
	if err != nil {
		return err
	}

	threads, err := d.ThreadStore.GetThreadsByUser(ctx, u, &model.Pagination{})
	if err != nil {
		return err
	}

	var (
		// Convert the events into Digestables and filter out read items
		digestables []model.Digestable
		// Save the upcoming events to a slice at the same time
		upcoming []*model.Event
		//
		toMarkEvents  []*model.Event
		toMarkThreads []*model.Thread
	)

	for i := range events {
		if !model.IsRead(events[i], u.Key) {
			digestables = append(digestables, events[i])
			toMarkEvents = append(toMarkEvents, events[i])
		}

		if events[i].IsUpcoming() {
			upcoming = append(upcoming, events[i])
		}
	}

	for i := range threads {
		if !model.IsRead(threads[i], u.Key) {
			digestables = append(digestables, threads[i])
			toMarkThreads = append(toMarkThreads, threads[i])
		}
	}

	digestList, err := generateDigestList(ctx, d.MessageStore, digestables, u)
	if err != nil {
		return err
	}

	if len(digestList) > 0 || len(upcoming) > 0 {
		if err := d.Mail.SendDigest(d.Magic, digestList, upcoming, u); err != nil {
			return err
		}

		if err := markDigestedMessagesAsRead(ctx, d.MessageStore, digestList, u); err != nil {
			return err
		}

		// The following two calls are bad because they're exposed to a race condition
		// so all this will have to change if there are ever a decent amount of real users
		if err := markThreadsAsRead(ctx, d.ThreadStore, toMarkThreads, u); err != nil {
			return err
		}

		if err := markEventsAsRead(ctx, d.EventStore, toMarkEvents, u); err != nil {
			return err
		}
	}

	log.Printf("digest.sendDigest: processed digest of %d items for user %q",
		len(digestList)+len(upcoming), u.Email)

	return nil
}

func generateDigestList(
	ctx context.Context,
	ms model.MessageStore,
	digestables []model.Digestable,
	u *model.User,
) ([]model.DigestItem, error) {
	var digest []model.DigestItem

	for i := range digestables {
		item, err := generateDigestItem(ctx, ms, digestables[i], u)
		if err != nil {
			switch err.(type) {
			case *DigestError:
				continue
			default:
				return digest, err
			}
		}

		digest = append(digest, item)
	}

	return digest, nil
}

func generateDigestItem(
	ctx context.Context,
	ms model.MessageStore,
	d model.Digestable,
	u *model.User,
) (model.DigestItem, error) {
	messages, err := ms.GetMessagesByKey(ctx, d.GetKey())
	if err != nil {
		return model.DigestItem{}, err
	}

	var unread []*model.Message

	for j := range messages {
		if !model.IsRead(messages[j], u.Key) {
			unread = append(unread, messages[j])
		}
	}

	if len(unread) > 0 {
		return model.DigestItem{
			ParentID: d.GetKey(),
			Name:     d.GetName(),
			Messages: unread,
		}, nil
	}

	return model.DigestItem{}, &DigestError{}
}

func markDigestedMessagesAsRead(
	ctx context.Context,
	ms model.MessageStore,
	digestList []model.DigestItem,
	user *model.User,
) error {
	var messages []*model.Message

	for i := range digestList {
		for j := range digestList[i].Messages {
			model.MarkAsRead(digestList[i].Messages[j], user.Key)
			messages = append(messages, digestList[i].Messages[j])
		}
	}

	err := ms.CommitMulti(ctx, messages)
	if err != nil {
		return err
	}

	return nil
}

func markThreadsAsRead(
	ctx context.Context,
	ts model.ThreadStore,
	threads []*model.Thread,
	user *model.User,
) error {
	for i := range threads {
		model.MarkAsRead(threads[i], user.Key)
	}

	err := ts.CommitMulti(ctx, threads)
	if err != nil {
		return err
	}

	return nil
}

func markEventsAsRead(
	ctx context.Context,
	es model.EventStore,
	events []*model.Event,
	user *model.User,
) error {
	for i := range events {
		model.MarkAsRead(events[i], user.Key)
	}

	err := es.CommitMulti(ctx, events)
	if err != nil {
		return err
	}

	return nil
}

type DigestError struct{}

func (e *DigestError) Error() string {
	return "Nothing to digest"
}
