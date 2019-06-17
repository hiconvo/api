package models

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"github.com/gosimple/slug"
	"github.com/hiconvo/api/db"
)

type Thread struct {
	Key      *datastore.Key   `json:"-"        datastore:"__key__"`
	ID       string           `json:"id"       datastore:"-"`
	OwnerKey *datastore.Key   `json:"-"`
	Owner    *Contact         `json:"owner"    datastore:"-"`
	UserKeys []*datastore.Key `json:"-"        datastore:",noindex"`
	Users    []*User          `json:"-"        datastore:"-"`
	Contacts []*Contact       `json:"users"    datastore:"-"`
	Subject  string           `json:"subject"  datastore:",noindex"`
	Preview  *Preview         `json:"preview"  datastore:",noindex"`
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
		return err
	}

	return nil
}

func (t *Thread) Commit(ctx context.Context) error {
	key, kErr := db.Client.Put(ctx, t.Key, t)
	if kErr != nil {
		return kErr
	}
	t.ID = key.Encode()
	t.Key = key
	return nil
}

func (t *Thread) Delete(ctx context.Context) error {
	if err := db.Client.Delete(ctx, t.Key); err != nil {
		return err
	}
	return nil
}

func (t *Thread) GetEmail() string {
	slugified := slug.Make(t.Subject)
	if len(slugified) > 20 {
		slugified = slugified[:20]
	}
	return fmt.Sprintf("%s-%d@mail.hiconvo.com", slugified, t.Key.ID)
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
func (t *Thread) AddUser(u *User) {
	// Cannot add owner or duplicate.
	if t.OwnerIs(u) || t.HasUser(u) {
		return
	}

	t.UserKeys = append(t.UserKeys, u.Key)
	t.Users = append(t.Users, u)
	t.Contacts = append(t.Contacts, MapUserToContact(u))
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
	for i, c := range t.Contacts {
		if c.ID == u.ID {
			t.Contacts[i] = t.Contacts[len(t.Contacts)-1]
			t.Contacts = t.Contacts[:len(t.Contacts)-1]
			break
		}
	}
}

func (t *Thread) Send(ctx context.Context) error {
	messages, merr := GetMessagesByThread(ctx, t)
	if merr != nil {
		return merr
	}

	users, uerr := GetUsersByThread(ctx, t)
	if uerr != nil {
		return uerr
	}

	return sendThread(t, messages, users)
}

func NewThread(subject string, owner *User, users []*User) (Thread, error) {
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
	if subject == "" {
		subject += owner.FirstName + " "
		for i, u := range users {
			if i == len(users) {
				subject += "and " + u.FullName
			} else {
				subject += u.FullName + ", "
			}
		}
	}

	return Thread{
		Key:      datastore.IncompleteKey("Thread", nil),
		OwnerKey: owner.Key,
		Owner:    MapUserToContact(owner),
		UserKeys: userKeys,
		Users:    users,
		Contacts: MapUsersToContacs(users),
		Subject:  subject,
	}, nil
}

func GetThreadByID(ctx context.Context, id string) (Thread, error) {
	var t Thread

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return t, err
	}

	return handleGetThread(ctx, key, t)
}

func GetThreadByInt64ID(ctx context.Context, id int64) (Thread, error) {
	var t Thread

	key := datastore.IDKey("Thread", id, nil)

	return handleGetThread(ctx, key, t)
}

func GetThreadsByUser(ctx context.Context, u *User) ([]*Thread, error) {
	// Get all of the keys of the threads that the user owns.
	q := datastore.NewQuery("Thread").Filter("OwnerKey =", u.Key).KeysOnly()
	ownedThreadKeys, oErr := db.Client.GetAll(ctx, q, nil)
	if oErr != nil {
		var tptrs []*Thread
		return tptrs, oErr
	}

	// Get all of the threads of which the user is a participant, plus all of
	// threads that the user owns.
	threads := make([]Thread, len(u.Threads)+len(ownedThreadKeys))
	keys := append(ownedThreadKeys, u.Threads...)
	if err := db.Client.GetMulti(ctx, keys, threads); err != nil {
		var tptrs []*Thread
		return tptrs, err
	}

	// Now that we have the threads, we need to get the users - both owners and
	// participants. So that we can do this in one query, we iterate over all
	// of the retrieved threads and append both the owner key and the user
	// keys to a slice. We keep track of where the users of one thread start
	// and another begin by incrementing an index.
	var uKeys []*datastore.Key
	var idxs []int
	for _, t := range threads {
		uKeys = append(uKeys, t.OwnerKey)
		uKeys = append(uKeys, t.UserKeys...)
		idxs = append(idxs, len(t.UserKeys)+1)
	}

	// We get all of the users in one go.
	us := make([]User, len(uKeys))
	if err := db.Client.GetMulti(ctx, uKeys, us); err != nil {
		var tptrs []*Thread
		return tptrs, err
	}

	// In order to satisfy MapUsersToContacs() and other functions, we map
	// user objects to pointers to them.
	uptrs := make([]*User, len(us))
	for i := range us {
		uptrs[i] = &us[i]
	}

	// We add the just retrieved user objects to their corresponding threads by
	// iterating through all of the threads and assigning their owners and
	// users according to the index which we created above.
	//
	// We also create a new slice of pointers to threads which we'll finally
	// return.
	start := 0
	tptrs := make([]*Thread, len(threads))
	for i := range threads {
		threads[i].Users = uptrs[start : start+idxs[i]]
		threads[i].Owner = MapUserToContact(uptrs[start])
		threads[i].Contacts = MapUsersToContacs(uptrs[start : start+idxs[i]])
		start += idxs[i]
		tptrs[i] = &threads[i]
	}

	return tptrs, nil
}

func handleGetThread(ctx context.Context, key *datastore.Key, t Thread) (Thread, error) {
	if err := db.Client.Get(ctx, key, &t); err != nil {
		return t, err
	}

	users := make([]User, len(t.UserKeys))
	if err := db.Client.GetMulti(ctx, t.UserKeys, users); err != nil {
		return t, err
	}

	userPointers := make([]*User, len(users))
	for i := range users {
		userPointers[i] = &users[i]
	}

	t.Users = userPointers
	t.Contacts = MapUsersToContacs(userPointers)
	t.Owner = MapUserToContact(userPointers[len(userPointers)-1])

	return t, nil
}
