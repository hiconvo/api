package models

import (
	"fmt"
	"strings"

	"github.com/hiconvo/api/utils/mail"
	"github.com/hiconvo/api/utils/template"
)

const (
	fromEmail = "robots@hiconvo.com"
	fromName  = "ConvoBot"

	passwordReset = "Hello,\n\nPlease [click here]( %s ) to reset your password.\n\nThanks,<br />ConvoBot"
	verifyEmail   = "Hello,\n\nPlease [click here]( %s ) to verify your email address.\n\nThanks,<br />ConvoBot"
	messageTplStr = "%s said:\n\n%s\n\n"
)

func sendPasswordResetEmail(u *User, magicLink string) error {
	return sendAdministrativeEmail(u, passwordReset, "[convo] Password Reset", magicLink)
}

func sendVerifyEmail(u *User, magicLink string) error {
	return sendAdministrativeEmail(u, verifyEmail, "[convo] Verify Email", magicLink)
}

func sendAdministrativeEmail(u *User, strTpl, subject, magicLink string) error {
	plainText := fmt.Sprintf(strTpl, magicLink)

	html, err := template.Render(template.Thread{
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

	// TODO: append convobot controls at the end

	plainText := builder.String()

	// Use the first 200 chars as the preview
	var preview string
	if len(plainText) > 200 {
		preview = plainText[:200] + "..."
	} else {
		preview = plainText
	}

	html, err := template.Render(template.Thread{
		Subject:  thread.Subject,
		Messages: tplMessages,
		Preview:  preview,
	})
	if err != nil {
		return "", "", err
	}

	return plainText, html, nil
}
