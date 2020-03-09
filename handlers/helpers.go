package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/models"
)

func extractUsers(ctx context.Context, owner models.User, users []interface{}) ([]models.User, []*datastore.Key, []string, error) {
	op := errors.Op("handlers.extractUsers")
	// Make sure users actually exist and remove both duplicate ids and
	// the owner's id from users
	//
	// First, decode ids into keys and put them into a userKeys slice.
	// Also, for event members indicated by email, save emails to
	// a slice for handling later.
	var userKeys []*datastore.Key
	var emails []string
	// Create a map to keep track of seen ids in order to avoid duplicates.
	// Add the `ou` to seen so that she won't be added to the users list.
	seen := make(map[string]struct{}, len(users)+1)
	seenEmails := make(map[string]struct{}, len(users)+1)
	seen[owner.ID] = struct{}{}
	for _, u := range users {
		// Make sure that the payload is of the expected type.
		//
		// First, check that the user key points to an array of maps.
		userMap, ok := u.(map[string]interface{})
		if !ok {
			return []models.User{}, []*datastore.Key{}, []string{}, errors.E(op,
				map[string]string{"users": "Users must be an array of objects"},
				http.StatusBadRequest)
		}
		// Second, check that the `id` or `email` key points to a string.
		id, idOk := userMap["id"].(string)
		email, emailOk := userMap["email"].(string)
		if !idOk && !emailOk {
			return []models.User{}, []*datastore.Key{}, []string{}, errors.E(op,
				map[string]string{"users": "User ID or email must be a string"},
				http.StatusBadRequest)
		}

		// If we recived an email, save it to the emails slice if we haven't
		// seen it before and keep going.
		if emailOk {
			if !isEmail(email) {
				return []models.User{}, []*datastore.Key{}, []string{}, errors.E(op,
					map[string]string{"users": fmt.Sprintf(`"%s" is not a valid email`, email)},
					http.StatusBadRequest)
			}

			if _, seenOk := seenEmails[email]; !seenOk {
				seen[email] = struct{}{}
				emails = append(emails, email)
			}

			continue
		}

		// Make sure we haven't seen this id before.
		if _, seenOk := seen[id]; seenOk {
			continue
		}
		seen[id] = struct{}{}

		// Decode the key and add to the slice.
		key, err := datastore.DecodeKey(id)
		if err != nil {
			return []models.User{}, []*datastore.Key{}, []string{}, errors.E(op,
				map[string]string{"users": "Invalid users"},
				http.StatusBadRequest)
		}
		userKeys = append(userKeys, key)
	}
	// Now, get the user objects and save to a new slice of user structs.
	// If this fails, then the input was not valid.
	userStructs := make([]models.User, len(userKeys))
	if err := db.Client.GetMulti(ctx, userKeys, userStructs); err != nil {
		return []models.User{}, []*datastore.Key{}, []string{}, errors.E(op,
			map[string]string{"users": "Invalid users"},
			http.StatusBadRequest)
	}

	return userStructs, userKeys, emails, nil
}

// createUserByEmail is a bit of a misnomer since we actually get or create
// the user.
func createUserByEmail(ctx context.Context, email string) (models.User, error) {
	op := errors.Op("handlers.createUserByEmail")

	u, created, err := models.GetOrCreateUserByEmail(ctx, email)
	if err != nil {
		return models.User{}, errors.E(op, err)
	}
	if created {
		err = u.Commit(ctx)
		if err != nil {
			return models.User{}, errors.E(op, err)
		}

		u.Welcome(ctx)
	}

	return u, nil
}

func createUsersByEmail(ctx context.Context, emails []string) ([]models.User, []*datastore.Key, error) {
	op := errors.Op("handlers.createUsersByEmail")

	var userStructs []models.User
	var userKeys []*datastore.Key

	var usersToCommit []models.User
	var usersToCommitKeys []*datastore.Key

	for i := range emails {
		u, created, err := models.GetOrCreateUserByEmail(ctx, emails[i])
		if err != nil {
			return userStructs, userKeys, errors.E(op, err)
		}

		if created {
			usersToCommit = append(usersToCommit, u)
			usersToCommitKeys = append(usersToCommitKeys, u.Key)
		} else {
			userStructs = append(userStructs, u)
			userKeys = append(userKeys, u.Key)
		}
	}

	keys, err := db.Client.PutMulti(ctx, usersToCommitKeys, usersToCommit)
	if err != nil {
		return []models.User{}, []*datastore.Key{}, errors.E(op, err)
	}

	for i := range keys {
		usersToCommit[i].Key = keys[i]
		usersToCommit[i].ID = keys[i].Encode()
	}

	models.UserWelcomeMulti(ctx, usersToCommit)

	userStructs = append(userStructs, usersToCommit...)
	userKeys = append(userKeys, usersToCommitKeys...)

	return userStructs, userKeys, nil
}

func extractAndCreateUsers(ctx context.Context, ou models.User, users []interface{}) ([]*models.User, error) {
	userStructs, _, emails, err := extractUsers(ctx, ou, users)
	if err != nil {
		return []*models.User{}, err
	}

	newUsers, _, err := createUsersByEmail(ctx, emails)
	if err != nil {
		return []*models.User{}, err
	}

	userStructs = append(userStructs, newUsers...)

	userPointers := make([]*models.User, len(userStructs))
	for i := range userStructs {
		userPointers[i] = &userStructs[i]
	}

	return userPointers, nil
}

func mapUsersToKeyPointers(users []*models.User) []*datastore.Key {
	keyPointers := make([]*datastore.Key, len(users))
	for i := range keyPointers {
		keyPointers[i] = users[i].Key
	}
	return keyPointers
}

func isEmail(email string) bool {
	re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,14}$`)
	return re.MatchString(strings.ToLower(email))
}

func isHostsDifferent(eventHosts []*datastore.Key, payloadHosts []*models.User) bool {
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

func getPagination(r *http.Request) *models.Pagination {
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("size"))
	return &models.Pagination{Page: pageNum, Size: pageSize}
}
