package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/olivere/elastic/v7"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

var _ model.UserStore = (*UserStore)(nil)

type UserStore struct {
	DB    db.Client
	Notif notification.Client
	S     search.Client
	Queue queue.Client
}

func (s *UserStore) Commit(ctx context.Context, u *model.User) error {
	op := errors.Opf("UserStore.Commit(%q)", u.Email)

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	u.UpdatedAt = time.Now()

	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(strings.TrimSpace(u.FirstName))
		u.LastName = strings.Title(strings.TrimSpace(u.LastName))
	}

	key, err := s.DB.Put(ctx, u.Key, u)
	if err != nil {
		return errors.E(op, err)
	}

	u.ID = key.Encode()
	u.Key = key

	// We have to do this after the user has been saved because we need the
	// ID, which isn't available until the user is in the database
	u.RealtimeToken = s.Notif.GenerateToken(u.ID)
	u.DeriveProperties()

	s.CreateOrUpdateSearchIndex(ctx, u)

	return nil
}

func (s *UserStore) CommitWithTransaction(
	tx db.Transaction,
	u *model.User,
) (*datastore.PendingKey, error) {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	u.UpdatedAt = time.Now()

	return tx.Put(u.Key, u)
}

func (s *UserStore) DeleteWithTransaction(
	ctx context.Context,
	tx db.Transaction,
	u *model.User,
) error {
	if u.IsRegistered() {
		_, err := s.S.Delete().
			Index("users").
			Id(u.ID).
			Do(ctx)
		if err != nil {
			log.Alarm(errors.Errorf("Failed to remove user from elasticsearch: %v", err))
		}
	}

	return tx.Delete(u.Key)
}

func (s *UserStore) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	op := errors.Opf("UserStore.GetUserByID(id=%s)", id)

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return nil, errors.E(op, err, http.StatusNotFound)
	}

	u := new(model.User)
	if err := s.DB.Get(ctx, key, u); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return nil, errors.E(op, http.StatusNotFound, err)
		}

		return nil, errors.E(op, err)
	}

	return u, nil
}

func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, bool, error) {
	op := errors.Opf("UserStore.GetUserByEmail(email=%q)", email)

	email, err := valid.Email(email)
	if err != nil {
		return nil, false, errors.E(op, err)
	}

	u, found, err := s.getUserByField(ctx, "Email", email)
	if !found && err == nil {
		return s.getUserByField(ctx, "Emails", email)
	}

	return u, found, err
}

func (s *UserStore) GetUserByToken(ctx context.Context, token string) (*model.User, bool, error) {
	return s.getUserByField(ctx, "Token", token)
}

func (s *UserStore) GetUserByOAuthID(ctx context.Context, oAuthToken, provider string) (*model.User, bool, error) {
	if provider == "google" {
		return s.getUserByField(ctx, "OAuthGoogleID", oAuthToken)
	}

	return s.getUserByField(ctx, "OAuthFacebookID", oAuthToken)
}

// func (s *UserStore) GetUsersByThread(ctx context.Context, t *model.Thread) ([]*model.User, error) {
// 	var userKeys []*datastore.Key
// 	copy(userKeys, t.UserKeys)
// 	userKeys = append(userKeys, t.OwnerKey)

// 	users := make([]*model.User, len(userKeys))
// 	if err := s.DB.GetMulti(ctx, userKeys, users); err != nil {
// 		return users, err
// 	}

// 	return users, nil
// }

func (s *UserStore) GetUsersByContact(ctx context.Context, u *model.User) ([]*model.User, error) {
	var users []*model.User

	q := datastore.NewQuery("User").Filter("ContactKeys =", u.Key)
	_, err := s.DB.GetAll(ctx, q, &users)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (s *UserStore) GetContactsByUser(ctx context.Context, u *model.User) ([]*model.User, error) {
	contacts := make([]*model.User, len(u.ContactKeys))

	if err := s.DB.GetMulti(ctx, u.ContactKeys, contacts); err != nil {
		return nil, err
	}

	return contacts, nil
}

func (s *UserStore) GetOrCreateUserByEmail(ctx context.Context, email string) (*model.User, bool, error) {
	u, found, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, false, err
	} else if found {
		return u, false, nil
	}

	u, err = model.NewIncompleteUser(email)
	if err != nil {
		return nil, false, err
	}

	err = s.Commit(ctx, u)
	if err != nil {
		return nil, false, err
	}

	model.UserWelcomeMulti(ctx, s.Queue, []*model.User{u})

	return u, true, nil
}

func (s *UserStore) GetOrCreateUsersByEmail(ctx context.Context, emails []string) ([]*model.User, error) {
	var (
		op                = errors.Op("UserStore.GetOrCreateUsersByEmail")
		users             []*model.User
		usersToCommit     []*model.User
		usersToCommitKeys []*datastore.Key
	)

	for i := range emails {
		u, created, err := s.getOrCreateUserByEmailNoCommit(ctx, emails[i])
		if err != nil {
			return nil, errors.E(op, err)
		}

		if created {
			usersToCommit = append(usersToCommit, u)
			usersToCommitKeys = append(usersToCommitKeys, u.Key)
		} else {
			users = append(users, u)
		}
	}

	keys, err := s.DB.PutMulti(ctx, usersToCommitKeys, usersToCommit)
	if err != nil {
		return nil, errors.E(op, err)
	}

	for i := range keys {
		usersToCommit[i].Key = keys[i]
		usersToCommit[i].ID = keys[i].Encode()
	}

	if len(usersToCommit) > 0 {
		model.UserWelcomeMulti(ctx, s.Queue, usersToCommit)
	}

	users = append(users, usersToCommit...)

	return users, nil
}

func (s *UserStore) GetOrCreateUsers(ctx context.Context, users []*model.UserInput) ([]*model.User, error) {
	var (
		op     = errors.Op("UserStore.GetOrCreateUsers")
		seen   = make(map[string]struct{}, len(users)+1)
		emails = make([]string, 0)
		keys   = make([]*datastore.Key, 0)
	)

	for _, u := range users {
		if u.Email != "" {
			email, err := valid.Email(u.Email)
			if err != nil {
				return nil, errors.E(
					op,
					err,
					map[string]string{"user": fmt.Sprintf("%q is not a valid email", u.Email)},
					http.StatusBadRequest)
			}

			if _, ok := seen[email]; !ok {
				seen[email] = struct{}{}

				emails = append(emails, email)
			}

			continue
		}

		if _, ok := seen[u.ID]; ok {
			continue
		}

		seen[u.ID] = struct{}{}

		key, err := datastore.DecodeKey(u.ID)
		if err != nil {
			return nil, errors.E(
				op,
				err,
				map[string]string{"user": "Invalid users"},
				http.StatusBadRequest)
		}

		keys = append(keys, key)
	}

	out := make([]*model.User, len(keys))
	if err := s.DB.GetMulti(ctx, keys, out); err != nil {
		return nil, errors.E(
			op,
			err,
			map[string]string{"users": "Invalid users"},
			http.StatusBadRequest)
	}

	newUsers, err := s.GetOrCreateUsersByEmail(ctx, emails)
	if err != nil {
		return nil, errors.E(op, err)
	}

	out = append(out, newUsers...)

	return out, nil
}

func (s *UserStore) Search(ctx context.Context, query string) ([]*model.UserPartial, error) {
	skip := 0
	take := 10

	contacts := make([]*model.UserPartial, 0)

	esQuery := elastic.NewMultiMatchQuery(query, "fullName", "firstName", "lastName").
		Fuzziness("3").
		MinimumShouldMatch("0")

	result, err := s.S.Search().
		Index("users").
		Query(esQuery).
		From(skip).Size(take).
		Do(ctx)
	if err != nil {
		return contacts, err
	}

	for _, hit := range result.Hits.Hits {
		contact := new(model.UserPartial)

		if err := json.Unmarshal(hit.Source, contact); err != nil {
			return contacts, err
		}

		contacts = append(contacts, contact)
	}

	return contacts, nil
}

func (s *UserStore) IterAll(ctx context.Context) *datastore.Iterator {
	query := datastore.NewQuery("User")
	return s.DB.Run(ctx, query)
}

func (s *UserStore) CreateOrUpdateSearchIndex(ctx context.Context, u *model.User) {
	if u.IsRegistered() {
		_, upsertErr := s.S.Update().
			Index("users").
			Id(u.ID).
			DocAsUpsert(true).
			Doc(model.MapUserToUserPartial(u)).
			Do(ctx)
		if upsertErr != nil {
			log.Printf("Failed to index user in elasticsearch: %v", upsertErr)
		}
	}
}

func (s *UserStore) getUserByField(ctx context.Context, field, value string) (*model.User, bool, error) {
	var (
		op    = errors.Opf("UserStore.getUserByField(field=%q, value=%q)", field, value)
		users []model.User
	)

	q := datastore.NewQuery("User").Filter(fmt.Sprintf("%s =", field), value)

	keys, err := s.DB.GetAll(ctx, q, &users)
	if err != nil {
		return nil, false, errors.E(op, err)
	}

	if len(keys) == 1 {
		user := users[0]

		// Generate the streamer token if not already present
		if user.RealtimeToken == "" && user.ID != "" {
			user.RealtimeToken = s.Notif.GenerateToken(user.ID)
		}

		return &user, true, nil
	}

	if len(keys) > 1 {
		return nil, false, errors.E(op, errors.Errorf("field=%q value=%q is duplicated", field, value))
	}

	return nil, false, nil
}

func (s *UserStore) getOrCreateUserByEmailNoCommit(ctx context.Context, email string) (*model.User, bool, error) {
	u, found, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, false, err
	} else if found {
		return u, false, nil
	}

	u, err = model.NewIncompleteUser(email)
	if err != nil {
		return nil, false, err
	}

	return u, true, nil
}
