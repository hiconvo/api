package event

import (
	"html"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/magic"
	notif "github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/places"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

type Config struct {
	UserStore     model.UserStore
	EventStore    model.EventStore
	MessageStore  model.MessageStore
	TxnMiddleware mux.MiddlewareFunc
	Mail          *mail.Client
	Magic         magic.Client
	Storage       *storage.Client
	OG            opengraph.Client
	Notif         notif.Client
	Places        places.Client
	Queue         queue.Client
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	s := r.NewRoute().Subrouter()
	s.Use(middleware.WithUser(c.UserStore))
	s.HandleFunc("/events", c.CreateEvent).Methods("POST")
	s.HandleFunc("/events", c.GetEvents).Methods("GET")

	t := r.NewRoute().Subrouter()
	t.Use(middleware.WithUser(c.UserStore), middleware.WithEvent(c.EventStore))
	t.HandleFunc("/events/{eventID}", c.GetEvent).Methods("GET")
	t.HandleFunc("/events/{eventID}", c.DeleteEvent).Methods("DELETE")
	t.HandleFunc("/events/{eventID}/messages", c.GetMessagesByEvent).Methods("GET")
	t.HandleFunc("/events/{eventID}/messages", c.AddMessageToEvent).Methods("POST")
	t.HandleFunc("/events/{eventID}/messages/{messageID}", c.DeleteEventMessage).Methods("DELETE")
	t.HandleFunc("/events/{eventID}/reads", c.MarkEventAsRead).Methods("POST")
	t.HandleFunc("/events/{eventID}/magic", c.GetMagicLink).Methods("GET")

	u := r.NewRoute().Subrouter()
	u.Use(c.TxnMiddleware, middleware.WithUser(c.UserStore), middleware.WithEvent(c.EventStore))
	u.HandleFunc("/events/{eventID}", c.UpdateEvent).Methods("PATCH")
	u.HandleFunc("/events/{eventID}/users/{userID}", c.AddUserToEvent).Methods("POST")
	u.HandleFunc("/events/{eventID}/users/{userID}", c.RemoveUserFromEvent).Methods("DELETE")
	u.HandleFunc("/events/{eventID}/rsvps", c.AddRSVPToEvent).Methods("POST")
	u.HandleFunc("/events/{eventID}/rsvps", c.RemoveRSVPFromEvent).Methods("DELETE")
	u.HandleFunc("/events/{eventID}/magic", c.MagicInvite).Methods("POST")
	u.HandleFunc("/events/{eventID}/magic", c.RollMagicLink).Methods("DELETE")

	v := r.NewRoute().Subrouter()
	v.Use(c.TxnMiddleware)
	v.HandleFunc("/events/rsvps", c.MagicRSVP).Methods("POST")

	return r
}

type createEventPayload struct {
	Name            string `validate:"max=255,nonzero"`
	PlaceID         string `validate:"max=255,nonzero"`
	Timestamp       string `validate:"max=255,nonzero"`
	Description     string `validate:"max=4097,nonzero"`
	Hosts           []*model.UserInput
	Users           []*model.UserInput
	GuestsCanInvite bool
	UTCOffset       int `json:"utcOffset"`
}

// CreateEvent creates a event.
func (c *Config) CreateEvent(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.CreateEvent")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	if !u.IsRegistered() {
		bjson.HandleError(w, errors.E(op, map[string]string{
			"message": "You must verify your account before you can create events",
		}, http.StatusBadRequest))

		return
	}

	var payload createEventPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if len(payload.Users) > model.MaxEventMembers {
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

	place, err := c.Places.Resolve(ctx, payload.PlaceID, payload.UTCOffset)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	users, err := c.UserStore.GetOrCreateUsers(ctx, payload.Users)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	hosts, err := c.UserStore.GetOrCreateUsers(ctx, payload.Hosts)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	event, err := model.NewEvent(
		html.UnescapeString(payload.Name),
		html.UnescapeString(payload.Description),
		place.PlaceID,
		place.Address,
		place.Lat,
		place.Lng,
		timestamp,
		place.UTCOffset,
		u,
		hosts,
		users,
		payload.GuestsCanInvite)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.EventStore.Commit(ctx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := event.SendInvitesAsync(ctx, c.Queue); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, event, http.StatusCreated)
}

func (c *Config) GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	p := model.GetPagination(r)

	events, err := c.EventStore.GetEventsByUser(ctx, u, p)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*model.Event{"events": events}, http.StatusOK)
}

// GetEvent gets a event.
func (c *Config) GetEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if event.OwnerIs(u) || event.HasUser(u) {
		bjson.WriteJSON(w, event, http.StatusOK)
		return
	}

	// Otherwise throw a 404.
	bjson.HandleError(w, errors.E(errors.Op("handlers.GetEvent"), http.StatusNotFound))
}

type deleteEventPayload struct {
	Message string `validate:"max=255"`
}

// DeleteEvent allows the owner to delete the event.
func (c *Config) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	var payload deleteEventPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// If the requestor is not the owner, throw an error
	if !event.OwnerIs(u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.DeleteEvent"),
			errors.Str("non-owner trying to delete event"),
			http.StatusNotFound))

		return
	}

	if err := c.EventStore.Delete(ctx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if event.IsInFuture() {
		if err := c.Mail.SendCancellation(c.Magic, event, html.UnescapeString(payload.Message)); err != nil {
			bjson.HandleError(w, err)
			return
		}

		if err := c.Notif.Put(&notif.Notification{
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

// GetMessagesByEvent gets the messages from the given thread.
func (c *Config) GetMessagesByEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !(event.OwnerIs(u) || event.HasUser(u)) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.GetMessagesByEvent"),
			errors.Str("no permission"),
			http.StatusNotFound))
		return
	}

	messages, err := c.MessageStore.GetMessagesByEvent(ctx, event)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*model.Message{"messages": messages}, http.StatusOK)
}

type createMessagePayload struct {
	Body string `validate:"nonzero"`
	Blob string
}

// AddMessageToEvent adds a message to the given thread.
func (c *Config) AddMessageToEvent(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.AddMessageToEvent")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	// Validate raw data
	var payload createMessagePayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	// Check permissions
	if !(event.OwnerIs(u) || event.HasUser(u)) {
		bjson.HandleError(w, errors.E(op,
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	var (
		photoURL string
		err      error
	)

	if payload.Blob != "" {
		photoURL, err = c.Storage.PutPhotoFromBlob(ctx, event.ID, payload.Blob)
		if err != nil {
			bjson.HandleError(w, errors.E(op, err))
			return
		}
	}

	messageBody := html.UnescapeString(payload.Body)
	link := c.OG.Extract(ctx, messageBody)

	message, err := model.NewEventMessage(
		u,
		event,
		messageBody,
		photoURL,
		link)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.MessageStore.Commit(ctx, message); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.EventStore.Commit(ctx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.Notif.Put(&notif.Notification{
		UserKeys:   notif.FilterKey(event.UserKeys, u.Key),
		Actor:      u.FullName,
		Verb:       notif.NewMessage,
		Target:     notif.Event,
		TargetID:   event.ID,
		TargetName: event.Name,
	}); err != nil {
		// Log the error but don't fail the request
		log.Alarm(err)
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}

func (c *Config) DeleteEventMessage(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.DeleteEventMessage")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	vars := mux.Vars(r)
	id := vars["messageID"]

	m, err := c.MessageStore.GetMessageByID(ctx, id)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err, http.StatusNotFound))
		return
	}

	if !event.Key.Equal(m.ParentKey) {
		bjson.HandleError(w, errors.E(op,
			errors.Str("message not in event"),
			http.StatusNotFound))

		return
	}

	if !(m.OwnerIs(u)) {
		bjson.HandleError(w, errors.E(op, errors.Str("no permission"), http.StatusNotFound))
		return
	}

	if err := c.MessageStore.Delete(ctx, m); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	m.User = model.MapUserToUserPartial(u)

	bjson.WriteJSON(w, m, http.StatusOK)
}

func (c *Config) MarkEventAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !(event.OwnerIs(user) || event.HasUser(user)) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.MarkEventAsRead"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	if model.IsRead(event, user.Key) {
		bjson.WriteJSON(w, event, http.StatusOK)
		return
	}

	if err := model.MarkMessagesAsRead(ctx, c.MessageStore, user, event.Key); err != nil {
		bjson.HandleError(w, err)
		return
	}

	model.MarkAsRead(event, user.Key)
	event.UserReads = model.MapReadsToUserPartials(event, event.Users)

	if err := c.EventStore.Commit(ctx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// GetMagicLink gets the magic link for the given event.
func (c *Config) GetMagicLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if event.OwnerIs(u) || event.HostIs(u) {
		bjson.WriteJSON(w,
			map[string]string{"url": event.GetInviteMagicLink(c.Magic)},
			http.StatusOK)

		return
	}

	// Otherwise throw a 404.
	bjson.HandleError(w, errors.E(
		errors.Op("handlers.GetMagicLink"),
		errors.Str("no permission"),
		http.StatusNotFound))
}

type updateEventPayload struct {
	Name            string `validate:"max=255"`
	PlaceID         string `validate:"max=255"`
	Timestamp       string `validate:"max=255"`
	Description     string `validate:"max=4097"`
	Hosts           []*model.UserInput
	GuestsCanInvite bool
	Resend          bool
	UTCOffset       int `json:"utcOffset"`
}

// UpdateEvent allows the owner to change the event name and location.
func (c *Config) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.UpdateEvent")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	tx, _ := middleware.TransactionFromContext(ctx)

	// If the requestor is not the owner, throw an error
	if !event.OwnerIs(u) {
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
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	hosts, err := c.UserStore.GetOrCreateUsers(ctx, payload.Hosts)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if model.IsHostsDifferent(event.HostKeys, hosts) {
		event.HostPartials = model.MapUsersToUserPartials(hosts)
		event.HostKeys = model.MapUsersToKeys(hosts)
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
		place, err := c.Places.Resolve(ctx, payload.PlaceID, payload.UTCOffset)
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

	if _, err := c.EventStore.CommitWithTransaction(tx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if payload.Resend {
		if err := event.SendUpdatedInvitesAsync(ctx, c.Queue); err != nil {
			bjson.HandleError(w, err)
			return
		}
	}

	if err := c.Notif.Put(&notif.Notification{
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

// AddUserToEvent adds a user to the event. Only owners can add participants.
// Email addresses are also supported.
func (c *Config) AddUserToEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	tx, _ := middleware.TransactionFromContext(ctx)
	vars := mux.Vars(r)
	maybeUserID := vars["userID"]

	if !(event.OwnerIs(u) || event.HostIs(u) || (event.GuestsCanInvite && event.HasUser(u))) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.AddUserToEvent"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	// Either get the user if we got an ID or, if we got an email, get or
	// create the user by email.
	var (
		userToBeAdded *model.User
		err           error
	)

	if _, ee := valid.Email(maybeUserID); ee != nil {
		userToBeAdded, err = c.UserStore.GetUserByID(ctx, maybeUserID)
	} else {
		userToBeAdded, _, err = c.UserStore.GetOrCreateUserByEmail(ctx, maybeUserID)
	}

	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := event.AddUser(userToBeAdded); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := c.EventStore.CommitWithTransaction(tx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.Mail.SendEventInvitation(c.Magic, event, userToBeAdded); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// RemoveUserFromEvent removed a user from the event. The owner can remove
// anyone. Participants can remove themselves.
func (c *Config) RemoveUserFromEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, err := c.UserStore.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// If the requestor is the owner or the requestor is the user to be
	// removed, then remove the user.
	if event.OwnerIs(u) || userToBeRemoved.Key.Equal(u.Key) {
		if err := event.RemoveRSVP(userToBeRemoved); err != nil {
			bjson.HandleError(w, err)
			return
		}

		if err := event.RemoveUser(userToBeRemoved); err != nil {
			bjson.HandleError(w, err)
			return
		}
	} else {
		bjson.HandleError(w,
			errors.E(errors.Op("handlers.RemoveUserFromEvent"), http.StatusNotFound))
		return
	}

	// Save the event.
	if _, err := c.EventStore.CommitWithTransaction(tx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, event, http.StatusOK)
}

// AddRSVPToEvent RSVPs a user to the event.
func (c *Config) AddRSVPToEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if !event.HasUser(u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.AddRSVPToEvent"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	if err := event.AddRSVP(u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := c.EventStore.CommitWithTransaction(tx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.Notif.Put(&notif.Notification{
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

// RemoveRSVPFromEvent removed a user from the event. The owner can remove
// anyone. Participants can remove themselves.
func (c *Config) RemoveRSVPFromEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)

	if err := event.RemoveRSVP(u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := c.EventStore.CommitWithTransaction(tx, event); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.Notif.Put(&notif.Notification{
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

type magicInvitePayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	EventID   string `validate:"nonzero"`
}

// MagicInvite adds and rsvps a user to an event.
func (c *Config) MagicInvite(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.MagicInvite")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	e := middleware.EventFromContext(ctx)
	tx, _ := middleware.TransactionFromContext(ctx)

	var payload magicInvitePayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := e.VerifyInviteMagicLink(
		c.Magic,
		payload.Timestamp,
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := e.AddUser(u); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := e.AddRSVP(u); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := c.EventStore.CommitWithTransaction(tx, e); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := c.Notif.Put(&notif.Notification{
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

	bjson.WriteJSON(w, e, http.StatusOK)
}

// RollMagicLink invalidates the current magic link and generates a new one.
func (c *Config) RollMagicLink(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.RollMagicLink")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	event := middleware.EventFromContext(ctx)
	tx, _ := middleware.TransactionFromContext(ctx)

	if !event.OwnerIs(u) {
		bjson.HandleError(w, errors.E(op, errors.Str("no permission"), http.StatusNotFound))
		return
	}

	event.RollToken()

	if _, err := c.EventStore.CommitWithTransaction(tx, event); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, map[string]string{"url": event.GetInviteMagicLink(c.Magic)}, http.StatusOK)
}

type magicRSVPPayload struct {
	Signature string `validate:"nonzero"`
	Timestamp string `validate:"nonzero"`
	UserID    string `validate:"nonzero"`
	EventID   string `validate:"nonzero"`
}

// MagicRSVP rsvps a user without a registered account to an event that
// she has been invited to.
func (c *Config) MagicRSVP(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.MagicRSVP")
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)

	var payload magicRSVPPayload
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
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	e, err := c.EventStore.GetEventByID(ctx, payload.EventID)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := e.VerifyRSVPMagicLink(
		c.Magic,
		payload.UserID,
		payload.Timestamp,
		payload.Signature,
	); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := e.AddRSVP(u); err != nil {
		log.Print(errors.E(op, err))
		// Just return the user and be done with it
		bjson.WriteJSON(w, u, http.StatusOK)

		return
	}

	u.Verified = true

	if _, err := c.EventStore.CommitWithTransaction(tx, e); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := c.UserStore.CommitWithTransaction(tx, u); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := c.Notif.Put(&notif.Notification{
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
