package model

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/crypto/bcrypt"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/random"
	"github.com/hiconvo/api/valid"
)

type User struct {
	Key              *datastore.Key   `json:"-"        datastore:"__key__"`
	ID               string           `json:"id"       datastore:"-"`
	Email            string           `json:"email"`
	Emails           []string         `json:"emails"`
	FirstName        string           `json:"firstName"`
	LastName         string           `json:"lastName"`
	FullName         string           `json:"fullName" datastore:"-"`
	Token            string           `json:"token"`
	RealtimeToken    string           `json:"realtimeToken"`
	PasswordDigest   string           `json:"-"        datastore:",noindex"`
	OAuthGoogleID    string           `json:"-"`
	OAuthFacebookID  string           `json:"-"`
	IsPasswordSet    bool             `json:"isPasswordSet"    datastore:"-"`
	IsGoogleLinked   bool             `json:"isGoogleLinked"   datastore:"-"`
	IsFacebookLinked bool             `json:"isFacebookLinked" datastore:"-"`
	IsLocked         bool             `json:"-"`
	Verified         bool             `json:"verified"`
	Avatar           string           `json:"avatar"`
	ContactKeys      []*datastore.Key `json:"-"`
	Contacts         []*UserPartial   `json:"-"        datastore:"-"`
	CreatedAt        time.Time        `json:"-"`
	UpdatedAt        time.Time        `json:"-"`
	SendDigest       bool             `json:"sendDigest"`
	SendThreads      bool             `json:"sendThreads"`
	SendEvents       bool             `json:"sendEvents"`
	Tags             TagList          `json:"tags"`
}

type UserInput struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UserPartial struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
	Avatar    string `json:"avatar"`
}

type UserStore interface {
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, bool, error)
	GetUserByToken(ctx context.Context, token string) (*User, bool, error)
	GetUserByOAuthID(ctx context.Context, oauthtoken, provider string) (*User, bool, error)
	// GetUsersByThread(ctx context.Context, t *Thread) ([]*User, error)
	GetUsersByContact(ctx context.Context, u *User) ([]*User, error)
	GetOrCreateUserByEmail(ctx context.Context, email string) (u *User, created bool, err error)
	GetOrCreateUsers(ctx context.Context, users []*UserInput) ([]*User, error)
	GetContactsByUser(ctx context.Context, u *User) ([]*User, error)
	Search(ctx context.Context, query string) ([]*UserPartial, error)
	Commit(ctx context.Context, u *User) error
	CommitWithTransaction(tx db.Transaction, u *User) (*datastore.PendingKey, error)
	DeleteWithTransaction(ctx context.Context, tx db.Transaction, u *User) error
	CreateOrUpdateSearchIndex(ctx context.Context, u *User)
	IterAll(ctx context.Context) *datastore.Iterator
}

type Welcomer interface {
	Welcome(
		ctx context.Context,
		ts ThreadStore,
		sclient *storage.Client,
		u *User) error
}

func NewIncompleteUser(emailAddress string) (*User, error) {
	email, err := valid.Email(emailAddress)
	if err != nil {
		return nil, errors.E(errors.Op("model.NewIncompleteUser()"), err)
	}

	user := User{
		Key:         datastore.IncompleteKey("User", nil),
		Email:       email,
		FirstName:   strings.Split(email, "@")[0],
		Token:       random.Token(),
		Verified:    false,
		CreatedAt:   time.Now(),
		SendDigest:  true,
		SendThreads: true,
		SendEvents:  true,
	}

	return &user, nil
}

func NewUserWithPassword(emailAddress, firstName, lastName, password string) (*User, error) {
	email, err := valid.Email(emailAddress)
	if err != nil {
		return nil, errors.E(errors.Op("model.NewIncompleteUser()"), err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return nil, errors.E(errors.Opf("model.NewUserWithPassword(%s)", emailAddress), err)
	}

	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           email,
		FirstName:       firstName,
		LastName:        lastName,
		FullName:        "",
		PasswordDigest:  string(hash),
		Token:           random.Token(),
		OAuthGoogleID:   "",
		OAuthFacebookID: "",
		Verified:        false,
		CreatedAt:       time.Now(),
		SendDigest:      true,
		SendThreads:     true,
		SendEvents:      true,
	}

	return &user, nil
}

func NewUserWithOAuth(emailAddress, firstName, lastName, avatar, oAuthProvider, oAuthToken string) (*User, error) {
	var (
		op         = errors.Op("model.NewUserWithOAuth()")
		googleID   string
		facebookID string
	)

	switch oAuthProvider {
	case "google":
		googleID = oAuthToken
	case "facebook":
		facebookID = oAuthToken
	default:
		return nil, errors.E(op, errors.Errorf("%q is not a valid oAuthProvider", oAuthProvider))
	}

	email, err := valid.Email(emailAddress)
	if err != nil {
		return nil, errors.E(errors.Op("model.NewIncompleteUser()"), err)
	}

	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           email,
		Emails:          []string{email},
		FirstName:       firstName,
		LastName:        lastName,
		FullName:        "",
		Avatar:          avatar,
		PasswordDigest:  "",
		Token:           random.Token(),
		OAuthGoogleID:   googleID,
		OAuthFacebookID: facebookID,
		Verified:        true,
		CreatedAt:       time.Now(),
		SendDigest:      true,
		SendThreads:     true,
		SendEvents:      true,
	}

	return &user, nil
}

func (u *User) LoadKey(k *datastore.Key) error {
	u.Key = k

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

	u.SendDigest = true
	u.SendThreads = true
	u.SendEvents = true

	for _, p := range ps {
		if p.Name == "SendDigest" {
			val := p.Value.(bool)
			u.SendDigest = val
		}

		if p.Name == "SendThreads" {
			val := p.Value.(bool)
			u.SendThreads = val
		}

		if p.Name == "SendEvents" {
			val := p.Value.(bool)
			u.SendEvents = val
		}
	}

	u.DeriveProperties()

	return nil
}

func (u *User) GetPasswordResetMagicLink(m magic.Client) string {
	return m.NewLink(u.Key, u.PasswordDigest, "reset")
}

func (u *User) VerifyPasswordResetMagicLink(m magic.Client, id, ts, sig string) error {
	return m.Verify(id, ts, u.PasswordDigest, sig)
}

func (u *User) GetVerifyEmailMagicLink(m magic.Client, email string) string {
	salt := email + strconv.FormatBool(u.HasEmail(email))
	return m.NewLink(u.Key, salt, "verify/"+email)
}

func (u *User) VerifyEmailMagicLink(m magic.Client, email, id, ts, sig string) error {
	salt := email + strconv.FormatBool(u.HasEmail(email))
	return m.Verify(id, ts, salt, sig)
}

func (u *User) GetMagicLoginMagicLink(m magic.Client) string {
	return m.NewLink(u.Key, u.Token, "magic")
}

func (u *User) VerifyMagicLogin(m magic.Client, id, ts, sig string) error {
	return m.Verify(id, ts, u.Token, sig)
}

func (u *User) GetUnsubscribeMagicLink(m magic.Client) string {
	return m.NewLink(u.Key, u.Token, "unsubscribe")
}

func (u *User) VerifyUnsubscribeMagicLink(m magic.Client, id, ts, sig string) error {
	return m.Verify(id, ts, u.Token, sig)
}

func (u *User) HasEmail(email string) bool {
	email = strings.ToLower(email)

	for i := range u.Emails {
		if u.Emails[i] == email {
			return true
		}
	}

	return false
}

// AddEmail adds a verified email to the user's Emails field. Only verified emails
// should be added.
func (u *User) AddEmail(email string) {
	femail := strings.ToLower(email)

	if u.HasEmail(femail) {
		return
	}

	u.Emails = append(u.Emails, femail)
}

// RemoveEmail removes the given email from the user's Emails field, if the email is present.
func (u *User) RemoveEmail(email string) error {
	email = strings.ToLower(email)

	if !u.HasEmail(email) {
		return nil
	}

	if u.Email == email {
		return errors.E(errors.Opf("models.RemoveEmail(%q)", email), http.StatusBadRequest, map[string]string{
			"message": "You cannot remove your primary email"})
	}

	for i := range u.Emails {
		if u.Emails[i] == email {
			u.Emails[i] = u.Emails[len(u.Emails)-1]
			u.Emails = u.Emails[:len(u.Emails)-1]

			break
		}
	}

	return nil
}

func (u *User) MakeEmailPrimary(email string) error {
	if !u.HasEmail(email) {
		return errors.E(errors.Opf("models.MakeEmailPrimary(%q)", email), http.StatusBadRequest, map[string]string{
			"message": "You cannot make an unverified email primary"})
	}

	u.Email = strings.ToLower(email)
	u.Verified = true

	return nil
}

func (u *User) IsRegistered() bool {
	return (u.IsGoogleLinked || u.IsFacebookLinked || u.IsPasswordSet) && u.Verified
}

func (u *User) DeriveProperties() {
	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(u.FirstName)
		u.LastName = strings.Title(u.LastName)
	}

	// Derive the full name
	u.FullName = strings.TrimSpace(fmt.Sprintf("%s %s", u.FirstName, u.LastName))

	// Derive useful bools
	u.IsPasswordSet = u.PasswordDigest != ""
	u.IsGoogleLinked = u.OAuthGoogleID != ""
	u.IsFacebookLinked = u.OAuthFacebookID != ""

	// For handling transition from single to multi-email model. If the single email was
	// verified, add it to the users Emails list.
	if u.Verified && !u.HasEmail(u.Email) {
		u.AddEmail(u.Email)
	}

	// Make the user's primary email one that is verified if possible.
	if !u.Verified && !u.HasEmail(u.Email) && len(u.Emails) > 0 {
		u.Email = u.Emails[0]
	}

	u.Verified = u.HasEmail(u.Email)
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

func (u *User) MergeWith(
	ctx context.Context,
	transacter db.Transacter,
	us UserStore,
	ms MessageStore,
	ts ThreadStore,
	es EventStore,
	ns NoteStore,
	oldUser *User,
) error {
	if u.Key.Incomplete() {
		return errors.Str("models.MergeWith: user's key is incomplete")
	}

	if oldUser.Key.Incomplete() {
		return errors.Str("models.MergeWith: oldUser's key is incomplete")
	}

	_, err := transacter.RunInTransaction(ctx, func(tx db.Transaction) error {
		// Contacts
		err := reassignContacts(ctx, tx, us, oldUser, u)
		if err != nil {
			return err
		}

		// Messages
		err = reassignMessageUsers(ctx, tx, ms, oldUser, u)
		if err != nil {
			return err
		}

		// Threads
		err = reassignThreadUsers(ctx, tx, ts, oldUser, u)
		if err != nil {
			return err
		}

		// Events
		err = reassignEventUsers(ctx, tx, es, oldUser, u)
		if err != nil {
			return err
		}

		// Notes
		err = reassignNoteUsers(ctx, tx, ns, oldUser, u)
		if err != nil {
			return err
		}

		// User details
		if oldUser.Avatar != "" && u.Avatar == "" {
			u.Avatar = oldUser.Avatar
		}
		if oldUser.FirstName != "" && u.FirstName == "" {
			u.FirstName = oldUser.FirstName
		}
		if oldUser.LastName != "" && u.LastName == "" {
			u.LastName = oldUser.LastName
		}
		for _, email := range oldUser.Emails {
			u.AddEmail(email)
		}
		u.ContactKeys = mergeContacts(u.ContactKeys, oldUser.ContactKeys)
		u.RemoveContact(oldUser)

		// Save user
		_, err = us.CommitWithTransaction(tx, u)
		if err != nil {
			return err
		}

		err = us.DeleteWithTransaction(ctx, tx, oldUser)
		if err != nil {
			return err
		}

		return nil
	})

	us.CreateOrUpdateSearchIndex(ctx, u)

	return err
}

func (u *User) AddContact(c *User) error {
	var op errors.Op = "models.AddContact"

	if u.HasContact(c) {
		return errors.E(op, http.StatusBadRequest, map[string]string{
			"message": "You already have this contact"})
	}

	if u.Key.Equal(c.Key) {
		return errors.E(op, http.StatusBadRequest, map[string]string{
			"message": "You cannot add yourself as a contact"})
	}

	if len(u.ContactKeys) >= 50 {
		return errors.E(op, http.StatusBadRequest, map[string]string{
			"message": "You can have a maximum of 50 contacts"})
	}

	u.ContactKeys = append(u.ContactKeys, c.Key)

	return nil
}

func (u *User) RemoveContact(c *User) error {
	if !u.HasContact(c) {
		return errors.E(
			errors.Op("user.RemoveContact"),
			http.StatusBadRequest,
			map[string]string{"message": "You don't have this contact"})
	}

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

func MapUserToUserPartial(u *User) *UserPartial {
	// If the user does not have any name info, show the part of their email
	// before the "@"
	var fullName string
	if u.FirstName == "" && u.LastName == "" && u.FullName == "" {
		fullName = strings.Split(u.Email, "@")[0]
	} else {
		fullName = u.FullName
	}

	return &UserPartial{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		FullName:  fullName,
		Avatar:    u.Avatar,
	}
}

func MapUsersToUserPartials(users []*User) []*UserPartial {
	ups := make([]*UserPartial, len(users))
	for i, u := range users {
		ups[i] = MapUserToUserPartial(u)
	}

	return ups
}

func MapUserPartialToUser(c *UserPartial, users []*User) (*User, error) {
	for _, u := range users {
		if u.ID == c.ID {
			return u, nil
		}
	}

	return nil, errors.Str("Matching user not in slice")
}

func MapUsersToKeys(users []*User) []*datastore.Key {
	ups := make([]*datastore.Key, len(users))
	for i, u := range users {
		ups[i] = u.Key
	}

	return ups
}

func UserWelcomeMulti(ctx context.Context, q queue.Client, users []*User) {
	ids := make([]string, len(users))
	for i := range users {
		ids[i] = users[i].ID
	}

	err := q.PutEmail(ctx, queue.EmailPayload{
		Type:   queue.User,
		Action: queue.SendWelcome,
		IDs:    ids,
	})

	if err != nil {
		log.Alarm(errors.E(errors.Op("model.UserWelcomeMulti"), err))
	}
}
