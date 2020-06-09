package inbound

import (
	"fmt"
	"html"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/pluck"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

type Config struct {
	Pluck        pluck.Client
	UserStore    model.UserStore
	ThreadStore  model.ThreadStore
	MessageStore model.MessageStore
	Magic        magic.Client
	Mail         *mail.Client
	OG           opengraph.Client
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/inbound", c.Inbound).Methods("POST")

	return r
}

type inboundMessagePayload struct {
	Body string `validate:"nonzero"`
}

func (c *Config) Inbound(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handler.Inbound")
	ctx := r.Context()

	if err := r.ParseMultipartForm(10485760); err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	encodedEnvelope := r.PostFormValue("envelope")
	to, from, err := c.Pluck.AddressesFromEnvelope(encodedEnvelope)

	if err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	// Get thread id from address
	threadID, err := c.Pluck.ThreadInt64IDFromAddress(to)
	if err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	// Get the thread
	thread, err := c.ThreadStore.GetThreadByInt64ID(ctx, threadID)
	if err != nil {
		if err := c.Mail.SendInboundTryAgainEmail(from); err != nil {
			log.Alarm(errors.E(op, err))
		}

		handleClientErrorResponse(w, err)

		return
	}

	// Get user from from address
	user, found, err := c.UserStore.GetUserByEmail(ctx, from)
	if !found {
		if err := c.Mail.SendInboundErrorEmail(from); err != nil {
			log.Alarm(errors.E(op, err))
		}

		handleClientErrorResponse(w, errors.E(op, errors.Str("Email not recognized")))

		return
	} else if err != nil {
		handleClientErrorResponse(w, errors.E(op, err))
		return
	}

	// Verify that the user is a particiapant of the thread
	if !(thread.OwnerIs(user) || thread.HasUser(user)) {
		if err := c.Mail.SendInboundErrorEmail(from); err != nil {
			log.Alarm(errors.E(op, err))
		}

		handleClientErrorResponse(w, errors.E(op, errors.Str("permission denied")))

		return
	}

	// Pluck the new message
	htmlMessage := html.UnescapeString(r.FormValue("html"))
	textMessage := r.FormValue("text")

	messageText, err := c.Pluck.MessageText(htmlMessage, textMessage, from, to)
	if err != nil {
		handleClientErrorResponse(w, errors.E(op, err))
		return
	}

	// Validate and sanitize
	var payload = inboundMessagePayload{Body: messageText}
	if err := valid.Raw(&payload); err != nil {
		handleClientErrorResponse(w, errors.E(op, err))
		return
	}

	messageBody := html.UnescapeString(payload.Body)
	link := c.OG.Extract(ctx, messageBody)

	// Create the new message
	message, err := model.NewThreadMessage(user, thread, messageBody, "", link)
	if err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	if err := c.MessageStore.Commit(ctx, message); err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	if err := c.ThreadStore.Commit(ctx, thread); err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	messages, err := c.MessageStore.GetMessagesByThread(ctx, thread)
	if err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	if err := c.Mail.SendThread(c.Magic, thread, messages); err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("PASS: message %s created", message.ID)))
}

func handleClientErrorResponse(w http.ResponseWriter, err error) {
	log.Alarm(errors.E(errors.Op("inboundClientError"), err))
	w.WriteHeader(http.StatusOK)
}

func handleServerErrorResponse(w http.ResponseWriter, err error) {
	log.Alarm(errors.E(errors.Op("inboundInternalError"), err))
	w.WriteHeader(http.StatusInternalServerError)
}
