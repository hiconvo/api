package models

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gosimple/slug"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/queue"
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

	// Since an email is sent when a thread is created,
	// it is initialized as having been read by all members.
	reads := make([]*Read, len(userKeys))
	for i := range users {
		reads[i] = NewRead(userKeys[i])
	}

	return Thread{
		Key:          datastore.IncompleteKey("Thread", nil),
		OwnerKey:     owner.Key,
		Owner:        MapUserToUserPartial(owner),
		UserKeys:     userKeys,
		Users:        users,
		UserPartials: MapUsersToUserPartials(users),
		Subject:      subject,
		Reads:        reads,
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

func (t *Thread) Commit(ctx context.Context) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}

	key, err := db.DefaultClient.Put(ctx, t.Key, t)
	if err != nil {
		return errors.E(errors.Op("thread.Commit"), err)
	}

	t.ID = key.Encode()
	t.Key = key

	return nil
}

func (t *Thread) CommitWithTransaction(tx db.Transaction) (*datastore.PendingKey, error) {
	return tx.Put(t.Key, t)
}

func (t *Thread) Delete(ctx context.Context) error {
	if err := db.DefaultClient.Delete(ctx, t.Key); err != nil {
		return err
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

	// Cannot add owner or duplicate.
	if t.OwnerIs(u) || t.HasUser(u) {
		return errors.E(op,
			map[string]string{"message": "This user is already a member of this Convo"},
			http.StatusBadRequest,
			errors.Str("AlreadyHasUser"))
	}

	if len(t.UserKeys) >= 11 {
		return errors.E(op,
			map[string]string{"message": "This Convo has the maximum number of users"},
			http.StatusBadRequest,
			errors.Str("UserCountLimit"))
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

func (t *Thread) Send(ctx context.Context) error {
	messages, err := GetMessagesByThread(ctx, t)
	if err != nil {
		return err
	}

	return sendThread(t, messages)
}

func (t *Thread) SendAsync(ctx context.Context) error {
	return queue.PutEmail(ctx, queue.EmailPayload{
		Type:   queue.Thread,
		Action: queue.SendThread,
		IDs:    []string{t.ID},
	})
}

func (t *Thread) IncRespCount() error {
	t.ResponseCount++
	return nil
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

func GetUnhydratedThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error) {
	var threads []*Thread

	q := datastore.NewQuery("Thread").
		Filter("UserKeys =", u.Key).
		Order("-CreatedAt").
		Offset(p.Offset()).
		Limit(p.Limit())

	_, err := db.DefaultClient.GetAll(ctx, q, &threads)
	if err != nil {
		return threads, err
	}

	return threads, nil
}

func GetThreadsByUser(ctx context.Context, u *User, p *Pagination) ([]*Thread, error) {
	// Get all of the threads of which the user is a member
	threads, err := GetUnhydratedThreadsByUser(ctx, u, p)
	if err != nil {
		return threads, err
	}

	// Now that we have the threads, we need to get the users. We keep track of
	// where the users of one thread start and another begin by incrementing
	// an index.
	var userKeys []*datastore.Key
	var idxs []int
	for _, t := range threads {
		userKeys = append(userKeys, t.UserKeys...)
		idxs = append(idxs, len(t.UserKeys))
	}

	// We get all of the users in one go.
	userPtrs := make([]*User, len(userKeys))
	if err := db.DefaultClient.GetMulti(ctx, userKeys, userPtrs); err != nil {
		return threads, err
	}

	// We add the just retrieved user objects to their corresponding threads by
	// iterating through all of the threads and assigning their users according
	// to the index which we created above.
	//
	// We also create a new slice of pointers to threads which we'll finally
	// return.
	start := 0
	threadPtrs := make([]*Thread, len(threads))
	for i := range threads {
		threadUsers := userPtrs[start : start+idxs[i]]

		var owner *User
		for j := range threadUsers {
			if threads[i].OwnerKey.Equal(threadUsers[j].Key) {
				owner = threadUsers[j]
				break
			}
		}

		threads[i].Users = threadUsers
		threads[i].Owner = MapUserToUserPartial(owner)
		threads[i].UserPartials = MapUsersToUserPartials(threadUsers)
		threads[i].UserReads = MapReadsToUserPartials(threads[i], threadUsers)

		start += idxs[i]
		threadPtrs[i] = threads[i]
	}

	return threadPtrs, nil
}

func handleGetThread(ctx context.Context, key *datastore.Key, t Thread) (Thread, error) {
	if err := db.DefaultClient.Get(ctx, key, &t); err != nil {
		return t, err
	}

	users := make([]User, len(t.UserKeys))
	if err := db.DefaultClient.GetMulti(ctx, t.UserKeys, users); err != nil {
		return t, err
	}

	userPointers := make([]*User, len(users))
	var owner User
	for i := range users {
		userPointers[i] = &users[i]

		if t.OwnerKey.Equal(users[i].Key) {
			owner = users[i]
		}
	}

	t.Users = userPointers
	t.UserPartials = MapUsersToUserPartials(userPointers)
	t.UserReads = MapReadsToUserPartials(&t, userPointers)
	t.Owner = MapUserToUserPartial(&owner)

	return t, nil
}
