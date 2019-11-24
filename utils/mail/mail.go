package mail

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	smail "github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/hiconvo/api/utils/reporter"
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
		reporter.Report(fmt.Errorf("mail.Send: %v", err))
		return fmt.Errorf("mail.Send: %v", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		err := errors.New("mail.Send: received non-200 status from SendGrid")
		reporter.Report(err)
		reporter.Log(resp.Body)
		return err
	}

	return nil
}
