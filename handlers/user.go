package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/storage"
	"github.com/hiconvo/api/utils/bjson"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/oauth"
	"github.com/hiconvo/api/utils/validate"
)

var (
	errMsgCreate = map[string]string{"message": "Could not create user"}
	errMsgSave   = map[string]string{"message": "Could not save user"}
	errMsgGet    = map[string]string{"message": "Could not get user"}
	errMsgReg    = map[string]string{"message": "This email has already been registered"}
	errMsgCreds  = map[string]string{"message": "Invalid credentials"}
	errMsgMagic  = map[string]string{"message": "This link is not valid anymore"}
	errMsgSend   = map[string]string{"message": "Could not send email"}
	errMsgUpload = map[string]string{"message": "Could not upload image"}
)

// CreateUser Endpoint: POST /users
//
// Request payload:
type createUserPayload struct {
	Email     string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
	FirstName string `validate:"nonzero"`
	LastName  string
	Password  string `validate:"min=8"`
}

// CreateUser is an endpoint that creates a user with password based
// authentication.
func CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload createUserPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Make sure the user is not already registered
	foundUser, found, err := models.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreate)
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
			foundUser.SendPasswordResetEmail()
			if err := foundUser.Commit(ctx); err != nil {
				bjson.HandleInternalServerError(w, err, errMsgSave)
				return
			}
			bjson.WriteJSON(w, map[string]string{
				"message": "Please verify your email to proceed",
			}, http.StatusOK)
			return
		}

		bjson.WriteJSON(w, errMsgReg, http.StatusBadRequest)
		return
	}

	// Create the user object
	user, uerr := models.NewUserWithPassword(
		payload.Email,
		payload.FirstName,
		payload.LastName,
		payload.Password)
	if uerr != nil {
		bjson.HandleInternalServerError(w, uerr, errMsgCreate)
		return
	}

	// Save the user object
	if err := user.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreate)
		return
	}

	user.SendVerifyEmail(user.Email)
	user.Welcome(ctx)

	bjson.WriteJSON(w, user, http.StatusCreated)
}

// GetCurrentUser Endpoint: GET /users

// GetCurrentUser is an endpoint that returns the current user.
func GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	bjson.WriteJSON(w, u, http.StatusOK)
}

// GetUser Endpoint: GET /users/{id}

// GetUser is an endpoint that returns the requested user if found
func GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userID := vars["userID"]

	u, err := models.GetUserByID(ctx, userID)
	if err != nil {
		bjson.WriteJSON(w, errMsgGet, http.StatusNotFound)
		return
	}

	bjson.WriteJSON(w, models.MapUserToUserPartial(&u), http.StatusOK)
}

// AuthenticateUser Endpoint: POST /users/auth
//
// Request payload:
type authenticateUserPayload struct {
	Email    string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
	Password string `validate:"nonzero"`
}

// AuthenticateUser is an endpoint that authenticates a user with a password.
func AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload authenticateUserPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, found, err := models.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGet)
		return
	} else if !found {
		bjson.WriteJSON(w, errMsgCreds, http.StatusBadRequest)
		return
	}

	if u.CheckPassword(payload.Password) {
		if u.IsLocked {
			u.SendVerifyEmail(u.Email)

			bjson.WriteJSON(w, map[string]string{
				"message": "You must verify your email before you can login",
			}, http.StatusBadRequest)
			return
		}

		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	bjson.WriteJSON(w, errMsgCreds, http.StatusBadRequest)
}

// OAuth Endpoint: POST /users/oauth
//
// Request payload: oauth.UserPayload

// OAuth is an endpoint that can do four things. It can
//   1. create a user with oauth based authentication
//   2. associate an existing user with an oauth token
//   3. authenticate a user via oauth
//   4. merge an oauth based account with an unregestered account
//      that was created as a result of an email-based invitation
//
func OAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload oauth.UserPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	oauthPayload, err := oauth.Verify(ctx, payload)
	if err != nil {
		bjson.WriteJSON(w, errMsgCreds, http.StatusBadRequest)
		return
	}

	// If this request includes a token, then it could be from a user
	// who RSVP'd and then tried to connect an oauth account. If the
	// token user is not fully registered, this user was made as a
	// result of an email-based invitation. In this case, we mark
	// canMergeTokenUser as true and attempt to merge the token user
	// into the oauth user if all else goes well below.
	token, includesToken := middleware.GetAuthToken(r.Header)
	var tokenUser *models.User
	var canMergeTokenUser bool = false
	if includesToken {
		_tokenUser, ok, err := models.GetUserByToken(ctx, token)

		if ok && err == nil {
			canMergeTokenUser = !_tokenUser.Key.Incomplete() && !_tokenUser.IsRegistered()
			tokenUser = &_tokenUser
		}
	}

	// Get the user and return if found
	u, found, err := models.GetUserByOAuthID(ctx, oauthPayload.ID, oauthPayload.Provider)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGet)
		return
	} else if found {
		// If the user associated with the token is different from the user
		// received via oauth, merge the token user into the oauth user.
		if canMergeTokenUser && u.Token != tokenUser.Token {
			if err := u.MergeWith(ctx, tokenUser); err != nil {
				bjson.HandleInternalServerError(w, err, errMsgSave)
				return
			}
		}
		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	// This user hasn't used Oauth before. Try to find the user by email.
	// If found, associate the new token with the existing user.
	u, found, err = models.GetUserByEmail(ctx, oauthPayload.Email)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGet)
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
			avatarURI, err := storage.PutAvatarFromURL(ctx, oauthPayload.TempAvatar)
			if err != nil {
				// Print error but keep going. User might not have a profile pic.
				log.Alarm(err)
			}
			u.Avatar = avatarURI
		}

		u.AddEmail(oauthPayload.Email)
		u.DeriveProperties()

		if err := u.Commit(ctx); err != nil {
			bjson.HandleInternalServerError(w, err, errMsgSave)
			return
		}

		// Same as above:
		// If the user associated with the token is different from the user
		// received via oauth, merge the token user into the oauth user.
		if canMergeTokenUser && u.Token != tokenUser.Token {
			if err := u.MergeWith(ctx, tokenUser); err != nil {
				bjson.HandleInternalServerError(w, err, errMsgSave)
				return
			}
		}

		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	// Finally at new user case. Cache the avatar from the Oauth payload and
	// create a new account with the Oauth payload.
	avatarURI, err := storage.PutAvatarFromURL(ctx, oauthPayload.TempAvatar)
	if err != nil {
		// Print error but keep going. User might not have a profile pic.
		log.Alarm(err)
	}

	u, err = models.NewUserWithOAuth(
		oauthPayload.Email,
		oauthPayload.FirstName,
		oauthPayload.LastName,
		avatarURI,
		oauthPayload.Provider,
		oauthPayload.ID)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreate)
		return
	}

	// Save the user and create the welcome convo.
	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}
	u.Welcome(ctx)

	bjson.WriteJSON(w, u, http.StatusOK)
}

// UpdateUser Endpoint: PATCH /users
//
// Request payload:
type updateUserPayload struct {
	FirstName string
	LastName  string
	Password  bool
}

// UpdateUser is an endpoint that can do three things. It can
//   - update FirstName and LastName fields on a user
//   - initiate a password update which requires email based validation
//
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	var payload updateUserPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if payload.Password {
		u.SendPasswordResetEmail()
	}

	// TODO: Come up with something better than this.
	if payload.FirstName != "" && payload.FirstName != u.FirstName {
		u.FirstName = payload.FirstName
	}

	if payload.LastName != "" && payload.LastName != u.LastName {
		u.LastName = payload.LastName
	}

	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// UpdatePassword Endpoint: POST /users/password
//
// Request payload:
type updatePasswordPayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
	Password  string `validate:"min=8"`
}

// UpdatePassword updates a user's password.
func UpdatePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload updatePasswordPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, err := models.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.WriteJSON(w, errMsgMagic, http.StatusBadRequest)
		return
	}

	if !magic.Verify(
		payload.UserID,
		payload.Timestamp,
		u.PasswordDigest,
		payload.Signature,
	) {
		bjson.WriteJSON(w, errMsgMagic, http.StatusUnauthorized)
		return
	}

	u.ChangePassword(payload.Password)
	u.IsLocked = false
	u.AddEmail(u.Email)
	u.DeriveProperties()
	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// VerifyEmail Endpoint: POST /users/verify
//
// Request payload:
type verifyEmailPayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
	Email     string `validate:"nonzero"`
}

// VerifyEmail verifies that the user's email is really hers. It is secured
// with a signature.
func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload verifyEmailPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, err := models.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.WriteJSON(w, errMsgMagic, http.StatusBadRequest)
		return
	}

	femail := strings.ToLower(payload.Email)
	salt := femail + strconv.FormatBool(u.HasEmail(femail))

	if !magic.Verify(
		payload.UserID,
		payload.Timestamp,
		salt,
		payload.Signature,
	) {
		bjson.WriteJSON(w, errMsgMagic, http.StatusUnauthorized)
		return
	}

	// If the link is more than 30 min old, don't trust it.
	ts, err := magic.GetTimeFromB64(payload.Timestamp)
	if err != nil {
		bjson.WriteJSON(w, errMsgMagic, http.StatusBadRequest)
		return
	}
	diff := time.Now().Sub(ts)
	if diff.Hours() > float64(24) {
		bjson.WriteJSON(w, errMsgMagic, http.StatusBadRequest)
		return
	}

	// If there is already an account associated with this email, merge the two accounts.
	dupUser, found, err := models.GetUserByEmail(ctx, femail)
	if found {
		err := u.MergeWith(ctx, &dupUser)
		if err != nil {
			bjson.HandleInternalServerError(w, err, map[string]string{
				"message": "Could not merge accounts",
			})
			return
		}
	}

	u.AddEmail(femail)
	u.DeriveProperties()

	if u.Email == femail {
		u.IsLocked = false
	}

	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// ForgotPassword Endpoint: POST /users/forgot
//
// Request payload:
type forgotPasswordPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// ForgotPassword sends a set password email to the user whose email
// matches the email given in the payload. If no matching user is found
// this does nothing.
func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload forgotPasswordPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	u, found, err := models.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGet)
		return
	} else if found {
		u.SendPasswordResetEmail()
	}

	bjson.WriteJSON(w, map[string]string{
		"message": "Check your email for a link to reset your password",
	}, http.StatusOK)
}

// SendVerifyEmail Endpoint: POST /users/resend

// SendVerifyEmail resends the email verification email.
func SendVerifyEmail(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	err := u.SendVerifyEmail(u.Email)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSend)
		return
	}
	bjson.WriteJSON(w, u, http.StatusOK)
}

// AddEmail Endpoint: POST /users/emails
//
// Request payload:
type addEmailPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// AddEmail sends a verification email to the given email with a magic link that,
// when clicked, adds the new email to the user's account.
func AddEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	var payload addEmailPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	foundUser, found, err := models.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSend)
		return
	}

	if found && foundUser.IsRegistered() {
		// There's already an account associated with the email that the user
		// is attempting to add. Send an email to this address with some details
		// about what's going on.
		if err := u.SendMergeAccountsEmail(payload.Email); err != nil {
			bjson.HandleInternalServerError(w, err, errMsgSend)
			return
		}
	} else {
		if err := u.SendVerifyEmail(payload.Email); err != nil {
			bjson.HandleInternalServerError(w, err, errMsgSend)
			return
		}
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// RemoveEmail Endpoint: POST /users/emails
//
// Request payload:
type removeEmailPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// RemoveEmail removes the given email from the user's account.
func RemoveEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	var payload removeEmailPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.RemoveEmail(payload.Email); err != nil {
		bjson.WriteJSON(w, map[string]string{
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// MakeEmailPrimary Endpoint: POST /users/emails
//
// Request payload:
type makePrimaryEmailPayload struct {
	Email string `validate:"nonzero,regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]+$"`
}

// MakeEmailPrimary removes the given email from the user's account.
func MakeEmailPrimary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	var payload makePrimaryEmailPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.MakeEmailPrimary(payload.Email); err != nil {
		bjson.WriteJSON(w, map[string]string{
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// UserSearch Endpoint: GET /users/search?query={query}

// UserSearch returns search results
func UserSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("query")
	if query == "" {
		bjson.WriteJSON(w, map[string]string{"message": "query cannot be empty"}, http.StatusBadRequest)
		return
	}

	contacts, err := models.UserSearch(ctx, query)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGetUsers)
		return
	}

	bjson.WriteJSON(w, map[string]interface{}{"users": contacts}, http.StatusOK)
}

// PutAvatar Endpoint: POST /users/avatar
//
// Request payload:
type putAvatarPayload struct {
	Blob string `validate:"nonzero"`
	X    float64
	Y    float64
	Size float64
}

// PutAvatar sets the user's avatar to the one given
func PutAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	var payload putAvatarPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	avatarURL, err := storage.PutAvatarFromBlob(
		ctx,
		payload.Blob,
		int(payload.Size),
		int(payload.X),
		int(payload.Y),
		storage.GetKeyFromAvatarURL(u.Avatar))
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgUpload)
		return
	}

	u.Avatar = avatarURL
	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}
