package mail

import (
	"encoding/base64"
	"net/http"
	"os"
	"strings"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	smail "github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/utils/secrets"
)

var client sender

type sender interface {
	Send(email *smail.SGMailV3) (*rest.Response, error)
}

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

func init() {
	if strings.HasSuffix(os.Args[0], ".test") {
		client = &testClient{}
	} else {
		client = sendgrid.NewSendClient(secrets.Get("SENDGRID_API_KEY", ""))
	}
}

// Send sends the given EmailMessage.
func Send(e EmailMessage) error {
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

	resp, err := client.Send(email)
	if err != nil {
		return errors.E(errors.Op("mail.Send"), err)
	}

	if resp.StatusCode != http.StatusAccepted {
		log.Print(resp.Body)
		return errors.E(errors.Op("mail.Send"), errors.Str("received non-200 status from SendGrid"))
	}

	return nil
}
