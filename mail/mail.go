package mail

import (
	"encoding/base64"
	"net/http"

	"github.com/sendgrid/sendgrid-go"
	smail "github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/utils/secrets"
)

// EmailMessage is a sendable email message. All of its fields
// are strings. No additional processing or rendering is done
// in this package.
type EmailMessage struct {
	FromName      string
	FromEmail     string
	ToName        string
	ToEmail       string
	Subject       string
	HTMLContent   string
	TextContent   string
	ICSAttachment string
}

var DefaultClient Client

func init() {
	if apiKey := secrets.Get("SENDGRID_API_KEY", ""); apiKey == "" {
		DefaultClient = NewLogger()
	} else {
		DefaultClient = NewClient(apiKey)
	}
}

func Send(e EmailMessage) error {
	return DefaultClient.Send(e)
}

type Client interface {
	Send(e EmailMessage) error
}

type senderImpl struct {
	client *sendgrid.Client
}

func NewClient(apiKey string) Client {
	return &senderImpl{
		client: sendgrid.NewSendClient(apiKey),
	}
}

// Send sends the given EmailMessage.
func (s *senderImpl) Send(e EmailMessage) error {
	from := smail.NewEmail(e.FromName, e.FromEmail)
	to := smail.NewEmail(e.ToName, e.ToEmail)
	email := smail.NewSingleEmail(
		from, e.Subject, to, e.TextContent, e.HTMLContent,
	)

	if e.ICSAttachment != "" {
		attachment := smail.NewAttachment()
		attachment.SetContent(base64.StdEncoding.EncodeToString([]byte(e.ICSAttachment)))
		attachment.SetType("text/calendar")
		attachment.SetFilename("event.ics")

		email.AddAttachment(attachment)
	}

	resp, err := s.client.Send(email)
	if err != nil {
		return errors.E(errors.Op("mail.Send"), err)
	}

	if resp.StatusCode != http.StatusAccepted {
		log.Print(resp.Body)
		return errors.E(errors.Op("mail.Send"), errors.Str("received non-200 status from SendGrid"))
	}

	return nil
}

type loggerImpl struct{}

func NewLogger() Client {
	log.Print("mail.NewLogger: USING MAIL LOGGER FOR LOCAL DEVELOPMENT")
	return &loggerImpl{}
}

func (l *loggerImpl) Send(e EmailMessage) error {
	log.Printf("mail.Send(from='%s', to='%s')", e.FromEmail, e.ToEmail)
	return nil
}
