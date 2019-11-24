package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/olivere/elastic/v7"
	"golang.org/x/crypto/bcrypt"

	"github.com/hiconvo/api/db"
	notif "github.com/hiconvo/api/notifications"
	"github.com/hiconvo/api/queue"
	"github.com/hiconvo/api/search"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/reporter"
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
}

func NewIncompleteUser(email string) (User, error) {
	femail := strings.ToLower(email)

	user := User{
		Key:       datastore.IncompleteKey("User", nil),
		Email:     femail,
		FirstName: strings.Split(femail, "@")[0],
		Token:     random.Token(),
		Verified:  false,
		CreatedAt: time.Now(),
	}

	return user, nil
}

func NewUserWithPassword(email, firstname, lastname, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return User{}, err
	}

	femail := strings.ToLower(email)

	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           femail,
		FirstName:       firstname,
		LastName:        lastname,
		FullName:        "",
		PasswordDigest:  string(hash),
		Token:           random.Token(),
		OAuthGoogleID:   "",
		OAuthFacebookID: "",
		Verified:        false,
		CreatedAt:       time.Now(),
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

	femail := strings.ToLower(email)

	user := User{
		Key:             datastore.IncompleteKey("User", nil),
		Email:           femail,
		Emails:          []string{femail},
		FirstName:       firstname,
		LastName:        lastname,
		FullName:        "",
		Avatar:          avatar,
		PasswordDigest:  "",
		Token:           random.Token(),
		OAuthGoogleID:   googleID,
		OAuthFacebookID: facebookID,
		Verified:        true,
		CreatedAt:       time.Now(),
	}

	return user, nil
}

func (u *User) LoadKey(k *datastore.Key) error {
	u.Key = k

	// Add URL safe key
	u.ID = k.Encode()

	// Generate the streamer token if not already present
	if u.RealtimeToken == "" {
		u.RealtimeToken = notif.GenerateToken(u.ID)
	}

	return nil
}

func (u *User) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(u)
}

func (u *User) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(u, ps); err != nil {
		if mismatch, ok := err.(*datastore.ErrFieldMismatch); ok {
			if !(mismatch.FieldName == "Threads" || mismatch.FieldName == "Events") {
				return err
			}
		} else {
			return err
		}
	}

	u.DeriveProperties()

	return nil
}

func (u *User) Commit(ctx context.Context) error {
	// Add CreatedAt date
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	// If the user has both first and last names, capitalize them
	// properly
	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(u.FirstName)
		u.LastName = strings.Title(u.LastName)
	}

	// Trim whitespace around the user's names
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)

	key, err := db.Client.Put(ctx, u.Key, u)
	if err != nil {
		return err
	}

	u.ID = key.Encode()
	u.Key = key

	// We have to do this after the user has been saved because we need the
	// ID, which isn't available until the user is in the database
	u.RealtimeToken = notif.GenerateToken(u.ID)

	u.DeriveProperties()
	u.CreateOrUpdateSearchIndex(ctx)

	return nil
}

func (u *User) CreateOrUpdateSearchIndex(ctx context.Context) {
	if u.IsRegistered() {
		_, upsertErr := search.Client.Update().
			Index("users").
			Id(u.ID).
			DocAsUpsert(true).
			Doc(MapUserToUserPartial(u)).
			Do(ctx)
		if upsertErr != nil {
			reporter.Report(fmt.Errorf("Failed to index user in elasticsearch: %v", upsertErr))
		}
	}
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
	// This is repeated from Commit above and handles getting users whose
	// names are improperly formatted. Eventaually this should be removed.
	if u.FirstName != "" && u.LastName != "" {
		u.FirstName = strings.Title(u.FirstName)
		u.LastName = strings.Title(u.LastName)
	}
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)

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

func (u *User) IsRegistered() bool {
	return (u.IsGoogleLinked || u.IsFacebookLinked || u.IsPasswordSet) && u.Verified
}

func (u *User) SendPasswordResetEmail() error {
	magicLink := magic.NewLink(u.Key, u.PasswordDigest, "reset")
	return sendPasswordResetEmail(u, magicLink)
}

func (u *User) SendVerifyEmail(email string) error {
	femail := strings.ToLower(email)
	salt := femail + strconv.FormatBool(u.HasEmail(femail))
	magicLink := magic.NewLink(u.Key, salt, "verify/"+femail)
	return sendVerifyEmail(u, email, magicLink)
}

func (u *User) SendMergeAccountsEmail(emailToMerge string) error {
	femail := strings.ToLower(emailToMerge)
	salt := femail + strconv.FormatBool(u.HasEmail(femail))
	magicLink := magic.NewLink(u.Key, salt, "verify/"+femail)
	return sendMergeAccountsEmail(u, femail, magicLink)
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

// HasEmail returns true when the user has the given email and it it verified.
func (u *User) HasEmail(email string) bool {
	femail := strings.ToLower(email)

	for i := range u.Emails {
		if u.Emails[i] == femail {
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
	femail := strings.ToLower(email)

	if !u.HasEmail(email) {
		return nil
	}

	if u.Email == femail {
		return errors.New("You cannot remove your primary email")
	}

	for i := range u.Emails {
		if u.Emails[i] == femail {
			u.Emails[i] = u.Emails[len(u.Emails)-1]
			u.Emails = u.Emails[:len(u.Emails)-1]
			break
		}
	}

	return nil
}

func (u *User) MakeEmailPrimary(email string) error {
	if !u.HasEmail(email) {
		return errors.New("You cannot make an unverified email primary")
	}

	u.Email = strings.ToLower(email)
	u.Verified = true

	return nil
}

func (u *User) Welcome(ctx context.Context) {
	thread, err := NewThread("Welcome", supportUser, []*User{u})
	if err != nil {
		reporter.Report(fmt.Errorf("user.Welcome: %v", err))
		return
	}

	if err := thread.Commit(ctx); err != nil {
		reporter.Report(fmt.Errorf("user.Welcome: %v", err))
		return
	}

	if err := u.Commit(ctx); err != nil {
		reporter.Report(fmt.Errorf("user.Welcome: %v", err))
		return
	}

	message, err := NewThreadMessage(supportUser, &thread, welcomeMessage)
	if err != nil {
		reporter.Report(fmt.Errorf("user.Welcome: %v", err))
		return
	}

	if err := message.Commit(ctx); err != nil {
		reporter.Report(fmt.Errorf("user.Welcome: %v", err))
		return
	}

	// We have to save the thread again, which is annoying
	if err := thread.Commit(ctx); err != nil {
		reporter.Report(fmt.Errorf("user.Welcome: %v", err))
		return
	}
}

func (u *User) SendDigest(ctx context.Context) error {
	// Get all of the users events. We don't include threads in
	// the digest email because threads are always emailed when
	// a new message is added.
	events, err := GetEventsByUser(ctx, u)
	if err != nil {
		return err
	}

	// Convert the events into Digestables and filter out read items
	var digestables []Digestable
	// Save the upcoming events to a slice at the same time
	var upcoming []*Event
	for i := range events {
		if !IsRead(events[i], u.Key) {
			digestables = append(digestables, events[i])
		}

		if events[i].IsUpcoming() {
			upcoming = append(upcoming, events[i])
		}
	}

	digestList, err := GenerateDigestList(ctx, digestables, u)
	if err != nil {
		return err
	}

	if len(digestList) > 0 || len(upcoming) > 0 {
		if err := sendDigest(digestList, upcoming, u); err != nil {
			return err
		}

		if err := MarkDigestedMessagesAsRead(ctx, digestList, u); err != nil {
			return err
		}
	}

	return nil
}

func (u *User) MergeWith(ctx context.Context, oldUser *User) error {
	if u.Key.Incomplete() {
		return errors.New("MergeWith: User's key is incomplete")
	}

	if oldUser.Key.Incomplete() {
		return errors.New("MergeWith: oldUser's key is incomplete")
	}

	_, err := db.Client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		// Contacts
		err := reassignContacts(ctx, tx, oldUser, u)
		if err != nil {
			tx.Rollback()
			return err
		}

		// Messages
		err = reassignMessageUsers(ctx, tx, oldUser, u)
		if err != nil {
			tx.Rollback()
			return err
		}

		// Threads
		err = reassignThreadUsers(ctx, tx, oldUser, u)
		if err != nil {
			tx.Rollback()
			return err
		}

		// Events
		err = reassignEventUsers(ctx, tx, oldUser, u)
		if err != nil {
			tx.Rollback()
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
		_, err = tx.Put(u.Key, u)
		if err != nil {
			tx.Rollback()
			return err
		}

		// Remove the old user from search
		if oldUser.IsRegistered() {
			_, err = search.Client.Delete().
				Index("users").
				Id(oldUser.ID).
				Do(ctx)
			if err != nil {
				reporter.Report(fmt.Errorf("Failed to remove user from elasticsearch: %v", err))
			}
		}

		// Delete the old user
		err = tx.Delete(oldUser.Key)
		if err != nil {
			tx.Rollback()
			return err
		}

		return nil
	})

	if err != nil {
		u.CreateOrUpdateSearchIndex(ctx)
	}

	return err
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

func UserWelcomeMulti(ctx context.Context, users []User) {
	ids := make([]string, len(users))
	for i := range users {
		ids[i] = users[i].ID
	}

	queue.PutEmail(ctx, queue.EmailPayload{
		Type:   queue.User,
		Action: queue.SendWelcome,
		IDs:    ids,
	})
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
	femail := strings.ToLower(email)

	u, found, err := getUserByField(ctx, "Email", femail)
	if !found && err == nil {
		return getUserByField(ctx, "Emails", femail)
	}

	return u, found, err
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
	userKeys = append(userKeys, t.OwnerKey)

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
	var users []User

	q := datastore.NewQuery("User").Filter(fmt.Sprintf("%s =", field), value)

	keys, getErr := db.Client.GetAll(ctx, q, &users)

	if getErr != nil {
		return User{}, false, getErr
	}

	if len(keys) == 1 {
		user := users[0]
		return user, true, nil
	}

	if len(keys) > 1 {
		return User{}, false, fmt.Errorf("%s is duplicated", field)
	}

	return User{}, false, nil
}
