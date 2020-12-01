package thread

import (
	"html"
	"net/http"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/magic"
	notif "github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/opengraph"
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
	ThreadStore   model.ThreadStore
	MessageStore  model.MessageStore
	TxnMiddleware mux.MiddlewareFunc
	Mail          *mail.Client
	Magic         magic.Client
	Storage       *storage.Client
	OG            opengraph.Client
	Notif         notif.Client
	Queue         queue.Client
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.Use(middleware.WithUser(c.UserStore))
	r.HandleFunc("/threads", c.CreateThread).Methods("POST")
	r.HandleFunc("/threads", c.GetThreads).Methods("GET")

	s := r.NewRoute().Subrouter()
	s.Use(middleware.WithThread(c.ThreadStore))
	s.HandleFunc("/threads/{threadID}", c.GetThread).Methods("GET")
	s.HandleFunc("/threads/{threadID}", c.DeleteThread).Methods("DELETE")
	s.HandleFunc("/threads/{threadID}/messages", c.GetMessagesByThread).Methods("GET")

	t := r.NewRoute().Subrouter()
	t.Use(c.TxnMiddleware, middleware.WithThread(c.ThreadStore))
	t.HandleFunc("/threads/{threadID}/reads", c.MarkThreadAsRead).Methods("POST")
	t.HandleFunc("/threads/{threadID}", c.UpdateThread).Methods("PATCH")
	t.HandleFunc("/threads/{threadID}/users/{userID}", c.AddUserToThread).Methods("POST")
	t.HandleFunc("/threads/{threadID}/users/{userID}", c.RemoveUserFromThread).Methods("DELETE")
	t.HandleFunc("/threads/{threadID}/messages", c.AddMessageToThread).Methods("POST")
	t.HandleFunc("/threads/{threadID}/messages/{messageID}", c.DeleteThreadMessage).Methods("DELETE")

	return r
}

type createThreadPayload struct {
	Subject string `validate:"max=255"`
	Users   []*model.UserInput
	Body    string `validate:"nonzero"`
	Blob    string
}

// CreateThread creates a thread.
func (c *Config) CreateThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	if !u.IsRegistered() {
		bjson.WriteJSON(w, map[string]string{
			"message": "You must verify your account before you can create Convos",
		}, http.StatusBadRequest)

		return
	}

	var payload createThreadPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	users, err := c.UserStore.GetOrCreateUsers(ctx, payload.Users)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	thread, err := model.NewThread(
		ctx,
		c.ThreadStore,
		c.Storage,
		c.OG,
		&model.NewThreadInput{
			Owner:   u,
			Users:   users,
			Subject: html.UnescapeString(payload.Subject),
			Body:    html.UnescapeString(payload.Body),
			Blob:    payload.Blob,
		})
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.ThreadStore.Commit(ctx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if thread.IsSendable() {
		if err := c.Queue.PutEmail(ctx, queue.EmailPayload{
			IDs:    []string{thread.ID},
			Type:   queue.Thread,
			Action: queue.SendThread,
		}); err != nil {
			log.Alarm(err)
		}
	}

	bjson.WriteJSON(w, thread, http.StatusCreated)
}

// GetThreads gets the user's threads.
func (c *Config) GetThreads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	p := model.GetPagination(r)

	threads, err := c.ThreadStore.GetThreadsByUser(ctx, u, p)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*model.Thread{"threads": threads}, http.StatusOK)
}

// GetThread gets a thread.
func (c *Config) GetThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if thread.OwnerIs(u) || thread.HasUser(u) {
		bjson.WriteJSON(w, thread, http.StatusOK)
		return
	}

	// Otherwise throw a 404.
	bjson.HandleError(w, errors.E(
		errors.Op("handlers.GetThread"),
		errors.Str("no permission"),
		http.StatusNotFound))
}

// DeleteThread allows the owner to delete the thread.
func (c *Config) DeleteThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	// If the requester is not the owner, throw an error
	if !thread.OwnerIs(u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.DeleteThread"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	// TODO: Delete messages too

	if err := c.ThreadStore.Delete(ctx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

// GetMessagesByThread gets the messages from the given thread.
func (c *Config) GetMessagesByThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if !(thread.OwnerIs(u) || thread.HasUser(u)) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.GetMessagesByThread"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	// TODO: Pagination

	messages, err := c.MessageStore.GetMessagesByThread(ctx, thread)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string][]*model.Message{"messages": messages}, http.StatusOK)
}

func (c *Config) MarkThreadAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	user := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	if !(thread.OwnerIs(user) || thread.HasUser(user)) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.MarkThreadAsRead"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	if model.IsRead(thread, user.Key) {
		bjson.WriteJSON(w, thread, http.StatusOK)
		return
	}

	if err := model.MarkMessagesAsRead(ctx, c.MessageStore, user, thread.Key); err != nil {
		bjson.HandleError(w, err)
		return
	}

	model.MarkAsRead(thread, user.Key)
	thread.UserReads = model.MapReadsToUserPartials(thread, thread.Users)

	if _, err := c.ThreadStore.CommitWithTransaction(tx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

type updateThreadPayload struct {
	Subject string `validate:"nonzero,max=255"`
}

// UpdateThread allows the owner to change the thread subject.
func (c *Config) UpdateThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	// If the requester is not the owner, throw an error
	if !thread.OwnerIs(u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.UpdateThread"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	var payload updateThreadPayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	thread.Subject = html.UnescapeString(payload.Subject)

	if _, err := c.ThreadStore.CommitWithTransaction(tx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

// AddUserToThread adds a user to the thread. Only owners can add participants.
func (c *Config) AddUserToThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	vars := mux.Vars(r)
	maybeUserID := vars["userID"]

	// If the requester is not the owner, throw an error.
	if !thread.OwnerIs(u) {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.AddUserToThread"),
			errors.Str("no permission"),
			http.StatusNotFound))

		return
	}

	// Either get the user if we got an ID or, if we got an email, get or
	// create the user by email.
	var (
		userToBeAdded *model.User
		err           error
		created       bool
	)

	if _, ee := valid.Email(maybeUserID); ee != nil {
		userToBeAdded, err = c.UserStore.GetUserByID(ctx, maybeUserID)
	} else {
		userToBeAdded, created, err = c.UserStore.GetOrCreateUserByEmail(ctx, maybeUserID)
	}

	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := thread.AddUser(userToBeAdded); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := c.ThreadStore.CommitWithTransaction(tx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if created {
		err := c.Queue.PutEmail(ctx, queue.EmailPayload{
			IDs:    []string{thread.ID},
			Type:   queue.Thread,
			Action: queue.SendThread,
		})
		if err != nil {
			log.Alarm(err)
		}
	}

	if err := c.Notif.Put(&notif.Notification{
		UserKeys:   []*datastore.Key{userToBeAdded.Key},
		Actor:      u.FullName,
		Verb:       notif.NewMessage,
		Target:     notif.Thread,
		TargetID:   thread.ID,
		TargetName: thread.Subject,
	}); err != nil {
		// Log the error but don't fail the request
		log.Alarm(err)
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

// RemoveUserFromThread removed a user from the thread. The owner can remove
// anyone. Participants can remove themselves.
func (c *Config) RemoveUserFromThread(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.RemoveUserFromThread")
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, err := c.UserStore.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	// If the requestor is the owner or the requestor is the user to be
	// removed, then remove the user.
	if thread.HasUser(userToBeRemoved) && (thread.OwnerIs(u) || userToBeRemoved.Key.Equal(u.Key)) {
		// The owner cannot remove herself
		if userToBeRemoved.Key.Equal(thread.OwnerKey) {
			bjson.HandleError(w, errors.E(op,
				map[string]string{"message": "The Convo owner cannot be removed from the convo"},
				http.StatusBadRequest,
			))

			return
		}

		thread.RemoveUser(userToBeRemoved)
	} else {
		bjson.HandleError(w, errors.E(op,
			errors.Str("no permission"),
			http.StatusNotFound))
		return
	}

	// Save the thread.
	if _, err := c.ThreadStore.CommitWithTransaction(tx, thread); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, thread, http.StatusOK)
}

type createMessagePayload struct {
	Body string `validate:"nonzero"`
	Blob string
}

// AddMessageToThread adds a message to the given thread.
func (c *Config) AddMessageToThread(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.AddMessageToThread")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	tx, _ := middleware.TransactionFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)

	// Check permissions
	if !(thread.OwnerIs(u) || thread.HasUser(u)) {
		bjson.HandleError(w, errors.E(op, http.StatusNotFound, errors.Str("no permission")))
		return
	}

	var payload createMessagePayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, err)
		return
	}

	message, err := model.NewMessage(
		ctx,
		c.Storage,
		c.OG,
		&model.NewMessageInput{
			User:   u,
			Parent: thread.Key,
			Body:   html.UnescapeString(payload.Body),
			Blob:   payload.Blob,
		},
	)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := thread.IncRespCount(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	model.ClearReads(thread)
	model.MarkAsRead(thread, u.Key)

	if err := c.MessageStore.Commit(ctx, message); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := c.ThreadStore.CommitWithTransaction(tx, thread); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := c.Notif.Put(&notif.Notification{
		UserKeys:   notif.FilterKey(thread.UserKeys, u.Key),
		Actor:      u.FullName,
		Verb:       notif.NewMessage,
		Target:     notif.Thread,
		TargetID:   thread.ID,
		TargetName: thread.Subject,
	}); err != nil {
		// Log the error but don't fail the request
		log.Alarm(err)
	}

	bjson.WriteJSON(w, message, http.StatusCreated)
}

// DeleteThreadMessage deletes a thread message.
func (c *Config) DeleteThreadMessage(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.DeleteThreadMessage")
	ctx := r.Context()
	tx, _ := middleware.TransactionFromContext(ctx)
	u := middleware.UserFromContext(ctx)
	thread := middleware.ThreadFromContext(ctx)
	vars := mux.Vars(r)
	id := vars["messageID"]

	message, err := c.MessageStore.GetMessageByID(ctx, id)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if !(message.OwnerIs(u)) {
		bjson.HandleError(w, errors.E(op, errors.Str("no permission"), http.StatusNotFound))
		return
	}

	// GetMessageByID doesn't get the associated user. Since we confirmed that u is the owner,
	// we assign u to the User field of the message.
	message.User = model.MapUserToUserPartial(u)

	if !message.ParentKey.Equal(thread.Key) {
		bjson.HandleError(w, errors.E(
			op, errors.Str("message not a child of thread"), http.StatusNotFound))
		return
	}

	if err := c.MessageStore.Delete(ctx, message); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	thread.ResponseCount--

	if _, err := c.ThreadStore.CommitWithTransaction(tx, thread); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if _, err := tx.Commit(); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, message, http.StatusOK)
}
