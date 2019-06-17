package handlers

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"

	"github.com/getsentry/raven-go"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/pluck"
	"github.com/hiconvo/api/utils/validate"
)

type inboundMessagePayload struct {
	Body string `validate:"nonzero"`
}

func Inbound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if parseErr := r.ParseMultipartForm(10485760); parseErr != nil {
		handleClientErrorResponse(w, parseErr)
		return
	}

	encodedEnvelope := r.PostFormValue("envelope")
	to, from, envErr := pluck.AddressesFromEnvelope(encodedEnvelope)
	if envErr != nil {
		handleClientErrorResponse(w, envErr)
		return
	}

	// Get thread id from address
	threadID, plErr := pluck.ThreadInt64IDFromAddress(to)
	if plErr != nil {
		handleClientErrorResponse(w, plErr)
		return
	}

	// Get the thread
	thread, thErr := models.GetThreadByInt64ID(ctx, threadID)
	if thErr != nil {
		handleClientErrorResponse(w, thErr)
		return
	}

	// Get user from from address
	user, found, uErr := models.GetUserByEmail(ctx, from)
	if !found || uErr != nil {
		handleClientErrorResponse(w, uErr)
		return
	}

	// Verify that the user is a particiapant of the thread
	if !(thread.OwnerIs(&user) || thread.HasUser(&user)) {
		handleClientErrorResponse(w, errors.New("Permission denied"))
		return
	}

	// Pluck the new message
	htmlMessage := html.UnescapeString(r.FormValue("html"))
	textMessage := r.FormValue("text")
	messageText, messErr := pluck.MessageText(htmlMessage, textMessage, from, to)
	if messErr != nil {
		handleClientErrorResponse(w, messErr)
		return
	}

	// Validate and sanitize
	var payload inboundMessagePayload
	if valErr := validate.Do(&payload, map[string]interface{}{
		"body": messageText,
	}); valErr != nil {
		handleClientErrorResponse(w, valErr)
		return
	}

	// Create the new message
	message, mErr := models.NewMessage(&user, &thread, html.UnescapeString(payload.Body))
	if mErr != nil {
		handleServerErrorResponse(w, mErr)
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

	if senErr := thread.Send(ctx); senErr != nil {
		handleServerErrorResponse(w, senErr)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("PASS: message %s created", message.ID)))
}

func handleClientErrorResponse(w http.ResponseWriter, err error) {
	raven.CaptureError(err, map[string]string{"inbound": "ignored"})
	fmt.Fprintf(os.Stdout, "Ignoring inbound: "+err.Error())
	w.WriteHeader(http.StatusOK)
}

func handleServerErrorResponse(w http.ResponseWriter, err error) {
	raven.CaptureError(err, map[string]string{"inbound": "failure"})
	fmt.Fprintln(os.Stderr, err.Error())
	w.WriteHeader(http.StatusInternalServerError)
}
