package models

import (
	"context"
	"errors"

	"cloud.google.com/go/datastore"
	"github.com/hiconvo/api/db"
)

type Event struct {
	Key          *datastore.Key   `json:"-"        datastore:"__key__"`
	ID           string           `json:"id"       datastore:"-"`
	OwnerKey     *datastore.Key   `json:"-"`
	Owner        *UserPartial     `json:"owner"    datastore:"-"`
	UserKeys     []*datastore.Key `json:"-"        datastore:",noindex"`
	UserPartials []*UserPartial   `json:"users"    datastore:"-"`
	RSVPKeys     []*datastore.Key `json:"-"        datastore:",noindex"`
	RSVPs        []*UserPartial   `json:"rsvps"    datastore:"-"`
	LocationKey  string           `json:"-"`
	Location     string           `json:"location"`
	Name         string           `json:"name"     datastore:",noindex"`
}

func (e *Event) LoadKey(k *datastore.Key) error {
	e.Key = k

	// Add URL safe key
	e.ID = k.Encode()

	return nil
}

func (e *Event) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(e)
}

func (e *Event) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(e, ps); err != nil {
		return err
	}

	return nil
}

func (e *Event) Commit(ctx context.Context) error {
	key, kErr := db.Client.Put(ctx, e.Key, e)
	if kErr != nil {
		return kErr
	}
	e.ID = key.Encode()
	e.Key = key
	return nil
}

func (e *Event) Delete(ctx context.Context) error {
	if err := db.Client.Delete(ctx, e.Key); err != nil {
		return err
	}
	return nil
}

func (e *Event) HasUser(u *User) bool {
	for _, k := range e.UserKeys {
		if k.Equal(u.Key) {
			return true
		}
	}

	return false
}

func (e *Event) OwnerIs(u *User) bool {
	return e.OwnerKey.Equal(u.Key)
}

func (e *Event) HasRSVP(u *User) bool {
	for _, k := range e.RSVPKeys {
		if k.Equal(u.Key) {
			return true
		}
	}

	return false
}

// AddUser adds a user to the event.
func (e *Event) AddUser(u *User) error {
	// Cannot add owner or duplicate.
	if e.OwnerIs(u) || e.HasUser(u) {
		return errors.New("This user is already invited to this event")
	}

	if len(e.UserKeys) >= 300 {
		return errors.New("This event has the maximum number of guests")
	}

	e.UserKeys = append(e.UserKeys, u.Key)
	e.UserPartials = append(e.UserPartials, MapUserToUserPartial(u))

	return nil
}

func (e *Event) RemoveUser(u *User) {
	// Remove from keys.
	for i, k := range e.UserKeys {
		if k.Equal(u.Key) {
			e.UserKeys[i] = e.UserKeys[len(e.UserKeys)-1]
			e.UserKeys = e.UserKeys[:len(e.UserKeys)-1]
			break
		}
	}
	// Remove from partials.
	for i, c := range e.UserPartials {
		if c.ID == u.ID {
			e.UserPartials[i] = e.UserPartials[len(e.UserPartials)-1]
			e.UserPartials = e.UserPartials[:len(e.UserPartials)-1]
			break
		}
	}
}

// AddRSVP RSVPs a user for the event.
func (e *Event) AddRSVP(u *User) error {
	// Cannot add owner or duplicate.
	if e.OwnerIs(u) || e.HasRSVP(u) {
		return errors.New("This user has already RSVP'd")
	}

	e.RSVPKeys = append(e.RSVPKeys, u.Key)
	e.RSVPs = append(e.RSVPs, MapUserToUserPartial(u))

	return nil
}

func (e *Event) RemoveRSVP(u *User) {
	// Remove from keys.
	for i, k := range e.RSVPKeys {
		if k.Equal(u.Key) {
			e.RSVPKeys[i] = e.RSVPKeys[len(e.RSVPKeys)-1]
			e.RSVPKeys = e.RSVPKeys[:len(e.RSVPKeys)-1]
			break
		}
	}

	// Remove from partials.
	for i, c := range e.RSVPs {
		if c.ID == u.ID {
			e.RSVPs[i] = e.RSVPs[len(e.RSVPs)-1]
			e.RSVPs = e.RSVPs[:len(e.RSVPs)-1]
			break
		}
	}
}

func NewEvent(name, locationKey, location string, owner *User, users []*User) (Event, error) {
	// Get all of the users' keys, remove duplicates, and check whether
	// the owner was included in the users slice
	userKeys := make([]*datastore.Key, 0)
	seen := make(map[string]struct{})
	hasOwner := false
	for _, u := range users {
		if _, alreadySeen := seen[u.ID]; alreadySeen {
			continue
		}
		seen[u.ID] = struct{}{}
		if u.Key.Equal(owner.Key) {
			hasOwner = true
		}
		userKeys = append(userKeys, u.Key)
	}

	// Add the owner to the users if not already present
	if !hasOwner {
		userKeys = append(userKeys, owner.Key)
		users = append(users, owner)
	}

	// If a subject wasn't given, create one that is a list of the participants'
	// names.
	//
	// TODO: Change this when adding/removing users from threads.
	if name == "" {
		name += "Event for "
		for i, u := range users {
			if i == len(users)-1 {
				name += "and " + u.FirstName
			} else {
				name += u.FirstName + ", "
			}
		}
	}

	return Event{
		Key:          datastore.IncompleteKey("Event", nil),
		OwnerKey:     owner.Key,
		Owner:        MapUserToUserPartial(owner),
		UserKeys:     userKeys,
		UserPartials: MapUsersToUserPartials(users),
		Name:         name,
		LocationKey:  locationKey,
		Location:     location,
	}, nil
}

func GetEventByID(ctx context.Context, id string) (Event, error) {
	var e Event

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return e, err
	}

	return handleGetEvent(ctx, key, e)
}

func GetEventsByUser(ctx context.Context, u *User) ([]*Event, error) {
	// Get all of the keys of the events that the user owns.
	q := datastore.NewQuery("Event").Filter("OwnerKey =", u.Key).KeysOnly()
	ownedEventKeys, err := db.Client.GetAll(ctx, q, nil)
	if err != nil {
		var eventPtrs []*Event
		return eventPtrs, err
	}

	// Get all of the events to which the user is invited, plus all of
	// events that the user owns.
	events := make([]Event, len(u.Events)+len(ownedEventKeys))
	keys := append(ownedEventKeys, u.Events...)
	if err := db.Client.GetMulti(ctx, keys, events); err != nil {
		var eventPtrs []*Event
		return eventPtrs, err
	}

	// Now that we have the events, we need to get the users. We keep track of
	// where the users of one event start and another begin by incrementing
	// an index.
	var uKeys []*datastore.Key
	var idxs []int
	for _, e := range events {
		uKeys = append(uKeys, e.UserKeys...)
		idxs = append(idxs, len(e.UserKeys))
	}

	// We get all of the users in one go.
	us := make([]User, len(uKeys))
	if err := db.Client.GetMulti(ctx, uKeys, us); err != nil {
		var eventPtrs []*Event
		return eventPtrs, err
	}

	// In order to satisfy MapUsersToUserPartials() and other functions, we map
	// user objects to pointers to them.
	userPtrs := make([]*User, len(us))
	for i := range us {
		userPtrs[i] = &us[i]
	}

	// We add the just retrieved user objects to their corresponding events by
	// iterating through all of the events and assigning their users according
	// to the index which we created above.
	//
	// We also create a new slice of pointers to events which we'll finally
	// return.
	start := 0
	eventPtrs := make([]*Event, len(events))
	for i := range events {
		eventUsers := userPtrs[start : start+idxs[i]]

		eventRSVPs := make([]*User, 0)
		var owner *User
		for j := range eventUsers {

			if events[i].HasRSVP(eventUsers[j]) {
				eventRSVPs = append(eventRSVPs, eventUsers[j])
			}

			if events[i].OwnerIs(eventUsers[j]) {
				owner = eventUsers[j]
			}
		}

		events[i].Owner = MapUserToUserPartial(owner)
		events[i].UserPartials = MapUsersToUserPartials(eventUsers)
		events[i].RSVPs = MapUsersToUserPartials(eventRSVPs)

		start += idxs[i]
		eventPtrs[i] = &events[i]
	}

	return eventPtrs, nil
}

func handleGetEvent(ctx context.Context, key *datastore.Key, e Event) (Event, error) {
	if err := db.Client.Get(ctx, key, &e); err != nil {
		return e, err
	}

	users := make([]User, len(e.UserKeys))
	if err := db.Client.GetMulti(ctx, e.UserKeys, users); err != nil {
		return e, err
	}

	userPointers := make([]*User, len(users))
	rsvpPointers := make([]*User, 0)
	var owner User
	for i := range users {
		userPointers[i] = &users[i]

		if e.OwnerKey.Equal(users[i].Key) {
			owner = users[i]
		}

		if e.HasRSVP(userPointers[i]) {
			rsvpPointers = append(rsvpPointers, userPointers[i])
		}
	}

	e.UserPartials = MapUsersToUserPartials(userPointers)
	e.Owner = MapUserToUserPartial(&owner)
	e.RSVPs = MapUsersToUserPartials(rsvpPointers)

	return e, nil
}
