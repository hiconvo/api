package handlers

import (
	"html"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	notif "github.com/hiconvo/api/notifications"
	"github.com/hiconvo/api/utils/bjson"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/places"
	"github.com/hiconvo/api/utils/validate"
)

// CreateEvent Endpoint: POST /events
//
// Request payload:
type createEventPayload struct {
	Name            string `validate:"max=255,nonzero"`
	PlaceID         string `validate:"max=255,nonzero"`
	Timestamp       string `validate:"max=255,nonzero"`
	Description     string `validate:"max=4097,nonzero"`
	Hosts           []interface{}
	Users           []interface{}
	GuestsCanInvite bool
}

// CreateEvent creates a event
func CreateEvent(w http.ResponseWriter, r *http.Request) {
	var op errors.Op = "handlers.CreateEvent"

	ctx := r.Context()
	ou := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	if !ou.IsRegistered() {
		bjson.HandleError(w, errors.E(op, map[string]string{
			"message": "You must verify your account before you can create events",
		}, http.StatusBadRequest))
		return
	}

	// Validate raw data
	var payload createEventPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if len(payload.Users) > 300 {
		bjson.HandleError(w, errors.E(op, map[string]string{
			"message": "Events have a maximum of 300 members",
		}, http.StatusBadRequest))
		return
	}

	timestamp, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		bjson.HandleError(w, errors.E(op, map[string]string{
			"time": "Invalid time",
		}, http.StatusBadRequest))
		return
	}

	place, err := places.Resolve(ctx, payload.PlaceID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Handle users
	users, err := extractAndCreateUsers(ctx, ou, payload.Users)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Same thing for hosts
	hosts, err := extractAndCreateUsers(ctx, ou, payload.Hosts)
	if err != nil {
		bjson.HandleError(w, err)
		return
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
		hosts,
		users,
		payload.GuestsCanInvite)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := event.Commit(ctx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := event.SendInvitesAsync(ctx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, event, http.StatusCreated)
}

// GetEvents Endpoint: GET /events

// GetEvents gets the user's events
func GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	p := getPagination(r)

	events, err := models.GetEventsByUser(ctx, &u, p)
	if err != nil {
		bjson.HandleError(w, err)
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
	bjson.HandleError(w, errors.E(errors.Op("handlers.GetEvent"), http.StatusNotFound))
}

// UpdateEvent Endpoint: PATCH /events/{id}
//
// Request payload:
type updateEventPayload struct {
	Name            string `validate:"max=255"`
	PlaceID         string `validate:"max=255"`
	Timestamp       string `validate:"max=255"`
	Description     string `validate:"max=4097"`
	Hosts           []interface{}
	GuestsCanInvite bool
	Resend          bool
}

// UpdateEvent allows the owner to change the event name and location
func UpdateEvent(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.UpdateEvent")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	body := bjson.BodyFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	tx, _ := db.TransactionFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !event.OwnerIs(&u) {
		bjson.HandleError(w, errors.E(op, http.StatusNotFound))
		return
	}

	// Only allow updating future events
	if !event.IsInFuture() {
		bjson.HandleError(w, errors.E(op,
			map[string]string{"message": "You cannot update past events"},
			http.StatusBadRequest))
		return
	}

	var payload updateEventPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	hosts, err := extractAndCreateUsers(ctx, u, payload.Hosts)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if isHostsDifferent(event.HostKeys, hosts) {
		event.HostPartials = models.MapUsersToUserPartials(hosts)
		event.HostKeys = mapUsersToKeyPointers(hosts)
	}

	if payload.Name != "" && payload.Name != event.Name {
		event.Name = html.UnescapeString(payload.Name)
	}

	if payload.GuestsCanInvite != event.GuestsCanInvite {
		event.GuestsCanInvite = payload.GuestsCanInvite
	}

	if payload.Description != "" && payload.Description != event.Description {
		event.Description = html.UnescapeString(payload.Description)
	}

	if payload.Timestamp != "" {
		timestamp, err := time.Parse(time.RFC3339, payload.Timestamp)
		if err != nil {
			bjson.HandleError(w, errors.E(op,
				map[string]string{"time": "Invalid time"},
				http.StatusBadRequest))
			return
		}

		if !timestamp.Equal(event.Timestamp) {
			event.Timestamp = timestamp
		}
	}

	if payload.PlaceID != "" && payload.PlaceID != event.PlaceID {
		place, err := places.Resolve(ctx, payload.PlaceID)
		if err != nil {
			bjson.HandleError(w, err)
			return
		}

		event.PlaceID = place.PlaceID
		event.Address = place.Address
		event.Lat = place.Lat
		event.Lng = place.Lng
		event.UTCOffset = place.UTCOffset
	}

	if _, err := event.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if payload.Resend {
		if err := event.SendUpdatedInvitesAsync(ctx); err != nil {
			bjson.HandleError(w, err)
			return
		}
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
		log.Alarm(err)
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// DeleteEvent Endpoint: DELETE /events/{id}
//
// Request payload:
type deleteEventPayload struct {
	Message string `validate:"max=255"`
}

// DeleteEvent allows the owner to delete the event
func DeleteEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	body := bjson.BodyFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !event.OwnerIs(&u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.DeleteEvent"),
			errors.Str("non-owner trying to delete event"),
			http.StatusNotFound))
		return
	}

	var payload deleteEventPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := event.Delete(ctx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if event.IsInFuture() {
		if err := event.SendCancellation(ctx, html.UnescapeString(payload.Message)); err != nil {
			bjson.HandleError(w, err)
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
			log.Alarm(err)
		}
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
	tx, _ := db.TransactionFromContext(ctx)
	vars := mux.Vars(r)
	maybeUserID := vars["userID"]

	if !(event.OwnerIs(&u) || event.HostIs(&u) || (event.GuestsCanInvite && event.HasUser(&u))) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.AddUserToEvent"),
			errors.Str("no permission"),
			http.StatusNotFound))
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
		bjson.HandleError(w, err)
		return
	}

	if err := event.AddUser(&userToBeAdded); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Save the event.
	if _, err := event.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
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
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, err := models.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// If the requestor is the owner or the requestor is the user to be
	// removed, then remove the user.
	if event.OwnerIs(&u) || userToBeRemoved.Key.Equal(u.Key) {
		if err := event.RemoveUser(&userToBeRemoved); err != nil {
			bjson.HandleError(w, err)
			return
		}
	} else {
		bjson.HandleError(w, errors.E(errors.Op("handlers.RemoveUserFromEvent"), http.StatusNotFound))
		return
	}

	// Save the event.
	if _, err := event.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// AddRSVPToEvent Endpoint: POST /events/{eventID}/rsvps

// AddRSVPToEvent RSVPs a user to the event.
func AddRSVPToEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !event.HasUser(&u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.AddRSVPToEvent"),
			errors.Str("no permission"),
			http.StatusNotFound))
		return
	}

	if err := event.AddRSVP(&u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Save the event.
	if _, err := event.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
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
		log.Alarm(err)
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// RemoveRSVPFromEvent Endpoint: DELETE /events/{eventID}/rsvps

// RemoveRSVPFromEvent removed a user from the event. The owner can remove
// anyone. Participants can remove themselves.
func RemoveRSVPFromEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := db.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if err := event.RemoveRSVP(&u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Save the event.
	if _, err := event.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
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
		log.Alarm(err)
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
	op := errors.Op("handlers.MagicRSVP")
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)
	tx, _ := db.TransactionFromContext(ctx)

	var payload magicRSVPPayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, errors.E(op, err, http.StatusNotFound))
		return
	}

	u, err := models.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	e, err := models.GetEventByID(ctx, payload.EventID)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := magic.Verify(
		payload.UserID,
		payload.Timestamp,
		strconv.FormatBool(!e.IsInFuture()),
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := e.AddRSVP(&u); err != nil {
		log.Print(errors.E(op, err))
		// Just return the user and be done with it
		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	u.Verified = true

	if _, err := e.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := u.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
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
		log.Alarm(err)
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// MagicInvite Endpoint: POST /events/rsvp
//
// Request payload:
type magicInvitePayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
	EventID   string `validate:"nonzero"`
}

// MagicInvite rsvps a user without a registered account
func MagicInvite(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.MagicInvite")
	ctx := r.Context()
	body := bjson.BodyFromContext(ctx)
	tx, _ := db.TransactionFromContext(ctx)

	var payload magicInvitePayload
	if err := validate.Do(&payload, body); err != nil {
		bjson.HandleError(w, errors.E(op, err, http.StatusNotFound))
		return
	}

	u, err := models.GetUserByID(ctx, payload.UserID)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	e, err := models.GetEventByID(ctx, payload.EventID)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := magic.Verify(
		payload.EventID,
		payload.Timestamp,
		e.Token,
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := e.AddRSVP(&u); err != nil {
		log.Print(errors.E(op, err))
		// Just return the user and be done with it
		bjson.WriteJSON(w, u, http.StatusOK)
		return
	}

	if _, err := e.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
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
		log.Alarm(err)
	}

	bjson.WriteJSON(w, u, http.StatusOK)
}

// GetMagicLink Endpoint: GET /events/{eventID}/magic

// GetMagicLink gets the magic link for the given event.
func GetMagicLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if event.OwnerIs(&u) || event.HostIs(&u) {
		bjson.WriteJSON(w, map[string]string{"url": event.GetMagicLink()}, http.StatusOK)
		return
	}

	// Otherwise throw a 404.
	bjson.HandleError(w, errors.E(
		errors.Op("handlers.GetMagicLink"),
		errors.Str("no permission"),
		http.StatusNotFound))
}

// RollMagicLink Endpoint: PUT /events/{eventID}/magic

// RollMagicLink invalidates the current magic link and generates a new one.
func RollMagicLink(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.RollMagicLink")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	tx, _ := db.TransactionFromContext(ctx)

	if !event.OwnerIs(&u) {
		bjson.HandleError(w, errors.E(op, errors.Str("no permission"), http.StatusNotFound))
		return
	}

	event.RollToken()

	if _, err := event.CommitWithTransaction(tx); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, map[string]string{"url": event.GetMagicLink()}, http.StatusOK)
}
