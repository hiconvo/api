package task

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/digest"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
)

type Config struct {
	DB           db.Client
	UserStore    model.UserStore
	ThreadStore  model.ThreadStore
	EventStore   model.EventStore
	MessageStore model.MessageStore
	Welcome      model.Welcomer
	Mail         *mail.Client
	Magic        magic.Client
	Storage      *storage.Client
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/tasks/digest", c.CreateDigest)
	r.HandleFunc("/tasks/emails", c.SendEmailsAsync)

	return r
}

func (c *Config) CreateDigest(w http.ResponseWriter, r *http.Request) {
	if val := r.Header.Get("X-Appengine-Cron"); val != "true" {
		bjson.WriteJSON(w, map[string]string{
			"message": "Not found",
		}, http.StatusNotFound)

		return
	}

	d := digest.New(&digest.Config{
		DB:           c.DB,
		UserStore:    c.UserStore,
		EventStore:   c.EventStore,
		ThreadStore:  c.ThreadStore,
		MessageStore: c.MessageStore,
		Magic:        c.Magic,
		Mail:         c.Mail,
	})

	if err := d.Digest(r.Context()); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, map[string]string{"message": "pass"}, http.StatusOK)
}

func (c *Config) SendEmailsAsync(w http.ResponseWriter, r *http.Request) {
	var (
		op      errors.Op = "handlers.SendEmailsAsync"
		ctx               = r.Context()
		payload queue.EmailPayload
	)

	if val := r.Header.Get("X-Appengine-QueueName"); val != "convo-emails" {
		bjson.WriteJSON(w, map[string]string{
			"message": "Not found",
		}, http.StatusNotFound)

		return
	}

	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, err)
		return
	}

	for i := range payload.IDs {
		switch payload.Type {
		case queue.User:
			u, err := c.UserStore.GetUserByID(ctx, payload.IDs[i])
			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}

			if payload.Action == queue.SendWelcome {
				err = c.Welcome.Welcome(ctx, c.ThreadStore, c.Storage, u)
			}

			if err != nil {
				log.Alarm(errors.E(op, err))
			}
		case queue.Event:
			e, err := c.EventStore.GetEventByID(ctx, payload.IDs[i])
			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}

			if payload.Action == queue.SendInvites {
				err = c.Mail.SendEventInvites(c.Magic, e, false)
			} else if payload.Action == queue.SendUpdatedInvites {
				err = c.Mail.SendEventInvites(c.Magic, e, true)
			}

			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}
		case queue.Thread:
			thread, err := c.ThreadStore.GetThreadByID(ctx, payload.IDs[i])
			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}

			messages, err := c.MessageStore.GetMessagesByThread(ctx, thread, &model.Pagination{Size: 5})
			if err != nil {
				log.Alarm(errors.E(op, err))
				break
			}

			if payload.Action == queue.SendThread {
				if err := c.Mail.SendThread(c.Magic, thread, messages); err != nil {
					log.Alarm(errors.E(op, err))
					break
				}

				// SendThread only sends threads to non-registered users. In order not to spam
				// such users with a digest, we mark the thread as read for these users.
				for i := range thread.Users {
					if !thread.Users[i].IsRegistered() {
						model.MarkAsRead(thread, thread.Users[i].Key)
					}
				}

				if err := c.ThreadStore.Commit(ctx, thread); err != nil {
					log.Alarm(errors.E(op, err))
					break
				}
			}
		}
	}

	bjson.WriteJSON(w, map[string]string{"message": "pass"}, http.StatusOK)
}
