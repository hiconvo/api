package handlers

import (
	"html"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
	"github.com/hiconvo/api/utils/validate"
)

var (
	errMsgGetUsers     = map[string]string{"users": "Not all users are valid"}
	errMsgCreateThread = map[string]string{"message": "Could not create thread"}
	errMsgSaveThread   = map[string]string{"message": "Could not save thread"}
	errMsgGetThreads   = map[string]string{"message": "Could not get threads"}
	errMsgGetThread    = map[string]string{"message": "Could not get thread"}
	errMsgDeleteThread = map[string]string{"message": "Could not delete thread"}
)

// CreateThread Endpoint: POST /threads
//
// Request payload:
type createThreadPayload struct {
	Subject string `validate:"max=255"`
	Users   []interface{}
}

// CreateThread creates a thread
func CreateThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ou := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	if !ou.IsRegistered() {
		bjson.WriteJSON(w, map[string]string{
			"message": "You must verify your account before you can create Convos",
		}, http.StatusBadRequest)
		return
	}

	// Validate raw data
	var payload createThreadPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if len(payload.Users) > 11 {
		bjson.WriteJSON(w, map[string]string{
			"message": "Convos have a maximum of 11 members",
		}, http.StatusBadRequest)
		return
	}

	userStructs, userKeys, emails, err := extractUsers(ctx, ou, payload.Users)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	newUsers, newUserKeys, err := createUsersByEmail(ctx, emails)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreateThread)
		return
	}

	userStructs = append(userStructs, newUsers...)
	userKeys = append(userKeys, newUserKeys...)

	// Create the thread object.
	//
	// Create another slice of pointers to the user structs to satisfy the
	// thread functions below.
	userPointers := make([]*models.User, len(userStructs))
	for i := range userStructs {
		userPointers[i] = &userStructs[i]
	}
	// With userPointers in hand, we can now create the thread object. We set
	// the original requestor `ou` as the owner.
	thread, tErr := models.NewThread(html.UnescapeString(payload.Subject), &ou, userPointers)
	if tErr != nil {
		bjson.HandleInternalServerError(w, tErr, errMsgCreateThread)
		return
	}

	if err := thread.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusCreated)
}

// GetThreads Endpoint: GET /threads

// GetThreads gets the user's threads
func GetThreads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	p := getPagination(r)

	threads, err := models.GetThreadsByUser(ctx, &u, p)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGetThreads)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.Thread{"threads": threads}, http.StatusOK)
}

// GetThread Endpoint: GET /threads/{id}

// GetThread gets a thread
func GetThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if thread.OwnerIs(&u) || thread.HasUser(&u) {
		bjson.WriteJSON(w, thread, http.StatusOK)
		return
	}

	// Otherwise throw a 404.
	bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
}

// UpdateThread Endpoint: PATCH /threads/{id}
//
// Request payload:
type updateThreadPayload struct {
	Subject string `validate:"nonzero,max=255"`
}

// UpdateThread allows the owner to change the thread subject
func UpdateThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !thread.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	var payload updateThreadPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	thread.Subject = html.UnescapeString(payload.Subject)

	if _, err := thread.CommitWithTransaction(tx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

// DeleteThread Endpoint: DELETE /threads/{id}

// DeleteThread allows the owner to delete the thread
func DeleteThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !thread.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	if err := thread.Delete(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

// AddUserToThread Endpoint: POST /threads/{threadID}/users/{userID}

// AddUserToThread adds a user to the thread. Only owners can add participants.
func AddUserToThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	vars := mux.Vars(r)
	maybeUserID := vars["userID"]

	// If the requestor is not the owner, throw an error.
	if !thread.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	// Either get the user if we got an ID or, if we got an email, get or
	// create the user by email.
	var userToBeAdded models.User
	var err error
	if isEmail(maybeUserID) {
		userToBeAdded, err = createUserByEmail(ctx, maybeUserID)
	} else {
		userToBeAdded, err = models.GetUserByID(ctx, maybeUserID)
	}
	if err != nil {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	if err := thread.AddUser(&userToBeAdded); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Save the thread.
	if _, err := thread.CommitWithTransaction(tx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

// RemoveUserFromThread Endpoint: DELETE /threads/{threadID}/users/{userID}

// RemoveUserFromThread removed a user from the thread. The owner can remove
// anyone. Participants can remove themselves.
func RemoveUserFromThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, uErr := models.GetUserByID(ctx, userID)
	if uErr != nil {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	// If the requestor is the owner or the requestor is the user to be
	// removed, then remove the user.
	if thread.HasUser(&userToBeRemoved) && (thread.OwnerIs(&u) || userToBeRemoved.Key.Equal(u.Key)) {
		// The owner cannot remove herself
		if userToBeRemoved.Key.Equal(thread.OwnerKey) {
			bjson.HandleError(w, errors.E(
				errors.Op("handlers.RemoveUserFromThread"),
				map[string]string{"message": "The Convo owner cannot be removed from the convo"},
				http.StatusBadRequest,
			))
			return
		}

		thread.RemoveUser(&userToBeRemoved)
	} else {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	// Save the thread.
	if _, err := thread.CommitWithTransaction(tx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}
