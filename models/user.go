package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/datastore"
	"golang.org/x/crypto/bcrypt"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/random"
)

type User struct {
	Key             *datastore.Key   `json:"-"        datastore:"__key__"`
	ID              string           `json:"id"       datastore:"-"`
	Email           string           `json:"email"`
	FirstName       string           `json:"firstName"`
	LastName        string           `json:"lastName"`
	FullName        string           `json:"fullName" datastore:"-"`
	PasswordDigest  string           `json:"-"        datastore:",noindex"`
	Token           string           `json:"token"`
	OAuthGoogleID   string           `json:"-"`
	OAuthFacebookID string           `json:"-"`
	Verified        bool             `json:"verified"`
	Threads         []*datastore.Key `json:"-"        datastore:",noindex"`
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

	u.DeriveFullName()

	return nil
}

func (u *User) Commit(ctx context.Context) error {
	key, kErr := db.Client.Put(ctx, u.Key, u)
	if kErr != nil {
		return kErr
	}
	u.ID = key.Encode()
	u.Key = key
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

func (u *User) DeriveFullName() string {
	untrimmed := fmt.Sprintf("%s %s", u.FirstName, u.LastName)
	u.FullName = strings.TrimSpace(untrimmed)
	return u.FullName
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

func NewUserWithOAuth(email, firstname, lastname, oauthprovider, oauthtoken string) (User, error) {
	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           email,
		FirstName:       firstname,
		LastName:        lastname,
		FullName:        "",
		PasswordDigest:  "",
		Token:           random.Token(),
		OAuthGoogleID:   "",
		OAuthFacebookID: "",
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
