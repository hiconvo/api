package models

import (
	"fmt"
	"strconv"

	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/mail"
	"github.com/hiconvo/api/utils/template"
)

const (
	_fromEmail = "robots@mail.hiconvo.com"
	_fromName  = "Convo"
)

func sendPasswordResetEmail(u *User, magicLink string) error {
	body := fmt.Sprintf(_tplStrPasswordReset, magicLink)
	return sendAdministrativeEmail(u, u.Email, "[convo] Set Password", body)
}

func sendVerifyEmail(u *User, email, magicLink string) error {
	body := fmt.Sprintf(_tplStrVerifyEmail, magicLink)
	return sendAdministrativeEmail(u, email, "[convo] Verify Email", body)
}

func sendMergeAccountsEmail(u *User, emailToMerge, magicLink string) error {
	body := fmt.Sprintf(_tplStrMergeAccounts, magicLink, emailToMerge, u.Email)
	return sendAdministrativeEmail(u, emailToMerge, "[convo] Verify Email", body)
}

func sendAdministrativeEmail(u *User, toEmail, subject, body string) error {
	plainText, html, err := template.RenderThread(template.Thread{
		Subject: subject,
		Messages: []template.Message{
			template.Message{
				Body: body,
				Name: _fromName,
			},
		},
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    _fromName,
		FromEmail:   _fromEmail,
		ToName:      u.FullName,
		ToEmail:     toEmail,
		Subject:     subject,
		TextContent: plainText,
		HTMLContent: html,
	}

	return mail.Send(email)
}

func sendThread(thread *Thread, messages []*Message) error {
	// From is the most recent message sender: messages[0].User.
	sender, err := MapUserPartialToUser(messages[0].User, thread.Users)
	if err != nil {
		return err
	}

	// Loop through all participants and generate emails.
	//
	emailMessages := make([]mail.EmailMessage, len(thread.Users))
	// Get the last five messages to be included in the email.
	lastFive := getLastFive(messages)
	for i, curUser := range thread.Users {
		// Don't send an email to the sender.
		if curUser.Key.Equal(sender.Key) {
			continue
		}

		// Generate messages
		tplMessages := make([]template.Message, len(lastFive))
		for _, m := range lastFive {
			tplMessages[i] = template.Message{
				Body:   m.Body,
				Name:   m.User.FirstName,
				FromID: m.User.ID,
				ToID:   curUser.ID,
			}
		}

		plainText, html, err := template.RenderThread(template.Thread{
			Subject:  thread.Subject,
			Messages: tplMessages,
		})
		if err != nil {
			return err
		}

		emailMessages[i] = mail.EmailMessage{
			FromName:    sender.FullName,
			FromEmail:   thread.GetEmail(),
			ToName:      curUser.FullName,
			ToEmail:     curUser.Email,
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
	var fmtStr string
	if isUpdate {
		fmtStr = "Updated invitation to %s"
	} else {
		fmtStr = "Invitation to %s"
	}

	// Loop through all participants and generate emails
	emailMessages := make([]mail.EmailMessage, len(event.Users)-1)
	for i, curUser := range event.Users {
		// Don't send invitations to the host
		if event.OwnerIs(curUser) {
			continue
		}

		plainText, html, err := template.RenderEvent(template.Event{
			Name:        event.Name,
			Address:     event.Address,
			Time:        event.GetFormatedTime(),
			Description: event.Description,
			FromName:    event.Owner.FullName,
			MagicLink: magic.NewLink(
				curUser.Key,
				strconv.FormatBool(event.HasRSVP(curUser)),
				fmt.Sprintf("rsvp/%s",
					event.Key.Encode())),
		})
		if err != nil {
			return err
		}

		emailMessages[i] = mail.EmailMessage{
			FromName:    event.Owner.FullName,
			FromEmail:   event.GetEmail(),
			ToName:      curUser.FullName,
			ToEmail:     curUser.Email,
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
	plainText, html, err := template.RenderEvent(template.Event{
		Name:        event.Name,
		Address:     event.Address,
		Time:        event.GetFormatedTime(),
		Description: event.Description,
		FromName:    event.Owner.FullName,
		MagicLink: magic.NewLink(
			user.Key,
			strconv.FormatBool(event.HasRSVP(user)),
			fmt.Sprintf("rsvp/%s",
				event.Key.Encode())),
	})
	if err != nil {
		return err
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
	// Loop through all participants and generate emails
	emailMessages := make([]mail.EmailMessage, len(event.Users))
	for i, curUser := range event.Users {
		plainText, html, err := template.RenderCancellation(template.Event{
			Name:     event.Name,
			Address:  event.Address,
			Time:     event.GetFormatedTime(),
			FromName: event.Owner.FullName,
		})
		if err != nil {
			return err
		}

		emailMessages[i] = mail.EmailMessage{
			FromName:    event.Owner.FullName,
			FromEmail:   event.GetEmail(),
			ToName:      curUser.FullName,
			ToEmail:     curUser.Email,
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
	templateEvents := make([]template.Event, len(upcomingEvents))
	for i := range upcomingEvents {
		templateEvents[i] = template.Event{
			Name:    upcomingEvents[i].Name,
			Address: upcomingEvents[i].Address,
			Time:    upcomingEvents[i].GetFormatedTime(),
		}
	}

	// Render all the stuff
	plainText, html, err := template.RenderDigest(template.Digest{
		Items:  items,
		Events: templateEvents,
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    _fromName,
		FromEmail:   _fromEmail,
		ToName:      user.FullName,
		ToEmail:     user.Email,
		Subject:     "[convo] Digest",
		TextContent: plainText,
		HTMLContent: html,
	}

	return mail.Send(email)
}

func getLastFive(messages []*Message) []*Message {
	if len(messages) > 5 {
		return messages[:5]
	}

	return messages
}
