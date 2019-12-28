package handlers

import (
	"errors"
	"fmt"
	"html"
	"net/http"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/mail"
	og "github.com/hiconvo/api/utils/opengraph"
	"github.com/hiconvo/api/utils/pluck"
	"github.com/hiconvo/api/utils/reporter"
	"github.com/hiconvo/api/utils/validate"
)

type inboundMessagePayload struct {
	Body string `validate:"nonzero"`
}

func Inbound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(10485760); err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	encodedEnvelope := r.PostFormValue("envelope")
	to, from, err := pluck.AddressesFromEnvelope(encodedEnvelope)
	if err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	// Get thread id from address
	threadID, err := pluck.ThreadInt64IDFromAddress(to)
	if err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	// Get the thread
	thread, err := models.GetThreadByInt64ID(ctx, threadID)
	if err != nil {
		sendTryAgainEmail(from)
		handleClientErrorResponse(w, err)
		return
	}

	// Get user from from address
	user, found, err := models.GetUserByEmail(ctx, from)
	if !found {
		sendErrorEmail(from)
		handleClientErrorResponse(w, errors.New("Email not recognized"))
		return
	} else if err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	// Verify that the user is a particiapant of the thread
	if !(thread.OwnerIs(&user) || thread.HasUser(&user)) {
		sendErrorEmail(user.Email)
		handleClientErrorResponse(w, errors.New("Permission denied"))
		return
	}

	// Pluck the new message
	htmlMessage := html.UnescapeString(r.FormValue("html"))
	textMessage := r.FormValue("text")
	messageText, err := pluck.MessageText(htmlMessage, textMessage, from, to)
	if err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	// Validate and sanitize
	var payload inboundMessagePayload
	if err := validate.Do(&payload, map[string]interface{}{
		"body": messageText,
	}); err != nil {
		handleClientErrorResponse(w, err)
		return
	}

	messageBody := html.UnescapeString(payload.Body)
	link := og.Extract(ctx, messageBody)

	// Create the new message
	message, err := models.NewThreadMessage(&user, &thread, messageBody, "", link)
	if err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	if err := message.Commit(ctx); err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	if err := thread.Commit(ctx); err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	if err := thread.Send(ctx); err != nil {
		handleServerErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("PASS: message %s created", message.ID)))
}

func handleClientErrorResponse(w http.ResponseWriter, err error) {
	reporter.Report(fmt.Errorf("Inbound: ClientError: %v", err))
	w.WriteHeader(http.StatusOK)
}

func handleServerErrorResponse(w http.ResponseWriter, err error) {
	reporter.Report(fmt.Errorf("Inbound: ServerError: %v", err))
	w.WriteHeader(http.StatusInternalServerError)
}

func sendErrorEmail(email string) {
	err := mail.Send(mail.EmailMessage{
		FromName:    "Convo",
		FromEmail:   "support@mail.hiconvo.com",
		ToName:      "",
		ToEmail:     email,
		Subject:     "[convo] Send Failure",
		HTMLContent: "<p>Hello,</p><p>You responded to a Convo from an unrecognized email address. Please try again and make sure that you use the exact email to which the Convo was addressed.</p><p>Thanks,<br />Convo Support</p>",
		TextContent: "Hello,\n\nYou responded to a Convo from an unrecognized email address. Please try again and make sure that you use the exact email to which the Convo was addressed.\n\nThanks,\nConvo Support",
	})

	if err != nil {
		reporter.Report(fmt.Errorf("handlers.sendErrorEmail: %v", err))
	}
}

func sendTryAgainEmail(email string) {
	err := mail.Send(mail.EmailMessage{
		FromName:    "Convo",
		FromEmail:   "support@mail.hiconvo.com",
		ToName:      "",
		ToEmail:     email,
		Subject:     "[convo] Send Failure",
		HTMLContent: "<p>Hello,</p><p>You responded to a Convo that does not accept email responses. If you're attempting to respond to an event invitation, click RSVP in the invitation email and post your message to the message board. You won't have to create an account.</p><p>Thanks,<br />Convo Support</p>",
		TextContent: "Hello,\n\nYou responded to a Convo that does not accept email responses. If you're attempting to respond to an event invitation, click RSVP in the invitation email and post your message to the message board. You won't have to create an account.\n\nThanks,\nConvo Support",
	})

	if err != nil {
		reporter.Report(fmt.Errorf("handlers.sendTryAgainEmail: %v", err))
	}
}
