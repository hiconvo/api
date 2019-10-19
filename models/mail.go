package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/mail"
	"github.com/hiconvo/api/utils/template"
)

const (
	fromEmail = "robots@mail.hiconvo.com"
	fromName  = "Convo"

	passwordReset      = "Hello,\n\nPlease [click here]( %s ) to set your password.\n\nThanks,<br />ConvoBot"
	verifyEmail        = "Hello,\n\nPlease [click here]( %s ) to verify your email address.\n\nThanks,<br />ConvoBot"
	messageTplStr      = "%s said:\n\n%s\n\n"
	eventTplStr        = "%s invited you to:\n\n%s\n\n%s\n\n%s\n\n%s\n"
	cancellationTplStr = "%s has cancelled:\n\n%s\n\n%s\n\n%s\n"
)

func sendPasswordResetEmail(u *User, magicLink string) error {
	return sendAdministrativeEmail(u, passwordReset, "[convo] Set Password", magicLink)
}

func sendVerifyEmail(u *User, magicLink string) error {
	return sendAdministrativeEmail(u, verifyEmail, "[convo] Verify Email", magicLink)
}

func sendAdministrativeEmail(u *User, strTpl, subject, magicLink string) error {
	plainText := fmt.Sprintf(strTpl, magicLink)

	html, err := template.RenderThread(template.Thread{
		Subject: subject,
		Messages: []template.Message{
			template.Message{
				Body: plainText,
				Name: fromName,
			},
		},
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    fromName,
		FromEmail:   fromEmail,
		ToName:      u.FullName,
		ToEmail:     u.Email,
		Subject:     subject,
		TextContent: plainText,
		HTMLContent: html,
	}

	return mail.Send(email)
}

func sendThread(thread *Thread, messages []*Message) error {
	users := thread.Users
	// From is the most recent message sender.
	sender, serr := MapUserPartialToUser(messages[0].User, users)
	if serr != nil {
		return serr
	}

	// Loop through all participants and generate emails
	emailMessages := make([]mail.EmailMessage, len(users))
	for i := range users {
		currentUser := users[i]

		// Don't send an email to the sender.
		if currentUser.Key.Equal(sender.Key) {
			continue
		}

		plainText, html, rerr := renderThread(thread, messages, currentUser)
		if rerr != nil {
			return rerr
		}
		emailMessages[i] = mail.EmailMessage{
			FromName:    sender.FullName,
			FromEmail:   thread.GetEmail(),
			ToName:      currentUser.FullName,
			ToEmail:     currentUser.Email,
			Subject:     thread.Subject,
			TextContent: plainText,
			HTMLContent: html,
		}
	}

	for i := range emailMessages {
		mail.Send(emailMessages[i])
	}

	return nil
}

func sendEvent(event *Event, isUpdate bool) error {
	users := event.Users
	var fmtStr string
	if isUpdate {
		fmtStr = "Updated invitation to %s"
	} else {
		fmtStr = "Invitation to %s"
	}

	// Loop through all participants and generate emails
	emailMessages := make([]mail.EmailMessage, len(users)-1)
	for i := range users {
		currentUser := users[i]

		// Don't send invitations to the host
		if event.OwnerIs(currentUser) {
			continue
		}

		plainText, html, rerr := renderEvent(event, currentUser)
		if rerr != nil {
			return rerr
		}
		emailMessages[i] = mail.EmailMessage{
			FromName:    event.Owner.FullName,
			FromEmail:   event.GetEmail(),
			ToName:      currentUser.FullName,
			ToEmail:     currentUser.Email,
			Subject:     fmt.Sprintf(fmtStr, event.Name),
			TextContent: plainText,
			HTMLContent: html,
		}
	}

	for i := range emailMessages {
		mail.Send(emailMessages[i])
	}

	return nil
}

func sendEventInvitation(event *Event, user *User) error {
	plainText, html, rerr := renderEvent(event, user)
	if rerr != nil {
		return rerr
	}
	email := mail.EmailMessage{
		FromName:    event.Owner.FullName,
		FromEmail:   event.GetEmail(),
		ToName:      user.FullName,
		ToEmail:     user.Email,
		Subject:     fmt.Sprintf("Invitation to %s", event.Name),
		TextContent: plainText,
		HTMLContent: html,
	}

	mail.Send(email)

	return nil
}

func sendCancellation(event *Event) error {
	users := event.Users

	// Loop through all participants and generate emails
	emailMessages := make([]mail.EmailMessage, len(users))
	for i := range users {
		currentUser := users[i]
		plainText, html, rerr := renderCancellation(event, currentUser)
		if rerr != nil {
			return rerr
		}
		emailMessages[i] = mail.EmailMessage{
			FromName:    event.Owner.FullName,
			FromEmail:   event.GetEmail(),
			ToName:      currentUser.FullName,
			ToEmail:     currentUser.Email,
			Subject:     fmt.Sprintf("Cancelled: %s", event.Name),
			TextContent: plainText,
			HTMLContent: html,
		}
	}

	for i := range emailMessages {
		mail.Send(emailMessages[i])
	}

	return nil
}

func sendDigest(digestList []DigestItem, upcomingEvents []*Event, user *User) error {
	_, html, err := renderDigest(digestList, upcomingEvents, user)
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    fromName,
		FromEmail:   fromEmail,
		ToName:      user.FullName,
		ToEmail:     user.Email,
		Subject:     "[convo] Digest",
		TextContent: "You have unread messages on Convo.",
		HTMLContent: html,
	}

	return mail.Send(email)
}

func renderThread(thread *Thread, messages []*Message, user *User) (string, string, error) {
	var lastFive []*Message
	if len(messages) > 5 {
		lastFive = messages[:5]
	} else {
		lastFive = messages
	}

	tplMessages := make([]template.Message, len(lastFive))
	var builder strings.Builder

	for i, m := range lastFive {
		fmt.Fprintf(&builder, messageTplStr, m.User.FirstName, m.Body)
		tplMessages[i] = template.Message{
			Body:   m.Body,
			Name:   m.User.FirstName,
			FromID: m.User.ID,
			ToID:   user.ID,
		}
	}

	plainText := builder.String()

	// Use the first 200 chars as the preview
	var preview string
	if len(plainText) > 200 {
		preview = plainText[:200] + "..."
	} else {
		preview = plainText
	}

	html, err := template.RenderThread(template.Thread{
		Subject:  thread.Subject,
		Messages: tplMessages,
		Preview:  preview,
	})
	if err != nil {
		return "", "", err
	}

	return plainText, html, nil
}

func renderEvent(event *Event, user *User) (string, string, error) {
	var builder strings.Builder
	fmt.Fprintf(&builder, eventTplStr,
		event.Owner.FullName,
		event.Name,
		event.Address,
		event.GetFormatedTime(),
		event.Description)
	plainText := builder.String()

	// Use the first 200 chars as the preview
	var preview string
	if len(plainText) > 200 {
		preview = plainText[:200] + "..."
	} else {
		preview = plainText
	}

	html, err := template.RenderEvent(template.Event{
		Name:        event.Name,
		Address:     event.Address,
		Time:        event.GetFormatedTime(),
		Description: event.Description,
		Preview:     preview,
		FromName:    event.Owner.FullName,
		MagicLink: magic.NewLink(
			user.Key,
			strconv.FormatBool(event.HasRSVP(user)),
			fmt.Sprintf("rsvp/%s",
				event.Key.Encode())),
	})
	if err != nil {
		return "", "", err
	}

	return plainText, html, nil
}

func renderCancellation(event *Event, user *User) (string, string, error) {
	loc := time.FixedZone("Given", event.UTCOffset)
	timestamp := event.Timestamp.In(loc).Format("Monday, January 2nd @ 3:04 PM")

	var builder strings.Builder
	fmt.Fprintf(&builder, cancellationTplStr,
		event.Owner.FullName,
		event.Name,
		event.Address,
		timestamp)
	plainText := builder.String()

	// Use the first 200 chars as the preview
	var preview string
	if len(plainText) > 200 {
		preview = plainText[:200] + "..."
	} else {
		preview = plainText
	}

	html, err := template.RenderCancellation(template.Event{
		Name:     event.Name,
		Address:  event.Address,
		Time:     timestamp,
		Preview:  preview,
		FromName: event.Owner.FullName,
	})
	if err != nil {
		return "", "", err
	}

	return plainText, html, nil
}

func renderDigest(digestList []DigestItem, events []*Event, user *User) (string, string, error) {
	// Convert all the DigestItems into template.Threads with their messages
	items := make([]template.Thread, len(digestList))
	for i := range digestList {
		messages := make([]template.Message, len(digestList[i].Messages))
		for j := range messages {
			messages[j] = template.Message{
				Body:   digestList[i].Messages[j].Body,
				Name:   digestList[i].Messages[j].User.FullName,
				FromID: digestList[i].Messages[j].User.ID,
				ToID:   user.ID,
			}
		}

		items[i] = template.Thread{
			Subject:  digestList[i].Name,
			Messages: messages,
		}
	}

	// Convert all of the upcomingEvents to template.Events
	templateEvents := make([]template.Event, len(events))
	for i := range events {
		templateEvents[i] = template.Event{
			Name:    events[i].Name,
			Address: events[i].Address,
			Time:    events[i].GetFormatedTime(),
		}
	}

	// Render all the stuff
	html, err := template.RenderDigest(template.Digest{
		Items:  items,
		Events: templateEvents,
	})
	if err != nil {
		return "", "", err
	}

	return "", html, nil
}
