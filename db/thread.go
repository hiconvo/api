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

var _ model.ThreadStore = (*ThreadStore)(nil)

type ThreadStore struct {
	DB db.Client
}

func (s *ThreadStore) GetThreadByID(ctx context.Context, id string) (*model.Thread, error) {
	var t = new(model.Thread)

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return nil, err
	}

	return s.handleGetThread(ctx, key, t)
}

func (s *ThreadStore) GetThreadByInt64ID(ctx context.Context, id int64) (*model.Thread, error) {
	var t = new(model.Thread)

	key := datastore.IDKey("Thread", id, nil)

	return s.handleGetThread(ctx, key, t)
}

func (s *ThreadStore) GetUnhydratedThreadsByUser(
	ctx context.Context,
	u *model.User,
	p *model.Pagination,
) ([]*model.Thread, error) {
	var threads []*model.Thread

	q := datastore.NewQuery("Thread").
		Filter("UserKeys =", u.Key).
		Order("-UpdatedAt").
		Offset(p.Offset()).
		Limit(p.Limit())

	_, err := s.DB.GetAll(ctx, q, &threads)
	if err != nil {
		return threads, err
	}

	return threads, nil
}

func (s *ThreadStore) GetThreadsByUser(ctx context.Context, u *model.User, p *model.Pagination) ([]*model.Thread, error) {
	op := errors.Opf("ThreadStore.GetThreadsByUser(%q)", u.Email)
	// Get all of the threads of which the user is a member
	threads, err := s.GetUnhydratedThreadsByUser(ctx, u, p)
	if err != nil {
		return threads, errors.E(op, err)
	}

	// Now that we have the threads, we need to get the users. We keep track of
	// where the users of one thread start and another begin by incrementing
	// an index.
	var (
		userKeys []*datastore.Key
		idxs     []int
	)
	for _, t := range threads {
		userKeys = append(userKeys, t.UserKeys...)
		idxs = append(idxs, len(t.UserKeys))
	}

	// We get all of the users in one go.
	userPtrs := make([]*model.User, len(userKeys))
	if err := s.DB.GetMulti(ctx, userKeys, userPtrs); err != nil {
		return threads, errors.E(op, err)
	}

	// We add the just retrieved user objects to their corresponding threads by
	// iterating through all of the threads and assigning their users according
	// to the index which we created above.
	//
	// We also create a new slice of pointers to threads which we'll finally
	// return.
	start := 0
	threadPtrs := make([]*model.Thread, len(threads))
	for i := range threads {
		threadUsers := userPtrs[start : start+idxs[i]]

		var owner *model.User
		for j := range threadUsers {
			if threads[i].OwnerKey.Equal(threadUsers[j].Key) {
				owner = threadUsers[j]
				break
			}
		}

		threads[i].Users = threadUsers
		threads[i].Owner = model.MapUserToUserPartial(owner)
		threads[i].UserPartials = model.MapUsersToUserPartials(threadUsers)
		threads[i].UserReads = model.MapReadsToUserPartials(threads[i], threadUsers)

		start += idxs[i]
		threadPtrs[i] = threads[i]
	}

	return threadPtrs, nil
}

func (s *ThreadStore) Commit(ctx context.Context, t *model.Thread) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}

	t.UpdatedAt = time.Now()

	key, err := s.DB.Put(ctx, t.Key, t)
	if err != nil {
		return errors.E(errors.Op("thread.Commit"), err)
	}

	t.ID = key.Encode()
	t.Key = key

	return nil
}

func (s *ThreadStore) CommitMulti(ctx context.Context, threads []*model.Thread) error {
	keys := make([]*datastore.Key, len(threads))
	for i := range threads {
		keys[i] = threads[i].Key
	}

	if _, err := s.DB.PutMulti(ctx, keys, threads); err != nil {
		return errors.E(errors.Op("ThreadStore.CommitMulti"), err)
	}

	return nil
}

func (s *ThreadStore) CommitWithTransaction(tx db.Transaction, t *model.Thread) (*datastore.PendingKey, error) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}

	t.UpdatedAt = time.Now()

	return tx.Put(t.Key, t)
}

func (s *ThreadStore) Delete(ctx context.Context, t *model.Thread) error {
	if err := s.DB.Delete(ctx, t.Key); err != nil {
		return err
	}

	return nil
}

func (s *ThreadStore) AllocateKey(ctx context.Context) (*datastore.Key, error) {
	keys, err := s.DB.AllocateIDs(ctx, []*datastore.Key{datastore.IncompleteKey("Thread", nil)})
	if err != nil {
		return nil, err
	}

	return keys[0], nil
}

func (s *ThreadStore) handleGetThread(ctx context.Context, key *datastore.Key, t *model.Thread) (*model.Thread, error) {
	if err := s.DB.Get(ctx, key, t); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return nil, errors.E(errors.Op("ThreadStore.handleGetThread"), err, http.StatusNotFound)
		}

		return t, err
	}

	users := make([]model.User, len(t.UserKeys))
	if err := s.DB.GetMulti(ctx, t.UserKeys, users); err != nil {
		return t, err
	}

	var (
		userPointers = make([]*model.User, len(users))
		owner        model.User
	)

	for i := range users {
		userPointers[i] = &users[i]

		if t.OwnerKey.Equal(users[i].Key) {
			owner = users[i]
		}
	}

	t.Users = userPointers
	t.UserPartials = model.MapUsersToUserPartials(userPointers)
	t.UserReads = model.MapReadsToUserPartials(t, userPointers)
	t.Owner = model.MapUserToUserPartial(&owner)

	return t, nil
}
