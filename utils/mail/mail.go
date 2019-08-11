package mail

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	smail "github.com/sendgrid/sendgrid-go/helpers/mail"

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
	FromName    string
	FromEmail   string
	ToName      string
	ToEmail     string
	Subject     string
	HTMLContent string
	TextContent string
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

	resp, err := client.Send(email)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		return errors.New("Did not recieve OK status code response")
	}

	return nil
}
