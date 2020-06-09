package db

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
)

var _ model.EventStore = (*EventStore)(nil)

type EventStore struct {
	DB db.Client
}

func (s *EventStore) GetEventByID(ctx context.Context, id string) (*model.Event, error) {
	var e model.Event

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return nil, err
	}

	return s.handleGetEvent(ctx, key, e)
}

func (s *EventStore) GetUnhydratedEventsByUser(
	ctx context.Context,
	u *model.User,
	p *model.Pagination,
) ([]*model.Event, error) {
	var events []*model.Event

	q := datastore.NewQuery("Event").
		Filter("UserKeys =", u.Key).
		Order("-CreatedAt").
		Offset(p.Offset()).
		Limit(p.Limit())

	_, err := s.DB.GetAll(ctx, q, &events)
	if err != nil {
		return events, err
	}

	return events, nil
}

func (s *EventStore) GetEventsByUser(
	ctx context.Context,
	u *model.User,
	p *model.Pagination,
) ([]*model.Event, error) {
	// Get all of the events of which the user is a member
	events, err := s.GetUnhydratedEventsByUser(ctx, u, p)
	if err != nil {
		return events, err
	}

	// Now that we have the events, we need to get the users. We keep track of
	// where the users of one event start and another begin by incrementing
	// an index.
	var (
		uKeys []*datastore.Key
		idxs  []int
	)

	for _, e := range events {
		uKeys = append(uKeys, e.UserKeys...)
		idxs = append(idxs, len(e.UserKeys))
	}

	// We get all of the users in one go.
	userPtrs := make([]*model.User, len(uKeys))
	if err := s.DB.GetMulti(ctx, uKeys, userPtrs); err != nil {
		return events, err
	}

	// We add the just retrieved user objects to their corresponding events by
	// iterating through all of the events and assigning their users according
	// to the index which we created above.
	//
	// We also create a new slice of pointers to events which we'll finally
	// return.
	start := 0
	eventPtrs := make([]*model.Event, len(events))

	for i := range events {
		eventUsers := userPtrs[start : start+idxs[i]]

		var (
			eventRSVPs = make([]*model.User, 0)
			eventHosts = make([]*model.User, 0)
			owner      *model.User
		)

		for j := range eventUsers {
			if events[i].HasRSVP(eventUsers[j]) {
				eventRSVPs = append(eventRSVPs, eventUsers[j])
			}

			if events[i].OwnerIs(eventUsers[j]) {
				owner = eventUsers[j]
			}

			if events[i].HostIs(eventUsers[j]) {
				eventHosts = append(eventHosts, eventUsers[j])
			}
		}

		events[i].Owner = model.MapUserToUserPartial(owner)
		events[i].UserPartials = model.MapUsersToUserPartials(eventUsers)
		events[i].Users = eventUsers
		events[i].RSVPs = model.MapUsersToUserPartials(eventRSVPs)
		events[i].HostPartials = model.MapUsersToUserPartials(eventHosts)
		events[i].UserReads = model.MapReadsToUserPartials(events[i], eventUsers)

		start += idxs[i]
		eventPtrs[i] = events[i]
	}

	return eventPtrs, nil
}

func (s *EventStore) Commit(ctx context.Context, e *model.Event) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	key, err := s.DB.Put(ctx, e.Key, e)
	if err != nil {
		return err
	}

	e.ID = key.Encode()
	e.Key = key

	return nil
}

func (s *EventStore) CommitWithTransaction(
	tx db.Transaction,
	e *model.Event,
) (*datastore.PendingKey, error) {
	return tx.Put(e.Key, e)
}

func (s *EventStore) Delete(ctx context.Context, e *model.Event) error {
	if err := s.DB.Delete(ctx, e.Key); err != nil {
		return err
	}

	return nil
}

func (s *EventStore) handleGetEvent(
	ctx context.Context,
	key *datastore.Key,
	e model.Event,
) (*model.Event, error) {
	if err := s.DB.Get(ctx, key, &e); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, errors.E(errors.Op("models.handleGetEvent"), http.StatusNotFound, err)
		}

		return nil, err
	}

	users := make([]model.User, len(e.UserKeys))
	if err := s.DB.GetMulti(ctx, e.UserKeys, users); err != nil {
		return nil, err
	}

	var (
		userPointers = make([]*model.User, len(users))
		rsvpPointers = make([]*model.User, 0)
		hostPointers = make([]*model.User, 0)
		owner        model.User
	)

	for i := range users {
		userPointers[i] = &users[i]

		if e.OwnerKey.Equal(users[i].Key) {
			owner = users[i]
		}

		if e.HasRSVP(userPointers[i]) {
			rsvpPointers = append(rsvpPointers, userPointers[i])
		}

		if e.HostIs(userPointers[i]) {
			hostPointers = append(hostPointers, userPointers[i])
		}
	}

	e.UserPartials = model.MapUsersToUserPartials(userPointers)
	e.Users = userPointers
	e.Owner = model.MapUserToUserPartial(&owner)
	e.HostPartials = model.MapUsersToUserPartials(hostPointers)
	e.RSVPs = model.MapUsersToUserPartials(rsvpPointers)
	e.UserReads = model.MapReadsToUserPartials(&e, userPointers)

	return &e, nil
}
