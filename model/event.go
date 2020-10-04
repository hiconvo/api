package model

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	ics "github.com/arran4/golang-ical"
	"github.com/gosimple/slug"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/random"
)

const MaxEventMembers = 300

type Event struct {
	Key             *datastore.Key   `json:"-"        datastore:"__key__"`
	Token           string           `json:"-"`
	ID              string           `json:"id"       datastore:"-"`
	OwnerKey        *datastore.Key   `json:"-"`
	Owner           *UserPartial     `json:"owner"    datastore:"-"`
	HostKeys        []*datastore.Key `json:"-"`
	HostPartials    []*UserPartial   `json:"hosts"    datastore:"-"`
	UserKeys        []*datastore.Key `json:"-"`
	UserPartials    []*UserPartial   `json:"users"    datastore:"-"`
	Users           []*User          `json:"-"        datastore:"-"`
	RSVPKeys        []*datastore.Key `json:"-"`
	RSVPs           []*UserPartial   `json:"rsvps"    datastore:"-"`
	PlaceID         string           `json:"placeId"  datastore:",noindex"`
	Address         string           `json:"address"  datastore:",noindex"`
	Lat             float64          `json:"lat"      datastore:",noindex"`
	Lng             float64          `json:"lng"      datastore:",noindex"`
	Name            string           `json:"name"     datastore:",noindex"`
	Description     string           `json:"description"  datastore:",noindex"`
	Timestamp       time.Time        `json:"timestamp"    datastore:",noindex"`
	UTCOffset       int              `json:"-"        datastore:",noindex"`
	UserReads       []*UserPartial   `json:"reads"    datastore:"-"`
	Reads           []*Read          `json:"-"        datastore:",noindex"`
	CreatedAt       time.Time        `json:"createdAt"`
	GuestsCanInvite bool             `json:"guestsCanInvite"`
}

type EventStore interface {
	GetEventByID(ctx context.Context, id string) (*Event, error)
	GetUnhydratedEventsByUser(ctx context.Context, u *User, p *Pagination) ([]*Event, error)
	GetEventsByUser(ctx context.Context, u *User, p *Pagination) ([]*Event, error)
	Commit(ctx context.Context, e *Event) error
	CommitMulti(ctx context.Context, events []*Event) error
	CommitWithTransaction(tx db.Transaction, e *Event) (*datastore.PendingKey, error)
	Delete(ctx context.Context, e *Event) error
}

func NewEvent(
	name, description, placeID, address string,
	lat, lng float64,
	timestamp time.Time,
	utcOffset int,
	owner *User,
	hosts []*User,
	users []*User,
	guestsCanInvite bool,
) (*Event, error) {
	if timestamp.Before(time.Now().Add(-time.Minute)) {
		return nil, errors.E(
			errors.Op("model.NewEvent"),
			errors.Str("event in past"),
			map[string]string{"time": "Your event must be in the future"},
			http.StatusBadRequest)
	}

	// Get all of the users' keys, remove duplicates, and check whether
	// the owner was included in the users slice
	userKeys := make([]*datastore.Key, 0)
	seenUsers := make(map[string]*User)
	hasOwner := false
	for _, u := range users {
		if _, alreadySeen := seenUsers[u.ID]; alreadySeen {
			continue
		}
		seenUsers[u.ID] = u
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

	// Get all of the hosts' keys, remove duplicates, and check whether
	// hosts were included in the users slice.
	hostKeys := make([]*datastore.Key, 0)
	seenHosts := make(map[string]struct{})
	for _, u := range hosts {
		if _, alreadySeenHost := seenHosts[u.ID]; alreadySeenHost {
			continue
		}

		seenHosts[u.ID] = struct{}{}
		hostKeys = append(hostKeys, u.Key)

		if _, alreadySeenUser := seenUsers[u.ID]; alreadySeenUser {
			continue
		}

		seenUsers[u.ID] = u
		userKeys = append(userKeys, u.Key)
	}

	allUsers := make([]*User, 0)
	for _, u := range seenUsers {
		allUsers = append(allUsers, u)
	}

	if len(allUsers) > MaxEventMembers {
		return nil, errors.E(errors.Op("model.NewEvent"), errors.Str("max members exceeded"))
	}

	return &Event{
		Key:             datastore.IncompleteKey("Event", nil),
		Token:           random.Token(),
		OwnerKey:        owner.Key,
		Owner:           MapUserToUserPartial(owner),
		HostKeys:        hostKeys,
		HostPartials:    MapUsersToUserPartials(hosts),
		UserKeys:        userKeys,
		UserPartials:    MapUsersToUserPartials(users),
		Users:           allUsers,
		Name:            name,
		PlaceID:         placeID,
		Address:         address,
		Lat:             lat,
		Lng:             lng,
		Timestamp:       timestamp,
		UTCOffset:       utcOffset,
		Description:     description,
		GuestsCanInvite: guestsCanInvite,
	}, nil
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
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if mismatch.FieldName != "GuestsCanInvite" {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (e *Event) GetReads() []*Read {
	return e.Reads
}

func (e *Event) SetReads(newReads []*Read) {
	e.Reads = newReads
}

func (e *Event) GetKey() *datastore.Key {
	return e.Key
}

func (e *Event) GetName() string {
	return e.Name
}

func (e *Event) GetFormatedTime() string {
	loc := time.FixedZone("Given", e.UTCOffset)
	return e.Timestamp.In(loc).Format("Monday, January 2 @ 3:04 PM")
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

func (e *Event) HostIs(u *User) bool {
	for _, k := range e.HostKeys {
		if k.Equal(u.Key) {
			return true
		}
	}

	return false
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
	op := errors.Op("event.AddUser")
	// Cannot add owner or duplicate.
	if e.OwnerIs(u) || e.HasUser(u) {
		return errors.E(op,
			map[string]string{"message": "This user is already invited to this event"},
			errors.Str("already has user"),
			http.StatusBadRequest)
	}

	if len(e.UserKeys) >= 300 {
		return errors.E(op,
			map[string]string{"message": "This event has the maximum number of guests"},
			errors.Str("user count limit"),
			http.StatusBadRequest)
	}

	e.UserKeys = append(e.UserKeys, u.Key)
	e.UserPartials = append(e.UserPartials, MapUserToUserPartial(u))

	return nil
}

func (e *Event) RemoveUser(u *User) error {
	op := errors.Op("model.RemoveUser")

	if !e.HasUser(u) {
		return errors.E(op, errors.Str("event does not have user"), http.StatusBadRequest)
	}

	if e.OwnerIs(u) {
		return errors.E(op,
			map[string]string{"message": "You cannot remove yourself from your own event"},
			errors.Str("user cannot remove herself"),
			http.StatusBadRequest)
	}

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

	return nil
}

// AddRSVP RSVPs a user for the event.
func (e *Event) AddRSVP(u *User) error {
	op := errors.Op("event.AddRSVP")

	if !e.HasUser(u) {
		return errors.E(op, errors.Str("user not in event"), http.StatusUnauthorized)
	}

	// Cannot add owner or duplicate.
	if e.OwnerIs(u) || e.HasRSVP(u) {
		return errors.E(op,
			errors.Str("already has rsvp"),
			map[string]string{"message": "You have already RSVP'd"},
			http.StatusBadRequest)
	}

	e.RSVPKeys = append(e.RSVPKeys, u.Key)
	e.RSVPs = append(e.RSVPs, MapUserToUserPartial(u))
	e.SetReads([]*Read{})

	return nil
}

func (e *Event) RemoveRSVP(u *User) error {
	op := errors.Op("event.RemoveRSVP")

	if !e.HasUser(u) {
		return errors.E(op, errors.Str("no permission"), http.StatusNotFound)
	}

	if e.OwnerIs(u) {
		return errors.E(op,
			map[string]string{"message": "You cannot remove yourself from your own event"},
			errors.Str("user cannot remove herself"),
			http.StatusBadRequest)
	}

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

	return nil
}

func (e *Event) GetEmail() string {
	slugified := slug.Make(e.Name)
	if len(slugified) > 20 {
		slugified = slugified[:20]
	}
	return fmt.Sprintf("%s-%d@mail.convo.events", slugified, e.Key.ID)
}

func (e *Event) IsInFuture() bool {
	return e.Timestamp.After(time.Now())
}

func (e *Event) IsUpcoming() bool {
	start, err := time.ParseDuration("6h")
	if err != nil {
		return false
	}
	end, err := time.ParseDuration("30h")
	if err != nil {
		return false
	}

	windowStart := time.Now().Add(start)
	windowEnd := time.Now().Add(end)

	return e.Timestamp.After(windowStart) && e.Timestamp.Before(windowEnd)
}

func (e *Event) GetICS() string {
	cal := ics.NewCalendar()

	ev := cal.AddEvent(e.ID)

	ev.SetCreatedTime(e.CreatedAt)
	ev.SetStartAt(e.Timestamp)
	ev.SetEndAt(e.Timestamp.Add(time.Hour))
	ev.SetSummary(e.Name)
	ev.SetLocation(e.Address)
	ev.SetDescription(e.Description)
	ev.SetOrganizer(e.GetEmail(), ics.WithCN(e.Owner.FullName))

	return cal.Serialize()
}

func (e *Event) RollToken() {
	e.Token = random.Token()
}

func (e *Event) GetInviteMagicLink(m magic.Client) string {
	return m.NewLink(e.Key, e.Token, "invite")
}

func (e *Event) VerifyInviteMagicLink(m magic.Client, ts, sig string) error {
	return m.Verify(e.ID, ts, e.Token, sig)
}

func (e *Event) GetRSVPMagicLink(m magic.Client, u *User) string {
	return m.NewLink(
		u.Key,
		e.Token+strconv.FormatBool(!e.IsInFuture()),
		fmt.Sprintf("rsvp/%s", e.Key.Encode()))
}

func (e *Event) VerifyRSVPMagicLink(m magic.Client, userID, ts, sig string) error {
	return m.Verify(userID, ts, e.Token+strconv.FormatBool(!e.IsInFuture()), sig)
}

func (e *Event) SendInvitesAsync(ctx context.Context, q queue.Client) error {
	return q.PutEmail(ctx, queue.EmailPayload{
		Type:   queue.Event,
		Action: queue.SendInvites,
		IDs:    []string{e.ID},
	})
}

func (e *Event) SendUpdatedInvitesAsync(ctx context.Context, q queue.Client) error {
	return q.PutEmail(ctx, queue.EmailPayload{
		Type:   queue.Event,
		Action: queue.SendUpdatedInvites,
		IDs:    []string{e.ID},
	})
}

func IsHostsDifferent(eventHosts []*datastore.Key, payloadHosts []*User) bool {
	for i := range eventHosts {
		target := eventHosts[i].Encode()
		seen := false
		for j := range payloadHosts {
			if payloadHosts[j].ID == target {
				seen = true
				break
			}
		}

		if !seen {
			return true
		}
	}

	return false
}
