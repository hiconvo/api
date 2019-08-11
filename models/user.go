package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/olivere/elastic/v7"
	"golang.org/x/crypto/bcrypt"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/search"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/random"
)

type User struct {
	Key              *datastore.Key   `json:"-"        datastore:"__key__"`
	ID               string           `json:"id"       datastore:"-"`
	Email            string           `json:"email"`
	FirstName        string           `json:"firstName"`
	LastName         string           `json:"lastName"`
	FullName         string           `json:"fullName" datastore:"-"`
	Token            string           `json:"token"`
	PasswordDigest   string           `json:"-"        datastore:",noindex"`
	OAuthGoogleID    string           `json:"-"`
	OAuthFacebookID  string           `json:"-"`
	IsPasswordSet    bool             `json:"isPasswordSet"    datastore:"-"`
	IsGoogleLinked   bool             `json:"isGoogleLinked"   datastore:"-"`
	IsFacebookLinked bool             `json:"isFacebookLinked" datastore:"-"`
	IsLocked         bool             `json:"-"`
	Verified         bool             `json:"verified"`
	Threads          []*datastore.Key `json:"-"        datastore:",noindex"`
	Avatar           string           `json:"avatar"`
	ContactKeys      []*datastore.Key `json:"-"        datastore:",noindex"`
	Contacts         []*UserPartial   `json:"-"        datastore:"-"`
}

type UserPartial struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
	Avatar    string `json:"avatar"`
}

func MapUserToUserPartial(u *User) *UserPartial {
	return &UserPartial{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		FullName:  u.FullName,
		Avatar:    u.Avatar,
	}
}

func MapUsersToUserPartials(users []*User) []*UserPartial {
	contacts := make([]*UserPartial, len(users))
	for i, u := range users {
		contacts[i] = MapUserToUserPartial(u)
	}
	return contacts
}

func MapUserPartialToUser(c *UserPartial, users []*User) (*User, error) {
	for _, u := range users {
		if u.ID == c.ID {
			return u, nil
		}
	}

	return &User{}, errors.New("Matching user not in slice")
}

func (u *User) LoadKey(k *datastore.Key) error {
	u.Key = k

	// Add URL safe key
	u.ID = k.Encode()

	return nil
}

func (u *User) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(u)
}

func (u *User) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(u, ps); err != nil {
		return err
	}

	u.DeriveProperties()

	return nil
}

func (u *User) Commit(ctx context.Context) error {
	key, kErr := db.Client.Put(ctx, u.Key, u)
	if kErr != nil {
		return kErr
	}

	u.ID = key.Encode()
	u.Key = key
	u.DeriveProperties()

	// Only index users that are usefully searchable. There's no
	// point in indexing a user whose name is empty.
	if u.FirstName != "" {
		_, upsertErr := search.Client.Update().
			Index("users").
			Id(u.ID).
			DocAsUpsert(true).
			Doc(MapUserToUserPartial(u)).
			Do(ctx)
		if upsertErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to index user in elasticsearch: %s", upsertErr)
		}
	}

	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordDigest), []byte(password))
	return err == nil
}

func (u *User) ChangePassword(password string) bool {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return false
	}

	u.PasswordDigest = string(hash)

	return true
}

func (u *User) DeriveProperties() {
	untrimmed := fmt.Sprintf("%s %s", u.FirstName, u.LastName)
	u.FullName = strings.TrimSpace(untrimmed)

	u.IsPasswordSet = u.PasswordDigest != ""
	u.IsGoogleLinked = u.OAuthGoogleID != ""
	u.IsFacebookLinked = u.OAuthFacebookID != ""
}

func (u *User) SendPasswordResetEmail() error {
	magicLink := magic.NewLink(u.Key, u.PasswordDigest, "reset")
	return sendPasswordResetEmail(u, magicLink)
}

func (u *User) SendVerifyEmail() error {
	magicLink := magic.NewLink(u.Key, strconv.FormatBool(u.Verified), "verify")
	return sendVerifyEmail(u, magicLink)
}

func (u *User) AddThread(t *Thread) error {
	u.Threads = append(u.Threads, t.Key)

	return nil
}

func (u *User) RemoveThread(t *Thread) error {
	for i, k := range u.Threads {
		if k.Equal(t.Key) {
			u.Threads[i] = u.Threads[len(u.Threads)-1]
			u.Threads = u.Threads[:len(u.Threads)-1]
			return nil
		}
	}

	return nil
}

func (u *User) AddContact(c *User) error {
	if u.HasContact(c) {
		return errors.New("You already have this contact")
	}

	if u.Key.Equal(c.Key) {
		return errors.New("You cannot add yourself as a contact")
	}

	if len(u.ContactKeys) >= 50 {
		return errors.New("You can have a maximum of 50 contacts")
	}

	u.ContactKeys = append(u.ContactKeys, c.Key)

	return nil
}

func (u *User) RemoveContact(c *User) error {
	for i, k := range u.ContactKeys {
		if k.Equal(c.Key) {
			u.ContactKeys[i] = u.ContactKeys[len(u.ContactKeys)-1]
			u.ContactKeys = u.ContactKeys[:len(u.ContactKeys)-1]
			return nil
		}
	}

	return nil
}

func (u *User) HasContact(c *User) bool {
	for _, k := range u.ContactKeys {
		if k.Equal(c.Key) {
			return true
		}
	}

	return false
}

func UserSearch(ctx context.Context, query string) ([]UserPartial, error) {
	skip := 0
	take := 10

	contacts := make([]UserPartial, 0)

	esQuery := elastic.NewMultiMatchQuery(query, "fullName", "firstName", "lastName").
		Fuzziness("3").
		MinimumShouldMatch("0")
	result, err := search.Client.Search().
		Index("users").
		Query(esQuery).
		From(skip).Size(take).
		Do(ctx)
	if err != nil {
		return contacts, err
	}

	for _, hit := range result.Hits.Hits {
		var contact UserPartial
		jsonErr := json.Unmarshal(hit.Source, &contact)
		if jsonErr != nil {
			return contacts, jsonErr
		}

		contacts = append(contacts, contact)
	}

	return contacts, nil
}

func NewIncompleteUser(email string) (User, error) {
	user := User{
		Key:      datastore.IncompleteKey("User", nil),
		Email:    email,
		Token:    random.Token(),
		Verified: false,
	}

	return user, nil
}

func NewUserWithPassword(email, firstname, lastname, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return User{}, err
	}

	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           email,
		FirstName:       firstname,
		LastName:        lastname,
		FullName:        "",
		PasswordDigest:  string(hash),
		Token:           random.Token(),
		OAuthGoogleID:   "",
		OAuthFacebookID: "",
		Verified:        false,
	}

	return user, nil
}

func NewUserWithOAuth(email, firstname, lastname, avatar, oauthprovider, oauthtoken string) (User, error) {
	var googleID string
	var facebookID string
	if oauthprovider == "google" {
		googleID = oauthtoken
	} else {
		facebookID = oauthtoken
	}
	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           email,
		FirstName:       firstname,
		LastName:        lastname,
		FullName:        "",
		Avatar:          avatar,
		PasswordDigest:  "",
		Token:           random.Token(),
		OAuthGoogleID:   googleID,
		OAuthFacebookID: facebookID,
		Verified:        false,
	}

	return user, nil
}

func GetUserByID(ctx context.Context, id string) (User, error) {
	u := User{}

	key, keyErr := datastore.DecodeKey(id)
	if keyErr != nil {
		return u, keyErr
	}

	if getErr := db.Client.Get(ctx, key, &u); getErr != nil {
		return u, getErr
	}

	return u, nil
}

func GetUserByEmail(ctx context.Context, email string) (User, bool, error) {
	return getUserByField(ctx, "Email", email)
}

func GetUserByToken(ctx context.Context, token string) (User, bool, error) {
	return getUserByField(ctx, "Token", token)
}

func GetUserByOAuthID(ctx context.Context, oauthtoken, provider string) (User, bool, error) {
	if provider == "google" {
		return getUserByField(ctx, "OAuthGoogleID", oauthtoken)
	}

	return getUserByField(ctx, "OAuthFacebookID", oauthtoken)
}

func GetUsersByThread(ctx context.Context, t *Thread) ([]*User, error) {
	var userKeys []*datastore.Key
	copy(userKeys, t.UserKeys)
	userKeys = append(t.UserKeys, t.OwnerKey)

	users := make([]*User, len(userKeys))
	if err := db.Client.GetMulti(ctx, userKeys, users); err != nil {
		return users, err
	}

	return users, nil
}

func GetOrCreateUserByEmail(ctx context.Context, email string) (User, bool, error) {
	u, found, err := GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, false, err
	} else if found {
		return u, false, nil
	}

	u, err = NewIncompleteUser(email)
	if err != nil {
		return User{}, false, err
	}

	return u, true, nil
}

func getUserByField(ctx context.Context, field, value string) (User, bool, error) {
	var users []*User

	q := datastore.NewQuery("User").Filter(fmt.Sprintf("%s =", field), value)

	keys, getErr := db.Client.GetAll(ctx, q, &users)

	if getErr != nil {
		return User{}, false, getErr
	}

	if len(keys) == 1 {
		user := *users[0]
		return user, true, nil
	}

	if len(keys) > 1 {
		return User{}, false, fmt.Errorf("%s is duplicated", field)
	}

	return User{}, false, nil
}
