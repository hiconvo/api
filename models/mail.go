package models

import (
	"fmt"
	"strconv"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/mail"
	"github.com/hiconvo/api/utils/template"
)

const (
	_fromEmail = "robots@mail.convo.events"
	_fromName  = "Convo"
)

const (
	_tplStrPasswordReset = "Please click the link below to set your password."
	_tplStrVerifyEmail   = "Please click the link below to verify your email address."
	_tplStrMergeAccounts = "Please click the link below to verify your email address. This will merge your account with %s into your account with %s. If you did not attempt to add a new email to your account, it might be a good idea to notifiy support@convo.events."
)

func sendPasswordResetEmail(u *User, magicLink string) error {
	plainText, html, err := template.RenderAdminEmail(template.AdminEmail{
		Body:       _tplStrPasswordReset,
		ButtonText: "Set password",
		MagicLink:  magicLink,
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    _fromName,
		FromEmail:   _fromEmail,
		ToName:      u.FullName,
		ToEmail:     u.Email,
		Subject:     "[convo] Set Password",
		TextContent: plainText,
		HTMLContent: html,
	}

	return mail.Send(email)
}

func sendVerifyEmail(u *User, emailAddress, magicLink string) error {
	plainText, html, err := template.RenderAdminEmail(template.AdminEmail{
		Body:       _tplStrVerifyEmail,
		ButtonText: "Verify",
		MagicLink:  magicLink,
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    _fromName,
		FromEmail:   _fromEmail,
		ToName:      u.FullName,
		ToEmail:     emailAddress,
		Subject:     "[convo] Verify Email",
		TextContent: plainText,
		HTMLContent: html,
	}

	return mail.Send(email)
}

func sendMergeAccountsEmail(u *User, emailToMerge, magicLink string) error {
	plainText, html, err := template.RenderAdminEmail(template.AdminEmail{
		Body:       _tplStrMergeAccounts,
		ButtonText: "Verify",
		MagicLink:  magicLink,
		Fargs:      []interface{}{emailToMerge, u.Email},
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:    _fromName,
		FromEmail:   _fromEmail,
		ToName:      u.FullName,
		ToEmail:     u.Email,
		Subject:     "[convo] Verify Email",
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
		for j, m := range lastFive {
			tplMessages[j] = template.Message{
				Body:     m.Body,
				Name:     m.User.FirstName,
				HasPhoto: m.HasPhoto(),
				HasLink:  m.HasLink(),
				FromID:   m.User.ID,
				ToID:     curUser.ID,
			}
		}

		plainText, html, err := template.RenderThread(template.Thread{
			Subject:   thread.Subject,
			FromName:  sender.FullName,
			Messages:  tplMessages,
			MagicLink: magic.NewLink(curUser.Key, curUser.Token, "magic"),
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
		if emailMessages[i].FromEmail == "" {
			continue
		}

		if err := mail.Send(emailMessages[i]); err != nil {
			log.Alarm(errors.Errorf("models.sendThread: %v", err))
		}
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
	emailMessages := make([]mail.EmailMessage, len(event.Users))
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
				strconv.FormatBool(!event.IsInFuture()),
				fmt.Sprintf("rsvp/%s",
					event.Key.Encode())),
			ButtonText: "RSVP",
		})
		if err != nil {
			return err
		}

		emailMessages[i] = mail.EmailMessage{
			FromName:      event.Owner.FullName,
			FromEmail:     event.GetEmail(),
			ToName:        curUser.FullName,
			ToEmail:       curUser.Email,
			Subject:       fmt.Sprintf(fmtStr, event.Name),
			TextContent:   plainText,
			HTMLContent:   html,
			ICSAttachment: event.GetICS(),
		}
	}

	for i := range emailMessages {
		if emailMessages[i].FromEmail == "" {
			continue
		}

		if err := mail.Send(emailMessages[i]); err != nil {
			log.Alarm(errors.Errorf("models.sendEvent: %v", err))
		}
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
			strconv.FormatBool(!event.IsInFuture()),
			fmt.Sprintf("rsvp/%s",
				event.Key.Encode())),
		ButtonText: "RSVP",
	})
	if err != nil {
		return err
	}

	email := mail.EmailMessage{
		FromName:      event.Owner.FullName,
		FromEmail:     event.GetEmail(),
		ToName:        user.FullName,
		ToEmail:       user.Email,
		Subject:       fmt.Sprintf("Invitation to %s", event.Name),
		TextContent:   plainText,
		HTMLContent:   html,
		ICSAttachment: event.GetICS(),
	}

	return mail.Send(email)
}

func sendCancellation(event *Event, message string) error {
	// Loop through all participants and generate emails
	emailMessages := make([]mail.EmailMessage, len(event.Users))
	for i, curUser := range event.Users {
		plainText, html, err := template.RenderCancellation(template.Event{
			Name:     event.Name,
			Address:  event.Address,
			Time:     event.GetFormatedTime(),
			FromName: event.Owner.FullName,
			Message:  message,
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
		if emailMessages[i].FromEmail == "" {
			continue
		}

		if err := mail.Send(emailMessages[i]); err != nil {
			log.Alarm(errors.Errorf("models.sendCancellation: %v", err))
		}
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
				Body:     digestList[i].Messages[j].Body,
				Name:     digestList[i].Messages[j].User.FullName,
				HasPhoto: digestList[i].Messages[j].HasPhoto(),
				HasLink:  digestList[i].Messages[j].HasLink(),
				FromID:   digestList[i].Messages[j].User.ID,
				ToID:     user.ID,
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
		Items:     items,
		Events:    templateEvents,
		MagicLink: magic.NewLink(user.Key, user.Token, "magic"),
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
