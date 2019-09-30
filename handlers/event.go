package handlers

import (
	"html"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/places"
	"github.com/hiconvo/api/utils/validate"
)

var (
	errMsgCreateEvent = map[string]string{"message": "Could not create event"}
	errMsgSaveEvent   = map[string]string{"message": "Could not save event"}
	errMsgGetEvents   = map[string]string{"message": "Could not get events"}
	errMsgGetEvent    = map[string]string{"message": "Could not get event"}
	errMsgDeleteEvent = map[string]string{"message": "Could not delete event"}
	errMsgSendEvent   = map[string]string{"message": "Could not send event invitations"}
)

// CreateEvent Endpoint: POST /events
//
// Request payload:
type createEventPayload struct {
	Name        string `validate:"max=255,nonzero"`
	PlaceID     string `validate:"max=255,nonzero"`
	Timestamp   string `validate:"max=255,nonzero"`
	Description string `validate:"max=1023,nonzero"`
	Users       []interface{}
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
		return
	}

	timestamp, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		bjson.WriteJSON(w, map[string]string{
			"time": "Invalid time",
		}, http.StatusBadRequest)
		return
	}

	place, err := places.Resolve(ctx, payload.PlaceID)
	if err != nil {
		bjson.WriteJSON(w, map[string]string{
			"placeID": err.Error(),
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
	event, err := models.NewEvent(
		payload.Name,
		html.UnescapeString(payload.Description),
		place.PlaceID,
		place.Address,
		place.Lat,
		place.Lng,
		timestamp,
		place.UTCOffset,
		&ou,
		userPointers)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgCreateEvent)
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

	if err := event.SendInvites(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSendEvent)
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
	Name        string `validate:"max=255"`
	PlaceID     string `validate:"max=255"`
	Timestamp   string `validate:"max=255"`
	Description string `validate:"max=1023"`
}

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

	// TODO: Come up with something better than this.
	if payload.Name != "" && payload.Name != event.Name {
		event.Name = payload.Name
	}

	if payload.Description != "" && payload.Description != event.Description {
		event.Description = html.UnescapeString(payload.Description)
	}

	if payload.Timestamp != "" {
		timestamp, err := time.Parse(time.RFC3339, payload.Timestamp)
		if err != nil {
			bjson.WriteJSON(w, map[string]string{
				"time": "Invalid time",
			}, http.StatusBadRequest)
			return
		}

		if !timestamp.Equal(event.Timestamp) {
			event.Timestamp = timestamp
		}
	}

	if payload.PlaceID != "" && payload.PlaceID != event.PlaceID {
		place, err := places.Resolve(ctx, payload.PlaceID)
		if err != nil {
			bjson.WriteJSON(w, map[string]string{
				"placeID": err.Error(),
			}, http.StatusBadRequest)
			return
		}

		event.PlaceID = place.PlaceID
		event.Address = place.Address
		event.Lat = place.Lat
		event.Lng = place.Lng
		event.UTCOffset = place.UTCOffset
	}

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
