package handlers

import (
	"net/http"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
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

	// Validate raw data
	var payload createThreadPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	if len(payload.Users) > 20 {
		bjson.WriteJSON(w, map[string]string{
			"message": "Convos have a maximum of 20 members",
		}, http.StatusBadRequest)
		return
	}

	userStructs, userKeys, emails, err := extractUsers(ctx, ou, payload.Users)
	if err != nil {
		bjson.WriteJSON(w, map[string]string{
			"users": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	newUsers, newUserKeys, err := createUsersByEmail(ctx, emails)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreateEvent)
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
	thread, tErr := models.NewThread(payload.Subject, &ou, userPointers)
	if tErr != nil {
		bjson.HandleInternalServerError(w, tErr, errMsgCreateThread)
		return
	}

	if err := thread.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	// Save the thread to the corresponding users.
	//
	// Add the thread key to the userStructs.
	for _, u := range userPointers {
		u.AddThread(&thread)
	}
	// We can use userPointers here because they point to the user structs
	// which we just modified.
	if _, err := db.Client.PutMulti(ctx, userKeys, userPointers); err != nil {
		// This error would be very bad. It would mean our data is
		// inconsistent.
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

	// TODO: Paginate
	threads, err := models.GetThreadsByUser(ctx, &u)
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
	Subject string `validate:"max=255,nonzero"`
}

// UpdateThread allows the owner to change the thread subject
func UpdateThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	thread.Subject = payload.Subject

	if err := thread.Commit(ctx); err != nil {
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

	// Remove the thread from all of the thread's users
	for i := range thread.Users {
		thread.Users[i].RemoveThread(&thread)
	}

	userKeys := make([]*datastore.Key, len(thread.Users))
	for i := range thread.Users {
		userKeys[i] = thread.Users[i].Key
	}

	// Save the updated users
	if _, err := db.Client.PutMulti(ctx, userKeys, thread.Users); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
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
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	// If the requestor is not the owner, throw an error.
	if !thread.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	userToBeAdded, uErr := models.GetUserByID(ctx, userID)
	if uErr != nil {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	if err := thread.AddUser(&userToBeAdded); err != nil {
		bjson.WriteJSON(w, map[string]string{
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	userToBeAdded.AddThread(&thread)

	// Save the user.
	if err := userToBeAdded.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	// Save the thread.
	if err := thread.Commit(ctx); err != nil {
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
			bjson.WriteJSON(w, map[string]string{
				"message": "The convo owner cannot be removed from the convo",
			}, http.StatusBadRequest)
			return
		}

		thread.RemoveUser(&userToBeRemoved)
		userToBeRemoved.RemoveThread(&thread)
	} else {
		bjson.WriteJSON(w, errMsgGetThread, http.StatusNotFound)
		return
	}

	// Save the user.
	if err := userToBeRemoved.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	// Save the thread.
	if err := thread.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveThread)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}
