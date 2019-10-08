package handlers

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/models"
)

func extractUsers(ctx context.Context, owner models.User, users []interface{}) ([]models.User, []*datastore.Key, []string, error) {
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
			return []models.User{}, []*datastore.Key{}, []string{}, errors.New("Users must be an array of objects")
		}
		// Second, check that the `id` or `email` key points to a string.
		id, idOk := userMap["id"].(string)
		email, emailOk := userMap["email"].(string)
		if !idOk && !emailOk {
			return []models.User{}, []*datastore.Key{}, []string{}, errors.New("User ID or email must be a string")
		}

		// If we recived an email, save it to the emails slice if we haven't
		// seen it before and keep going.
		if emailOk {
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
			return []models.User{}, []*datastore.Key{}, []string{}, errors.New("Invalid users")
		}
		userKeys = append(userKeys, key)
	}
	// Now, get the user objects and save to a new slice of user structs.
	// If this fails, then the input was not valid.
	userStructs := make([]models.User, len(userKeys))
	if err := db.Client.GetMulti(ctx, userKeys, userStructs); err != nil {
		return []models.User{}, []*datastore.Key{}, []string{}, errors.New("Invalid users")
	}

	return userStructs, userKeys, emails, nil
}

// createUserByEmail is a bit of a misnomer since we actually get or create
// the user.
func createUserByEmail(ctx context.Context, email string) (models.User, error) {
	u, created, err := models.GetOrCreateUserByEmail(ctx, email)
	if err != nil {
		return models.User{}, errors.New("Could not create user")
	}
	if created {
		err = u.Commit(ctx)
		if err != nil {
			return models.User{}, errors.New("Could not save user")
		}

		u.Welcome(ctx)
	}

	return u, nil
}

func createUsersByEmail(ctx context.Context, emails []string) ([]models.User, []*datastore.Key, error) {
	userStructs := make([]models.User, len(emails))
	userKeys := make([]*datastore.Key, len(emails))
	// Handle members indicated by email.
	for i := range emails {
		u, err := createUserByEmail(ctx, emails[i])
		if err != nil {
			return []models.User{}, []*datastore.Key{}, err
		}

		userStructs[i] = u
		userKeys[i] = u.Key
	}

	return userStructs, userKeys, nil
}

func isEmail(email string) bool {
	re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return re.MatchString(strings.ToLower(email))
}
