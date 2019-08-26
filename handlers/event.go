package handlers

import (
	"net/http"
	"strconv"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/validate"
)

var (
	errMsgCreateEvent = map[string]string{"message": "Could not create event"}
	errMsgSaveEvent   = map[string]string{"message": "Could not save event"}
	errMsgGetEvents   = map[string]string{"message": "Could not get events"}
	errMsgGetEvent    = map[string]string{"message": "Could not get event"}
	errMsgDeleteEvent = map[string]string{"message": "Could not delete event"}
)

// CreateEvent Endpoint: POST /events
//
// Request payload:
type createEventPayload struct {
	Name        string `validate:"max=255"`
	Users       []interface{}
	LocationKey string `validate:"max=255"`
	Location    string `validate:"max=255"`
}

// CreateEvent creates a event
func CreateEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ou := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	// Validate raw data
	var payload createEventPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	if len(payload.Users) > 300 {
		bjson.WriteJSON(w, map[string]string{
			"message": "Events have a maximum of 300 members",
		}, http.StatusBadRequest)
	}

	// Make sure users actually exist and remove both duplicate ids and
	// the owner's id from payload.Users
	//
	// First, decode ids into keys and put them into a userKeys slice.
	// Also, for event members indicated by email, save emails to
	// a slice for handling later.
	var userKeys []*datastore.Key
	var emails []string
	// Create a map to keep track of seen ids in order to avoid duplicates.
	// Add the `ou` to seen so that she won't be added to the users list.
	seen := make(map[string]struct{}, len(payload.Users)+1)
	seenEmails := make(map[string]struct{}, len(payload.Users)+1)
	seen[ou.ID] = struct{}{}
	for _, u := range payload.Users {
		// Make sure that the payload is of the expected type.
		//
		// First, check that the user key points to an array of maps.
		umap, uOK := u.(map[string]interface{})
		if !uOK {
			bjson.WriteJSON(w, map[string]string{
				"users": "Users must be an array of objects",
			}, http.StatusBadRequest)
			return
		}
		// Second, check that the `id` or `email` key points to a string.
		id, idOK := umap["id"].(string)
		email, emailOK := umap["email"].(string)
		if !idOK && !emailOK {
			bjson.WriteJSON(w, map[string]string{
				"users": "User ID or email must be a string",
			}, http.StatusBadRequest)
			return
		}

		// If we recived an email, save it to the emails slice if we haven't
		// seen it before and keep going.
		if emailOK {
			if _, seenOK := seenEmails[email]; !seenOK {
				seen[email] = struct{}{}
				emails = append(emails, email)
			}
			continue
		}

		// Make sure we haven't seen this id before.
		if _, seenOK := seen[id]; seenOK {
			continue
		}
		seen[id] = struct{}{}

		// Decode the key and add to the slice.
		key, kErr := datastore.DecodeKey(id)
		if kErr != nil {
			bjson.WriteJSON(w, map[string]string{
				"users": "Invalid users",
			}, http.StatusBadRequest)
			return
		}
		userKeys = append(userKeys, key)
	}
	// Now, get the user objects and save to a new slice of user structs.
	// If this fails, then the input was not valid.
	userStructs := make([]models.User, len(userKeys))
	if uErr := db.Client.GetMulti(ctx, userKeys, userStructs); uErr != nil {
		bjson.WriteJSON(w, map[string]string{
			"users": "Invalid users",
		}, http.StatusBadRequest)
		return
	}

	// Handle members indicated by email.
	for i := range emails {
		u, created, err := models.GetOrCreateUserByEmail(ctx, emails[i])
		if err != nil {
			bjson.HandleInternalServerError(w, err, errMsgCreateEvent)
			return
		}
		if created {
			err = u.Commit(ctx)
			if err != nil {
				bjson.HandleInternalServerError(w, err, errMsgCreateEvent)
				return
			}
		}

		userStructs = append(userStructs, u)
		userKeys = append(userKeys, u.Key)
	}

	// Create the event object.
	//
	// Create another slice of pointers to the user structs to satisfy the
	// event functions below.
	userPointers := make([]*models.User, len(userStructs))
	for i := range userStructs {
		userPointers[i] = &userStructs[i]
	}
	// With userPointers in hand, we can now create the event object. We set
	// the original requestor `ou` as the owner.
	event, tErr := models.NewEvent(payload.Name, payload.LocationKey, payload.Location, &ou, userPointers)
	if tErr != nil {
		bjson.HandleInternalServerError(w, tErr, errMsgCreateEvent)
		return
	}

	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	// Save the event to the corresponding users.
	//
	// Add the event key to the userStructs.
	for _, u := range userPointers {
		u.AddEvent(&event)
	}
	// We can use userPointers here because they point to the user structs
	// which we just modified.
	if _, err := db.Client.PutMulti(ctx, userKeys, userPointers); err != nil {
		// This error would be very bad. It would mean our data is
		// inconsistent.
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusCreated)
}

// GetEvents Endpoint: GET /events

// GetEvents gets the user's events
func GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	// TODO: Paginate
	events, err := models.GetEventsByUser(ctx, &u)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGetEvents)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.Event{"events": events}, http.StatusOK)
}

// GetEvent Endpoint: GET /events/{id}

// GetEvent gets a event
func GetEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if event.OwnerIs(&u) || event.HasUser(&u) {
		bjson.WriteJSON(w, event, http.StatusOK)
		return
	}

	// Otherwise throw a 404.
	bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
}

// UpdateEvent Endpoint: PATCH /events/{id}
//
// Request payload:
type updateEventPayload struct {
	Name string `validate:"max=255,nonzero"`
}

// TODO: LOCATION
// UpdateEvent allows the owner to change the event name and location
func UpdateEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !event.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	var payload updateEventPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	event.Name = payload.Name

	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// DeleteEvent Endpoint: DELETE /events/{id}

// DeleteEvent allows the owner to delete the event
func DeleteEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !event.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	// Get the users
	users := make([]models.User, len(event.UserKeys))
	if err := db.Client.GetMulti(ctx, event.UserKeys, users); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	// Remove the event from all of the event's users
	for i := range users {
		users[i].RemoveEvent(&event)
	}

	// Save the updated users
	if _, err := db.Client.PutMulti(ctx, event.UserKeys, users); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	if err := event.Delete(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// AddUserToEvent Endpoint: POST /events/{eventID}/users/{userID}

// AddUserToEvent adds a user to the event. Only owners can add participants.
func AddUserToEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	// If the requestor is not the owner, throw an error.
	if !event.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	userToBeAdded, uErr := models.GetUserByID(ctx, userID)
	if uErr != nil {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	if err := event.AddUser(&userToBeAdded); err != nil {
		bjson.WriteJSON(w, map[string]string{
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	userToBeAdded.AddEvent(&event)

	// Save the user.
	if err := userToBeAdded.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	// Save the event.
	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// RemoveUserFromEvent Endpoint: DELETE /events/{eventID}/users/{userID}

// RemoveUserFromEvent removed a user from the event. The owner can remove
// anyone. Participants can remove themselves.
func RemoveUserFromEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, uErr := models.GetUserByID(ctx, userID)
	if uErr != nil {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	// If the requestor is the owner or the requestor is the user to be
	// removed, then remove the user.
	if event.HasUser(&userToBeRemoved) && (event.OwnerIs(&u) || userToBeRemoved.Key.Equal(u.Key)) {
		// The owner cannot remove herself
		if userToBeRemoved.Key.Equal(event.OwnerKey) {
			bjson.WriteJSON(w, map[string]string{
				"message": "The event owner cannot be removed from the event",
			}, http.StatusBadRequest)
			return
		}

		event.RemoveUser(&userToBeRemoved)
		userToBeRemoved.RemoveEvent(&event)
	} else {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	// Save the user.
	if err := userToBeRemoved.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	// Save the event.
	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// AddRSVPToEvent Endpoint: POST /events/{eventID}/rsvps

// AddRSVPToEvent RSVPs a user to the event.
func AddRSVPToEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !event.HasUser(&u) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	if err := event.AddRSVP(&u); err != nil {
		bjson.WriteJSON(w, map[string]string{
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	// Save the event.
	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// RemoveRSVPFromEvent Endpoint: DELETE /events/{eventID}/rsvps

// RemoveRSVPFromEvent removed a user from the event. The owner can remove
// anyone. Participants can remove themselves.
func RemoveRSVPFromEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !event.HasUser(&u) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	// The owner cannot remove herself
	if event.OwnerIs(&u) {
		bjson.WriteJSON(w, map[string]string{
			"message": "The event owner cannot be removed from the event",
		}, http.StatusBadRequest)
		return
	}

	event.RemoveRSVP(&u)

	// Save the event.
	if err := event.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// MagicRSVP Endpoint: POST /events/rsvp
//
// Request payload:
type magicRSVPPayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
	EventID   string `validate:"nonzero"`
}

// MagicRSVP rsvps a user without a registered account
func MagicRSVP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)

	var payload magicRSVPPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.WriteJSON(w, err.ToMapString(), http.StatusBadRequest)
		return
	}

	u, err := models.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.WriteJSON(w, errMsgMagic, http.StatusBadRequest)
		return
	}

	e, err := models.GetEventByID(ctx, payload.EventID)
	if err != nil {
		bjson.WriteJSON(w, errMsgMagic, http.StatusBadRequest)
		return
	}

	if !e.HasUser(&u) {
		bjson.WriteJSON(w, errMsgMagic, http.StatusUnauthorized)
		return
	}

	if !magic.Verify(
		payload.UserID,
		payload.Timestamp,
		strconv.FormatBool(e.HasRSVP(&u)),
		payload.Signature,
	) {
		bjson.WriteJSON(w, errMsgMagic, http.StatusUnauthorized)
		return
	}

	if err := e.AddRSVP(&u); err != nil {
		bjson.WriteJSON(w, map[string]string{
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	u.Verified = true

	if err := e.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSave)
		return
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}
