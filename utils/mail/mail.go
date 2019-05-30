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
	if strings.Contains(os.Args[0], "/_test/") {
		client = &testClient{}
	} else {
		client = sendgrid.NewSendClient(secrets.Get("SENDGRID_API_KEY"))
	}
}

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
