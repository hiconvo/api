package model

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gosimple/slug"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/errors"
)

type Thread struct {
	Key           *datastore.Key   `json:"-"        datastore:"__key__"`
	ID            string           `json:"id"       datastore:"-"`
	OwnerKey      *datastore.Key   `json:"-"`
	Owner         *UserPartial     `json:"owner"    datastore:"-"`
	UserKeys      []*datastore.Key `json:"-"`
	Users         []*User          `json:"-"        datastore:"-"`
	UserPartials  []*UserPartial   `json:"users"    datastore:"-"`
	Subject       string           `json:"subject"  datastore:",noindex"`
	Preview       *Message         `json:"preview"  datastore:",noindex"`
	UserReads     []*UserPartial   `json:"reads"    datastore:"-"`
	Reads         []*Read          `json:"-"        datastore:",noindex"`
	CreatedAt     time.Time        `json:"-"`
	ResponseCount int              `json:"responseCount" datastore:",noindex"`
}

type ThreadStore interface {
	GetThreadByID(ctx context.Context, id string) (*Thread, error)
	GetThreadByInt64ID(ctx context.Context, id int64) (*Thread, error)
	GetUnhydratedThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error)
	GetThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error)
	Commit(ctx context.Context, t *Thread) error
	CommitWithTransaction(tx db.Transaction, t *Thread) (*datastore.PendingKey, error)
	Delete(ctx context.Context, t *Thread) error
}

func NewThread(subject string, owner *User, users []*User) (*Thread, error) {
	if len(users) > 11 {
		return nil, errors.E(errors.Op("NewThread"), http.StatusBadRequest, map[string]string{
			"message": "Convos have a maximum of 11 members",
		})
	}

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
	if subject == "" {
		if len(users) == 1 {
			subject = owner.FirstName + "'s Private Convo"
		} else {
			for i, u := range users {
				if i == len(users)-1 {
					subject += "and " + u.FirstName
				} else if i == len(users)-2 {
					subject += u.FirstName + " "
				} else {
					subject += u.FirstName + ", "
				}
			}
		}
	}

	return &Thread{
		Key:          datastore.IncompleteKey("Thread", nil),
		OwnerKey:     owner.Key,
		Owner:        MapUserToUserPartial(owner),
		UserKeys:     userKeys,
		Users:        users,
		UserPartials: MapUsersToUserPartials(users),
		Subject:      subject,
	}, nil
}

func (t *Thread) LoadKey(k *datastore.Key) error {
	t.Key = k

	// Add URL safe key
	t.ID = k.Encode()

	return nil
}

func (t *Thread) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(t)
}

func (t *Thread) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(t, ps); err != nil {
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if mismatch.FieldName != "Preview" {
				return err
			}
		} else {
			return err
		}
	}

	for _, p := range ps {
		if p.Name == "Preview" {
			preview, ok := p.Value.(*Preview)
			if ok {
				t.Preview = &Message{
					Body:      preview.Body,
					User:      preview.Sender,
					Timestamp: preview.Timestamp,
				}
			}
		}
	}

	return nil
}

func (t *Thread) GetReads() []*Read {
	return t.Reads
}

func (t *Thread) SetReads(newReads []*Read) {
	t.Reads = newReads
}

func (t *Thread) GetKey() *datastore.Key {
	return t.Key
}

func (t *Thread) GetName() string {
	return t.Subject
}

func (t *Thread) GetEmail() string {
	slugified := slug.Make(t.Subject)
	if len(slugified) > 20 {
		slugified = slugified[:20]
	}
	return fmt.Sprintf("%s-%d@mail.convo.events", slugified, t.Key.ID)
}

func (t *Thread) HasUser(u *User) bool {
	for _, k := range t.UserKeys {
		if k.Equal(u.Key) {
			return true
		}
	}

	return false
}

func (t *Thread) OwnerIs(u *User) bool {
	return t.OwnerKey.Equal(u.Key)
}

// AddUser adds a user to the thread.
func (t *Thread) AddUser(u *User) error {
	op := errors.Op("thread.AddUser")

	if u.Key.Incomplete() {
		return errors.E(op, errors.Str("incomplete key"))
	}

	// Cannot add owner or duplicate.
	if t.OwnerIs(u) || t.HasUser(u) {
		return errors.E(op,
			map[string]string{"message": "This user is already a member of this Convo"},
			http.StatusBadRequest,
			errors.Str("already has user"))
	}

	if len(t.UserKeys) >= 11 {
		return errors.E(op,
			map[string]string{"message": "This Convo has the maximum number of users"},
			http.StatusBadRequest,
			errors.Str("user count limit"))
	}

	t.UserKeys = append(t.UserKeys, u.Key)
	t.Users = append(t.Users, u)
	t.UserPartials = append(t.UserPartials, MapUserToUserPartial(u))

	return nil
}

func (t *Thread) RemoveUser(u *User) {
	// Remove from keys.
	for i, k := range t.UserKeys {
		if k.Equal(u.Key) {
			t.UserKeys[i] = t.UserKeys[len(t.UserKeys)-1]
			t.UserKeys = t.UserKeys[:len(t.UserKeys)-1]
			break
		}
	}
	// Remove from users.
	for i, c := range t.Users {
		if c.ID == u.ID {
			t.Users[i] = t.Users[len(t.Users)-1]
			t.Users = t.Users[:len(t.Users)-1]
			break
		}
	}
	// Remove from contacts.
	for i, c := range t.UserPartials {
		if c.ID == u.ID {
			t.UserPartials[i] = t.UserPartials[len(t.UserPartials)-1]
			t.UserPartials = t.UserPartials[:len(t.UserPartials)-1]
			break
		}
	}
}

func (t *Thread) IncRespCount() error {
	t.ResponseCount++
	return nil
}
