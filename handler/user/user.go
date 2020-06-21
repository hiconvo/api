package user

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/clients/oauth"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

type Config struct {
	Transacter   db.Transacter
	UserStore    model.UserStore
	ThreadStore  model.ThreadStore
	EventStore   model.EventStore
	MessageStore model.MessageStore
	Mail         *mail.Client
	Magic        magic.Client
	OA           oauth.Client
	Storage      *storage.Client
	Welcome      model.Welcomer
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/users", c.CreateUser).Methods("POST")
	r.HandleFunc("/users/auth", c.AuthenticateUser).Methods("POST")
	r.HandleFunc("/users/oauth", c.OAuth).Methods("POST")
	r.HandleFunc("/users/password", c.UpdatePassword).Methods("POST")
	r.HandleFunc("/users/verify", c.VerifyEmail).Methods("POST")
	r.HandleFunc("/users/forgot", c.ForgotPassword).Methods("POST")
	r.HandleFunc("/users/magic", c.MagicLogin).Methods("POST")
	r.HandleFunc("/users/unsubscribe", c.MagicUnsubscribe).Methods("POST")

	s := r.NewRoute().Subrouter()
	s.Use(middleware.WithUser(c.UserStore))
	s.HandleFunc("/users", c.GetCurrentUser).Methods("GET")
	s.HandleFunc("/users", c.UpdateUser).Methods("PATCH")
	s.HandleFunc("/users/emails", c.AddEmail).Methods("POST")
	s.HandleFunc("/users/emails", c.RemoveEmail).Methods("DELETE")
	s.HandleFunc("/users/emails", c.MakeEmailPrimary).Methods("PATCH")
	s.HandleFunc("/users/resend", c.SendVerifyEmail).Methods("POST")
	s.HandleFunc("/users/search", c.UserSearch).Methods("GET")
	s.HandleFunc("/users/avatar", c.PutAvatar).Methods("POST")
	s.HandleFunc("/users/{userID}", c.GetUser).Methods("GET")

	return r
}

type createUserPayload struct {
	Email     string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
	FirstName string `validate:"nonzero"`
	LastName  string
	Password  string `validate:"min=8"`
}

// CreateUser creates a new user with a password.
func (c *Config) CreateUser(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handler.CreateUser")
	ctx := r.Context()

	var payload createUserPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Make sure the user is not already registered
	foundUser, found, err := c.UserStore.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleError(w, err)
		return
	} else if found {
		if !foundUser.IsPasswordSet && !foundUser.IsGoogleLinked && !foundUser.IsFacebookLinked {
			// The email is registered but the user has not setup their account.
			// In order to make sure the requestor is who they say thay are and is not
			// trying to gain access to someone else's identity, we lock the account and
			// require that the email be verified before the user can get access.
			foundUser.FirstName = payload.FirstName
			foundUser.LastName = payload.LastName
			foundUser.IsLocked = true

			if err := c.Mail.SendPasswordResetEmail(
				foundUser,
				foundUser.GetPasswordResetMagicLink(c.Magic)); err != nil {
				bjson.HandleError(w, err)
				return
			}

			if err := c.UserStore.Commit(ctx, foundUser); err != nil {
				bjson.HandleError(w, err)
				return
			}

			bjson.WriteJSON(w, map[string]string{
				"message": "Please verify your email to proceed",
			}, http.StatusOK)
			return
		}

		bjson.HandleError(w, errors.E(op,
			errors.Str("already registered"),
			map[string]string{"message": "This email has already been registered"},
			http.StatusBadRequest))
		return
	}

	// Create the user object
	user, err := model.NewUserWithPassword(
		payload.Email,
		payload.FirstName,
		payload.LastName,
		payload.Password)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Save the user object
	if err := c.UserStore.Commit(ctx, user); err != nil {
		bjson.HandleError(w, err)
		return
	}

	err = c.Mail.SendVerifyEmail(user, user.Email, user.GetVerifyEmailMagicLink(c.Magic, user.Email))
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.Welcome.Welcome(ctx, c.ThreadStore, c.MessageStore, user); err != nil {
		log.Alarm(err)
	}

	bjson.WriteJSON(w, user, http.StatusCreated)
}

type authenticateUserPayload struct {
	Email    string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
	Password string `validate:"nonzero"`
}

// AuthenticateUser is an endpoint that authenticates a user with a password.
func (c *Config) AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.AuthenticateUser")
	ctx := r.Context()

	var payload authenticateUserPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, found, err := c.UserStore.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleError(w, err)
		return
	} else if !found {
		bjson.HandleError(w, errors.E(op,
			errors.Str("unknown email"),
			map[string]string{"message": "Invalid credentials"},
			http.StatusBadRequest))
		return
	}

	if u.CheckPassword(payload.Password) {
		if u.IsLocked {
			if err := c.Mail.SendVerifyEmail(
				u, u.Email, u.GetVerifyEmailMagicLink(c.Magic, u.Email)); err != nil {
				log.Alarm(err)
			}

			bjson.HandleError(w, errors.E(op,
				errors.Str("unknown email"),
				map[string]string{"message": "You must verify your email before you can login"},
				http.StatusBadRequest))

			return
		}

		bjson.WriteJSON(w, u, http.StatusOK)

		return
	}

	bjson.HandleError(w, errors.E(op,
		errors.Str("invalid password"),
		map[string]string{"message": "Invalid credentials"},
		http.StatusBadRequest))
}

// OAuth is an endpoint that can do four things. It can
//   1. create a user with oauth based authentication
//   2. associate an existing user with an oauth token
//   3. authenticate a user via oauth
//   4. merge an oauth based account with an unregestered account
//      that was created as a result of an email-based invitation
//
func (c *Config) OAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload oauth.UserPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	oauthPayload, err := c.OA.Verify(ctx, payload)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// If this request includes a token, then it could be from a user
	// who RSVP'd and then tried to connect an oauth account. If the
	// token user is not fully registered, this user was made as a
	// result of an email-based invitation. In this case, we mark
	// canMergeTokenUser as true and attempt to merge the token user
	// into the oauth user if all else goes well below.
	var (
		token, includesToken = middleware.GetAuthToken(r.Header)
		tokenUser            *model.User
		canMergeTokenUser    bool = false
	)

	if includesToken {
		_tokenUser, ok, err := c.UserStore.GetUserByToken(ctx, token)

		if ok && err == nil {
			canMergeTokenUser = !_tokenUser.Key.Incomplete() && !_tokenUser.IsRegistered()
			tokenUser = _tokenUser
		}
	}

	// Get the user and return if found
	u, found, err := c.UserStore.GetUserByOAuthID(ctx, oauthPayload.ID, oauthPayload.Provider)
	if err != nil {
		bjson.HandleError(w, err)
		return
	} else if found {
		// If the user associated with the token is different from the user
		// received via oauth, merge the token user into the oauth user.
		if canMergeTokenUser && u.Token != tokenUser.Token {
			if err := u.MergeWith(
				ctx,
				c.Transacter,
				c.UserStore,
				c.MessageStore,
				c.ThreadStore,
				c.EventStore,
				tokenUser,
			); err != nil {
				bjson.HandleError(w, err)
				return
			}
		}
		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	// This user hasn't used Oauth before. Try to find the user by email.
	// If found, associate the new token with the existing user.
	u, found, err = c.UserStore.GetUserByEmail(ctx, oauthPayload.Email)
	if err != nil {
		bjson.HandleError(w, err)
		return
	} else if found {
		if oauthPayload.Provider == "google" {
			u.OAuthGoogleID = oauthPayload.ID
		} else {
			u.OAuthFacebookID = oauthPayload.ID
		}

		// Add missing user data
		if u.FirstName == "" || u.LastName == "" {
			u.FirstName = oauthPayload.FirstName
			u.LastName = oauthPayload.LastName
			u.FullName = strings.Join([]string{
				oauthPayload.FirstName,
				oauthPayload.LastName,
			}, " ")
		}

		if u.Avatar == "" {
			avatarURI, err := c.Storage.PutAvatarFromURL(ctx, oauthPayload.TempAvatar)
			if err != nil {
				// Print error but keep going. User might not have a profile pic.
				log.Alarm(err)
			}
			u.Avatar = avatarURI
		}

		u.AddEmail(oauthPayload.Email)
		u.DeriveProperties()

		if err := c.UserStore.Commit(ctx, u); err != nil {
			bjson.HandleError(w, err)
			return
		}

		// Same as above:
		// If the user associated with the token is different from the user
		// received via oauth, merge the token user into the oauth user.
		if canMergeTokenUser && u.Token != tokenUser.Token {
			if err := u.MergeWith(
				ctx,
				c.Transacter,
				c.UserStore,
				c.MessageStore,
				c.ThreadStore,
				c.EventStore,
				tokenUser,
			); err != nil {
				bjson.HandleError(w, err)
				return
			}
		}

		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	// Finally at new user case. Cache the avatar from the Oauth payload and
	// create a new account with the Oauth payload.
	avatarURI, err := c.Storage.PutAvatarFromURL(ctx, oauthPayload.TempAvatar)
	if err != nil {
		// Print error but keep going. User might not have a profile pic.
		log.Alarm(err)
	}

	u, err = model.NewUserWithOAuth(
		oauthPayload.Email,
		oauthPayload.FirstName,
		oauthPayload.LastName,
		avatarURI,
		oauthPayload.Provider,
		oauthPayload.ID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Save the user and create the welcome convo.
	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.Welcome.Welcome(ctx, c.ThreadStore, c.MessageStore, u); err != nil {
		log.Alarm(err)
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

type updatePasswordPayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
	Password  string `validate:"min=8"`
}

// UpdatePassword updates a user's password.
func (c *Config) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload updatePasswordPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, err := c.UserStore.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.UpdatePassword"),
			err,
			http.StatusBadRequest))

		return
	}

	if err := u.VerifyPasswordResetMagicLink(
		c.Magic,
		payload.UserID,
		payload.Timestamp,
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := magic.TooOld(payload.Timestamp); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u.ChangePassword(payload.Password)
	u.IsLocked = false
	u.AddEmail(u.Email)
	u.DeriveProperties()

	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// GetCurrentUser is an endpoint that returns the current user.
func (c *Config) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	bjson.WriteJSON(w, u, http.StatusOK)
}

// GetUser is an endpoint that returns the requested user if found.
func (c *Config) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userID := vars["userID"]

	u, err := c.UserStore.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, errors.E(errors.Op("handlers.GetUser"), err, http.StatusNotFound))
		return
	}

	bjson.WriteJSON(w, model.MapUserToUserPartial(u), http.StatusOK)
}

type verifyEmailPayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
	Email     string `validate:"nonzero"`
}

// VerifyEmail verifies that the user's email is really hers. It is secured
// with a signature.
func (c *Config) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload verifyEmailPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, err := c.UserStore.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.VerifyEmail"),
			err,
			http.StatusBadRequest))

		return
	}

	if err := u.VerifyEmailMagicLink(
		c.Magic,
		payload.Email,
		payload.UserID,
		payload.Timestamp,
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := magic.TooOld(payload.Timestamp); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// If there is already an account associated with this email, merge the two accounts.
	dupUser, found, err := c.UserStore.GetUserByEmail(ctx, payload.Email)
	if found && !dupUser.Key.Equal(u.Key) {
		if err := u.MergeWith(
			ctx,
			c.Transacter,
			c.UserStore,
			c.MessageStore,
			c.ThreadStore,
			c.EventStore,
			dupUser,
		); err != nil {
			bjson.HandleError(w, err)
			return
		}
	} else if err != nil {
		bjson.HandleError(w, err)
		return
	}

	u.AddEmail(payload.Email)
	u.DeriveProperties()

	if u.Email == payload.Email {
		u.IsLocked = false
	}

	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

type forgotPasswordPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// ForgotPassword sends a set password email to the user whose email
// matches the email given in the payload. If no matching user is found
// this does nothing.
func (c *Config) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload forgotPasswordPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, found, err := c.UserStore.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleError(w, err)
		return
	} else if found {
		if err := c.Mail.SendPasswordResetEmail(u,
			u.GetPasswordResetMagicLink(c.Magic)); err != nil {
			log.Alarm(err)
		}
	}

	bjson.WriteJSON(w, map[string]string{
		"message": "Check your email for a link to reset your password",
	}, http.StatusOK)
}

type magicLoginPayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
}

// MagicLogin logs in a user with a signature.
func (c *Config) MagicLogin(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.MagicLogin")
	ctx := r.Context()

	var payload magicLoginPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, err := c.UserStore.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err, http.StatusUnauthorized))
		return
	}

	if err := u.VerifyMagicLogin(
		c.Magic,
		payload.UserID,
		payload.Timestamp,
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := magic.TooOld(payload.Timestamp); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

type updateUserPayload struct {
	FirstName   string
	LastName    string
	Password    bool
	SendDigest  *bool
	SendThreads *bool
	SendEvents  *bool
}

// UpdateUser is an endpoint that can do three things. It can
//   - update FirstName and LastName fields on a user
//   - initiate a password update which requires email based validation
//
func (c *Config) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	var payload updateUserPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if payload.Password {
		if err := c.Mail.SendPasswordResetEmail(u,
			u.GetPasswordResetMagicLink(c.Magic)); err != nil {
			log.Alarm(err)
		}
	}

	if payload.FirstName != "" && payload.FirstName != u.FirstName {
		u.FirstName = payload.FirstName
	}

	if payload.LastName != "" && payload.LastName != u.LastName {
		u.LastName = payload.LastName
	}

	if payload.SendDigest != nil {
		u.SendDigest = *payload.SendDigest
	}

	if payload.SendThreads != nil {
		u.SendThreads = *payload.SendThreads
	}

	if payload.SendEvents != nil {
		u.SendEvents = *payload.SendEvents
	}

	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// SendVerifyEmail resends the email verification email.
func (c *Config) SendVerifyEmail(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())

	err := c.Mail.SendVerifyEmail(u, u.Email, u.GetVerifyEmailMagicLink(c.Magic, u.Email))
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

type addEmailPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// AddEmail sends a verification email to the given email with a magic link that,
// when clicked, adds the new email to the user's account.
func (c *Config) AddEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	var payload addEmailPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	foundUser, found, err := c.UserStore.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if found && foundUser.IsRegistered() {
		// There's already an account associated with the email that the user
		// is attempting to add. Send an email to this address with some details
		// about what's going on.
		if err := c.Mail.SendMergeAccountsEmail(u, payload.Email,
			u.GetVerifyEmailMagicLink(c.Magic, payload.Email)); err != nil {
			bjson.HandleError(w, err)
			return
		}
	} else {
		if err := c.Mail.SendVerifyEmail(u, payload.Email,
			u.GetVerifyEmailMagicLink(c.Magic, payload.Email)); err != nil {
			bjson.HandleError(w, err)
			return
		}
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

type removeEmailPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// RemoveEmail removes the given email from the user's account.
func (c *Config) RemoveEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	var payload removeEmailPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.RemoveEmail(payload.Email); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

type makePrimaryEmailPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// MakeEmailPrimary removes the given email from the user's account.
func (c *Config) MakeEmailPrimary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	var payload makePrimaryEmailPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.MakeEmailPrimary(payload.Email); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// UserSearch returns search results.
func (c *Config) UserSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("query")
	if query == "" {
		bjson.WriteJSON(w, map[string]string{"message": "query cannot be empty"}, http.StatusBadRequest)
		return
	}

	contacts, err := c.UserStore.Search(ctx, query)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string]interface{}{"users": contacts}, http.StatusOK)
}

type putAvatarPayload struct {
	Blob string `validate:"nonzero"`
	X    float64
	Y    float64
	Size float64
}

// PutAvatar sets the user's avatar to the one given.
func (c *Config) PutAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	var payload putAvatarPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	avatarURL, err := c.Storage.PutAvatarFromBlob(
		ctx,
		payload.Blob,
		int(payload.Size),
		int(payload.X),
		int(payload.Y),
		c.Storage.GetKeyFromAvatarURL(u.Avatar))
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	u.Avatar = avatarURL
	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

type magicUnsubscribePayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
}

func (c *Config) MagicUnsubscribe(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.MagicUnsubscribe")
	ctx := r.Context()

	var payload magicUnsubscribePayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, err := c.UserStore.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err, http.StatusUnauthorized))
		return
	}

	if err := u.VerifyUnsubscribeMagicLink(
		c.Magic,
		payload.UserID,
		payload.Timestamp,
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u.SendDigest = false
	u.SendThreads = false
	u.SendEvents = false

	bjson.WriteJSON(w, map[string]string{"message": "unsubscribed"}, http.StatusOK)
}
