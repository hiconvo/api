package handler

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	notif "github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/oauth"
	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/places"
	"github.com/hiconvo/api/clients/pluck"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/handler/contact"
	"github.com/hiconvo/api/handler/event"
	"github.com/hiconvo/api/handler/inbound"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/handler/note"
	"github.com/hiconvo/api/handler/task"
	"github.com/hiconvo/api/handler/thread"
	"github.com/hiconvo/api/handler/user"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
)

type Config struct {
	Transacter    db.Transacter
	UserStore     model.UserStore
	ThreadStore   model.ThreadStore
	EventStore    model.EventStore
	MessageStore  model.MessageStore
	NoteStore     model.NoteStore
	Welcome       model.Welcomer
	TxnMiddleware mux.MiddlewareFunc
	Mail          *mail.Client
	Magic         magic.Client
	OAuth         oauth.Client
	Storage       *storage.Client
	Notif         notif.Client
	OG            opengraph.Client
	Places        places.Client
	Queue         queue.Client
}

func New(c *Config) http.Handler {
	router := mux.NewRouter()

	router.NotFoundHandler = http.HandlerFunc(notFound)

	s := router.NewRoute().Subrouter()
	s.PathPrefix("/inbound").Handler(inbound.NewHandler(&inbound.Config{
		Pluck:        pluck.NewClient(),
		UserStore:    c.UserStore,
		ThreadStore:  c.ThreadStore,
		MessageStore: c.MessageStore,
		Mail:         c.Mail,
		Magic:        c.Magic,
		OG:           c.OG,
		Storage:      c.Storage,
	}))
	s.PathPrefix("/tasks").Handler(task.NewHandler(&task.Config{
		UserStore:    c.UserStore,
		ThreadStore:  c.ThreadStore,
		EventStore:   c.EventStore,
		MessageStore: c.MessageStore,
		Welcome:      c.Welcome,
		Mail:         c.Mail,
		Magic:        c.Magic,
		Storage:      c.Storage,
	}))

	t := router.NewRoute().Subrouter()
	t.Use(middleware.WithJSONRequests)

	t.PathPrefix("/users").Handler(user.NewHandler(&user.Config{
		Transacter:   c.Transacter,
		UserStore:    c.UserStore,
		ThreadStore:  c.ThreadStore,
		EventStore:   c.EventStore,
		MessageStore: c.MessageStore,
		Mail:         c.Mail,
		Magic:        c.Magic,
		OA:           c.OAuth,
		Storage:      c.Storage,
		Welcome:      c.Welcome,
	}))
	t.PathPrefix("/contacts").Handler(contact.NewHandler(&contact.Config{
		UserStore: c.UserStore,
	}))
	t.PathPrefix("/threads").Handler(thread.NewHandler(&thread.Config{
		UserStore:     c.UserStore,
		ThreadStore:   c.ThreadStore,
		MessageStore:  c.MessageStore,
		TxnMiddleware: c.TxnMiddleware,
		Mail:          c.Mail,
		Magic:         c.Magic,
		Storage:       c.Storage,
		Notif:         c.Notif,
		OG:            c.OG,
		Queue:         c.Queue,
	}))
	t.PathPrefix("/events").Handler(event.NewHandler(&event.Config{
		UserStore:     c.UserStore,
		EventStore:    c.EventStore,
		MessageStore:  c.MessageStore,
		TxnMiddleware: c.TxnMiddleware,
		Mail:          c.Mail,
		Magic:         c.Magic,
		Storage:       c.Storage,
		Notif:         c.Notif,
		OG:            c.OG,
		Places:        c.Places,
		Queue:         c.Queue,
	}))
	t.PathPrefix("/notes").Handler(note.NewHandler(&note.Config{
		UserStore: c.UserStore,
		NoteStore: c.NoteStore,
		OG:        c.OG,
	}))

	h := middleware.WithCORS(router)
	h = middleware.WithLogging(h)
	h = middleware.WithErrorReporting(h)

	return h
}

func notFound(w http.ResponseWriter, r *http.Request) {
	bjson.WriteJSON(w, map[string]string{"message": "Not found"}, http.StatusNotFound)
}
