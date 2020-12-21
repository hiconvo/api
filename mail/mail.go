package mail

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/clients/mail"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/template"
)

const (
	_fromEmail = "robots@mail.convo.events"
	_fromName  = "Convo"
)

type Client struct {
	mail mail.Client
	tpl  *template.Client

	tplStrPasswordReset string
	tplStrVerifyEmail   string
	tplStrMergeAccounts string
}

func New(sender mail.Client, tpl *template.Client) *Client {
	return &Client{
		mail: sender,
		tpl:  tpl,

		tplStrPasswordReset: readStringFromFile("password-reset.txt"),
		tplStrVerifyEmail:   readStringFromFile("verify-email.txt"),
		tplStrMergeAccounts: readStringFromFile("merge-accounts.txt"),
	}
}

func (c *Client) SendPasswordResetEmail(u *model.User, magicLink string) error {
	plainText, html, err := c.tpl.RenderAdminEmail(&template.AdminEmail{
		Body:       c.tplStrPasswordReset,
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

	return c.mail.Send(email)
}

func (c *Client) SendVerifyEmail(u *model.User, emailAddress, magicLink string) error {
	plainText, html, err := c.tpl.RenderAdminEmail(&template.AdminEmail{
		Body:       c.tplStrVerifyEmail,
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

	return c.mail.Send(email)
}

func (c *Client) SendMergeAccountsEmail(u *model.User, emailToMerge, magicLink string) error {
	plainText, html, err := c.tpl.RenderAdminEmail(&template.AdminEmail{
		Body:       c.tplStrMergeAccounts,
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

	return c.mail.Send(email)
}

// SendThread sends thread emails only to non-registered users.
func (c *Client) SendThread(
	magicClient magic.Client,
	thread *model.Thread,
	messages []*model.Message,
) error {
	// From is the most recent message sender: messages[0].User.
	sender, err := model.MapUserPartialToUser(messages[0].User, thread.Users)
	if err != nil {
		return err
	}

	// Filter out registered users
	var users []*model.User
	for i := range thread.Users {
		if thread.Users[i].SendThreads && !thread.Users[i].IsRegistered() && !model.IsRead(thread, thread.Users[i].Key) {
			users = append(users, thread.Users[i])
		}
	}

	// Loop through all participants and generate emails.
	//
	emailMessages := make([]mail.EmailMessage, len(users))
	// Get the last five messages to be included in the email.
	lastFive := getLastFive(messages)

	for i, curUser := range users {
		// Don't send an email to the sender.
		if curUser.Key.Equal(sender.Key) || !curUser.SendThreads {
			continue
		}

		// Generate messages
		cleanMessages := make([]*model.Message, 0)

		// If there are fewer than five messages, include the info in the thread by
		// creating a pseudo-message
		if len(messages) < 5 {
			firstMessage := &model.Message{
				Body:      thread.Body,
				PhotoKeys: thread.Photos,
				Link:      thread.Link,
				User:      thread.Owner,
				UserKey:   thread.OwnerKey,
				ParentKey: thread.Key,
				ParentID:  thread.Key.Encode(),
				CreatedAt: thread.CreatedAt,
			}
			cleanMessages = append(cleanMessages, lastFive...)
			cleanMessages = append(cleanMessages, firstMessage)
		} else {
			cleanMessages = lastFive
		}

		tplMessages := make([]template.Message, len(cleanMessages))
		for j, m := range cleanMessages {
			tplMessages[j] = template.Message{
				Body:     m.Body,
				Name:     m.User.FirstName,
				HasPhoto: m.HasPhoto(),
				HasLink:  m.HasLink(),
				Link:     m.Link,
				FromID:   m.User.ID,
				ToID:     curUser.ID,
				// Since these users are not registered, we do not show a magic login link
				MagicLink: "https://app.convo.events",
			}
		}

		plainText, html, err := c.tpl.RenderThread(&template.Thread{
			Subject:              thread.Subject,
			FromName:             sender.FullName,
			Messages:             tplMessages,
			UnsubscribeMagicLink: curUser.GetUnsubscribeMagicLink(magicClient),
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

		if err := c.mail.Send(emailMessages[i]); err != nil {
			log.Alarm(errors.Errorf("mail.SendThread: %v", err))
		}
	}

	return nil
}

func (c *Client) SendEventInvites(
	magicClient magic.Client,
	event *model.Event,
	isUpdate bool,
) error {
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
		if event.OwnerIs(curUser) || !curUser.SendEvents {
			continue
		}

		plainText, html, err := c.tpl.RenderEvent(&template.Event{
			Name:                 event.Name,
			Address:              event.Address,
			Time:                 event.GetFormatedTime(),
			Description:          event.Description,
			FromName:             event.Owner.FullName,
			MagicLink:            event.GetRSVPMagicLink(magicClient, curUser),
			ButtonText:           "RSVP",
			UnsubscribeMagicLink: curUser.GetUnsubscribeMagicLink(magicClient),
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

		if err := c.mail.Send(emailMessages[i]); err != nil {
			log.Alarm(errors.Errorf("mail.SendEventInvites: %v", err))
		}
	}

	return nil
}

func (c *Client) SendEventInvitation(m magic.Client, event *model.Event, user *model.User) error {
	if !user.SendEvents {
		return nil
	}

	plainText, html, err := c.tpl.RenderEvent(&template.Event{
		Name:                 event.Name,
		Address:              event.Address,
		Time:                 event.GetFormatedTime(),
		Description:          event.Description,
		FromName:             event.Owner.FullName,
		MagicLink:            event.GetRSVPMagicLink(m, user),
		ButtonText:           "RSVP",
		UnsubscribeMagicLink: user.GetUnsubscribeMagicLink(m),
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

	return c.mail.Send(email)
}

func (c *Client) SendCancellation(m magic.Client, event *model.Event, message string) error {
	emailMessages := make([]mail.EmailMessage, len(event.Users))

	// Loop through all participants and generate emails
	for i, curUser := range event.Users {
		if !curUser.SendEvents {
			continue
		}

		plainText, html, err := c.tpl.RenderCancellation(&template.Event{
			Name:                 event.Name,
			Address:              event.Address,
			Time:                 event.GetFormatedTime(),
			FromName:             event.Owner.FullName,
			Message:              message,
			UnsubscribeMagicLink: curUser.GetUnsubscribeMagicLink(m),
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

		if err := c.mail.Send(emailMessages[i]); err != nil {
			log.Alarm(errors.Errorf("mail.sendCancellation: %v", err))
		}
	}

	return nil
}

func (c *Client) SendDigest(
	magicClient magic.Client,
	digestList []*model.DigestItem,
	upcomingEvents []*model.Event,
	user *model.User,
) error {
	if !user.SendDigest {
		return nil
	}

	magicLink := user.GetMagicLoginMagicLink(magicClient)
	// Convert all the DigestItems into template.Threads with their messages
	items := make([]template.Thread, len(digestList))
	for i := range digestList {
		messages := make([]template.Message, len(digestList[i].Messages))
		for j := range messages {
			messages[j] = template.Message{
				Body:      digestList[i].Messages[j].Body,
				Name:      digestList[i].Messages[j].User.FullName,
				HasPhoto:  digestList[i].Messages[j].HasPhoto(),
				HasLink:   digestList[i].Messages[j].HasLink(),
				Link:      digestList[i].Messages[j].Link,
				FromID:    digestList[i].Messages[j].User.ID,
				ToID:      user.ID,
				MagicLink: magicLink,
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
	plainText, html, err := c.tpl.RenderDigest(&template.Digest{
		Items:                items,
		Events:               templateEvents,
		MagicLink:            magicLink,
		UnsubscribeMagicLink: user.GetUnsubscribeMagicLink(magicClient),
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

	return c.mail.Send(email)
}

func (c *Client) SendInboundTryAgainEmail(email string) error {
	return c.mail.Send(mail.EmailMessage{
		FromName:    "Convo",
		FromEmail:   "support@mail.convo.events",
		ToName:      "",
		ToEmail:     email,
		Subject:     "[convo] Send Failure",
		HTMLContent: "<p>Hello,</p><p>You responded to a Convo that does not accept email responses. If you're attempting to respond to an event invitation, click RSVP in the invitation email and post your message to the message board. You won't have to create an account.</p><p>Thanks,<br />Convo Support</p>",
		TextContent: "Hello,\n\nYou responded to a Convo that does not accept email responses. If you're attempting to respond to an event invitation, click RSVP in the invitation email and post your message to the message board. You won't have to create an account.\n\nThanks,\nConvo Support",
	})
}

func (c *Client) SendInboundErrorEmail(email string) error {
	return c.mail.Send(mail.EmailMessage{
		FromName:    "Convo",
		FromEmail:   "support@mail.convo.events",
		ToName:      "",
		ToEmail:     email,
		Subject:     "[convo] Send Failure",
		HTMLContent: "<p>Hello,</p><p>You responded to a Convo from an unrecognized email address. Please try again and make sure that you use the exact email to which the Convo was addressed.</p><p>Thanks,<br />Convo Support</p>",
		TextContent: "Hello,\n\nYou responded to a Convo from an unrecognized email address. Please try again and make sure that you use the exact email to which the Convo was addressed.\n\nThanks,\nConvo Support",
	})
}

func getLastFive(messages []*model.Message) []*model.Message {
	if len(messages) > 5 {
		return messages[:5]
	}

	return messages
}

func readStringFromFile(file string) string {
	op := errors.Opf("mail.readStringFromFile(file=%s)", file)

	wd, err := os.Getwd()
	if err != nil {
		// This function should only be run at startup time, so we
		// panic if it fails.
		panic(errors.E(op, err))
	}

	var basePath string
	if strings.HasSuffix(wd, "mail") || strings.HasSuffix(wd, "integ") {
		// This package is the cwd, so we need to go up one dir to resolve the
		// layouts and includes dirs consistently.
		basePath = "../mail/content"
	} else {
		basePath = "./mail/content"
	}

	b, err := ioutil.ReadFile(path.Join(basePath, file))
	if err != nil {
		panic(err)
	}

	return string(b)
}
