package model

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gosimple/slug"

	"github.com/hiconvo/api/clients/db"
	og "github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/storage"
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
	Preview       *Preview         `json:"preview"  datastore:",noindex"`
	UserReads     []*UserPartial   `json:"reads"    datastore:"-"`
	Reads         []*Read          `json:"-"        datastore:",noindex"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	ResponseCount int              `json:"responseCount" datastore:",noindex"`
}

type ThreadStore interface {
	GetThreadByID(ctx context.Context, id string) (*Thread, error)
	GetThreadByInt64ID(ctx context.Context, id int64) (*Thread, error)
	GetUnhydratedThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error)
	GetThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error)
	Commit(ctx context.Context, t *Thread) error
	CommitMulti(ctx context.Context, threads []*Thread) error
	CommitWithTransaction(tx db.Transaction, t *Thread) (*datastore.PendingKey, error)
	Delete(ctx context.Context, t *Thread) error
	AllocateKey(ctx context.Context) (*datastore.Key, error)
}

type NewThreadInput struct {
	Owner   *User
	Users   []*User
	Subject string
	Body    string
	Blob    string
}

func NewThread(
	ctx context.Context,
	tstore ThreadStore,
	sclient *storage.Client,
	ogclient og.Client,
	input *NewThreadInput,
) (*Thread, error) {
	op := errors.Op("NewThread")

	if len(input.Users) > 11 {
		return nil, errors.E(op, http.StatusBadRequest, map[string]string{
			"message": "Convos have a maximum of 11 members",
		})
	}

	// Get all of the users' keys, remove duplicates, and check whether
	// the owner was included in the users slice
	userKeys := make([]*datastore.Key, 0)
	seen := make(map[string]struct{})
	hasOwner := false
	for _, u := range input.Users {
		if _, alreadySeen := seen[u.ID]; alreadySeen {
			continue
		}
		seen[u.ID] = struct{}{}
		if u.Key.Equal(input.Owner.Key) {
			hasOwner = true
		}
		userKeys = append(userKeys, u.Key)
	}

	// Add the owner to the users if not already present
	if !hasOwner {
		userKeys = append(userKeys, input.Owner.Key)
		input.Users = append(input.Users, input.Owner)
	}

	key, err := tstore.AllocateKey(ctx)
	if err != nil {
		return nil, errors.E(op, err)
	}

	link, photoURL, err := handleLinkAndPhoto(
		ctx, sclient, ogclient, key, input.Body, input.Blob)
	if err != nil {
		return nil, errors.E(op, err)
	}

	var photos []string
	if photoURL != "" {
		photos = []string{photoURL}
	}

	if input.Subject == "" && link != nil && link.Title != "" {
		input.Subject = link.Title
	}

	// If a subject wasn't given, create one that is a list of the participants'
	// names.
	if input.Subject == "" {
		if len(input.Users) == 1 {
			input.Subject = input.Owner.FirstName + "'s Private Convo"
		} else {
			for i, u := range input.Users {
				if i == len(input.Users)-1 {
					input.Subject += "and " + u.FirstName
				} else if i == len(input.Users)-2 {
					input.Subject += u.FirstName + " "
				} else {
					input.Subject += u.FirstName + ", "
				}
			}
		}
	}

	return &Thread{
		Key:          key,
		OwnerKey:     input.Owner.Key,
		Owner:        MapUserToUserPartial(input.Owner),
		UserKeys:     userKeys,
		Users:        input.Users,
		UserPartials: MapUsersToUserPartials(input.Users),
		Subject:      input.Subject,
		Preview: &Preview{
			Body:   removeLink(input.Body, link),
			Photos: photos,
			Link:   link,
		},
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
				t.Preview = preview
			} else {
				// ignore it
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

func (t *Thread) IsSendable() bool {
	for i := range t.Users {
		if !t.Users[i].IsRegistered() && !IsRead(t, t.Users[i].Key) {
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
