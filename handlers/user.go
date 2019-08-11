package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	uuid "github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"gocloud.dev/blob"

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
	errMsgUpload = map[string]string{"message": "Could not upload avatar"}
)

// CreateUser Endpoint: POST /users
//
// Request payload:
type createUserPayload struct {
	Email     string `validate:"regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2\\,4}$"`
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
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
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

	user.SendVerifyEmail()

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
	Email    string `validate:"regexp=^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2\\,4}$"`
	Password string `validate:"nonzero"`
}

// AuthenticateUser is an endpoint that authenticates a user with a password.
func AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload authenticateUserPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
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
			u.SendVerifyEmail()

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

// OAuth is an endpoint that can do three things. It can
//   1. create a user with oauth based authentication
//   2. associate an existing user with an oauth token
//   3. authenticate a user via oauth
//
func OAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload oauth.UserPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	oauthPayload, oerr := oauth.Verify(ctx, payload)
	if oerr != nil {
		bjson.WriteJSON(w, errMsgCreds, http.StatusBadRequest)
		return
	}

	// Get the user and return if found
	u, found, err := models.GetUserByOAuthID(ctx, oauthPayload.ID, oauthPayload.Provider)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGet)
		return
	} else if found {
		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	// Try to find the user by email. If found, associate the new token
	// with the existing user.
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

		if err := u.Commit(ctx); err != nil {
			bjson.HandleInternalServerError(w, err, errMsgSave)
			return
		}

		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	// Finally at new user case

	avatarURI, err := oauth.CacheAvatar(ctx, oauthPayload.TempAvatar)
	if err != nil {
		// Print error but keep going. User might not have a profile pic.
		fmt.Fprintln(os.Stderr, err.Error())
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

	// Save the user
	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// UpdateUser Endpoint: PATCH /users
//
// Request payload:
type updateUserPayload struct {
	Email     string `validate:"regexp=^([a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2\\,4})?$"`
	FirstName string
	LastName  string
	Password  bool
}

// UpdateUser is an endpoint that can do three things. It can
//   - update FirstName and LastName fields on a user
//   - initiate an email update which requires email based validation
//   - initiate a password update which requires email based validation
//
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	var payload updateUserPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	if payload.Password {
		u.SendPasswordResetEmail()
	}

	if payload.Email != "" && payload.Email != u.Email {
		// Make sure the user is not already registered
		_, found, err := models.GetUserByEmail(ctx, payload.Email)
		if err != nil {
			bjson.HandleInternalServerError(w, err, errMsgCreate)
			return
		} else if found {
			bjson.WriteJSON(w, errMsgReg, http.StatusBadRequest)
			return
		}

		// Update email and send verification email
		u.Email = payload.Email
		u.Verified = false
		u.SendVerifyEmail()
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
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
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
}

// VerifyEmail verifies that the user's email is really hers. It is secured
// with a signature.
func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload verifyEmailPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
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
		strconv.FormatBool(u.Verified),
		payload.Signature,
	) {
		bjson.WriteJSON(w, errMsgMagic, http.StatusUnauthorized)
		return
	}

	u.Verified = true
	u.IsLocked = false
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
	Email string `validate:"regexp=^([a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2\\,4})?$"`
}

// ForgotPassword sends a set password email to the user whose email
// matches the email given in the payload. If no matching user is found
// this does nothing.
func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload forgotPasswordPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
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
	err := u.SendVerifyEmail()
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSend)
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
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	bucket, err := storage.GetAvatarBucket(ctx)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgUpload)
		return
	}
	defer bucket.Close()

	key := uuid.Must(uuid.NewV4()).String() + ".jpg"

	outputBlob, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
		CacheControl: "525600",
	})
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgUpload)
		return
	}
	defer outputBlob.Close()

	inputBlob := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload.Blob))
	var stderr bytes.Buffer

	cropGeo := fmt.Sprintf("%vx%v+%v+%v", int(payload.Size), int(payload.Size), int(payload.X), int(payload.Y))
	cmd := exec.Command("convert", "-", "-crop", cropGeo, "-adaptive-resize", "256x256", "jpeg:-")
	cmd.Stdin = inputBlob
	cmd.Stdout = outputBlob
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Println(stderr.String())
		bjson.HandleInternalServerError(w, err, errMsgUpload)
		return
	}

	// Delete existing avatar if there is one
	oldKey := storage.GetKeyFromAvatarURL(u.Avatar)
	exists, err := bucket.Exists(ctx, oldKey)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgUpload)
		return
	}
	if exists {
		bucket.Delete(ctx, oldKey)
	}

	u.Avatar = storage.GetFullAvatarURL(key)
	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}
