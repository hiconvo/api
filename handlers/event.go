package handlers

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	notif "github.com/hiconvo/api/notifications"
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
	errMsgUpdateEvent = map[string]string{"message": "You cannot update past events"}
)

// CreateEvent Endpoint: POST /events
//
// Request payload:
type createEventPayload struct {
	Name        string `validate:"max=255,nonzero"`
	PlaceID     string `validate:"max=255,nonzero"`
	Timestamp   string `validate:"max=255,nonzero"`
	Description string `validate:"max=4097,nonzero"`
	Users       []interface{}
}

// CreateEvent creates a event
func CreateEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ou := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	if !ou.IsRegistered() {
		bjson.WriteJSON(w, map[string]string{
			"message": "You must verify your account before you can create events",
		}, http.StatusBadRequest)
		return
	}

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
		html.UnescapeString(payload.Name),
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

	// Only allow updating future events
	if !event.IsInFuture() {
		bjson.WriteJSON(w, errMsgUpdateEvent, http.StatusBadRequest)
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

	if err := event.SendUpdatedInvites(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSendEvent)
		return
	}

	if err := notif.Put(notif.Notification{
		UserKeys:   notif.FilterKey(event.UserKeys, u.Key),
		Actor:      u.FullName,
		Verb:       notif.UpdateEvent,
		Target:     notif.Event,
		TargetID:   event.ID,
		TargetName: event.Name,
	}); err != nil {
		// Log the error but don't fail the request
		fmt.Fprintln(os.Stderr, err)
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

	if event.IsInFuture() {
		if err := event.SendCancellation(ctx); err != nil {
			bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
			return
		}

		if err := notif.Put(notif.Notification{
			UserKeys:   notif.FilterKey(event.UserKeys, u.Key),
			Actor:      u.FullName,
			Verb:       notif.DeleteEvent,
			Target:     notif.Event,
			TargetID:   event.ID,
			TargetName: event.Name,
		}); err != nil {
			// Log the error but don't fail the request
			fmt.Fprintln(os.Stderr, err)
		}
	}

	if err := event.Delete(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgSaveEvent)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// AddUserToEvent Endpoint: POST /events/{eventID}/users/{userID}

// AddUserToEvent adds a user to the event. Only owners can add participants.
// Email addresses are also supported.
func AddUserToEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	vars := mux.Vars(r)
	maybeUserID := vars["userID"]

	// If the requestor is not the owner, throw an error.
	if !event.OwnerIs(&u) {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
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
		fmt.Println(err.Error())
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
		return
	}

	if err := event.AddUser(&userToBeAdded); err != nil {
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

	event.SendInviteToUser(ctx, &userToBeAdded)

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
	} else {
		bjson.WriteJSON(w, errMsgGetEvent, http.StatusNotFound)
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

	if err := notif.Put(notif.Notification{
		UserKeys:   []*datastore.Key{event.OwnerKey},
		Actor:      u.FullName,
		Verb:       notif.AddRSVP,
		Target:     notif.Event,
		TargetID:   event.ID,
		TargetName: event.Name,
	}); err != nil {
		// Log the error but don't fail the request
		fmt.Fprintln(os.Stderr, err)
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

	if err := notif.Put(notif.Notification{
		UserKeys:   []*datastore.Key{event.OwnerKey},
		Actor:      u.FullName,
		Verb:       notif.RemoveRSVP,
		Target:     notif.Event,
		TargetID:   event.ID,
		TargetName: event.Name,
	}); err != nil {
		// Log the error but don't fail the request
		fmt.Fprintln(os.Stderr, err)
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
		strconv.FormatBool(!e.IsInFuture()),
		payload.Signature,
	) {
		bjson.WriteJSON(w, errMsgMagic, http.StatusUnauthorized)
		return
	}

	if err := e.AddRSVP(&u); err != nil {
		// Just return the user and be done with it
		bjson.WriteJSON(w, u, http.StatusOK)
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

	if err := notif.Put(notif.Notification{
		UserKeys:   []*datastore.Key{e.OwnerKey},
		Actor:      u.FullName,
		Verb:       notif.AddRSVP,
		Target:     notif.Event,
		TargetID:   e.ID,
		TargetName: e.Name,
	}); err != nil {
		// Log the error but don't fail the request
		fmt.Fprintln(os.Stderr, err)
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}
