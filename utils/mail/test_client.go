package mail

import (
	"github.com/sendgrid/rest"
	smail "github.com/sendgrid/sendgrid-go/helpers/mail"
)

type testClient struct{}

func (s *testClient) Send(email *smail.SGMailV3) (*rest.Response, error) {
	return &rest.Response{
		StatusCode: 202,
		Body:       "{}",
		Headers:    map[string][]string{},
	}, nil
}
