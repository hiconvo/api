package handler_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hiconvo/api/model"
)

func TestInbound(t *testing.T) {
	u1, _ := _mock.NewUser(_ctx, t)
	u2, _ := _mock.NewUser(_ctx, t)
	u3, _ := _mock.NewUser(_ctx, t)
	thread := _mock.NewThread(_ctx, t, u1, []*model.User{u2, u3})

	messages, err := _mock.MessageStore.GetMessagesByThread(_ctx, thread)
	if err != nil {
		t.Fatal(err)
	}
	initalMessageCount := len(messages)

	var b bytes.Buffer
	form := multipart.NewWriter(&b)

	form.WriteField("dkim", "{@sendgrid.com : pass}")
	form.WriteField("to", thread.GetEmail())
	form.WriteField("html", "<html><body><p>Hello, does this work?</p></body></html>")
	form.WriteField("from", fmt.Sprintf("%s <%s>", u1.FullName, u1.Email))
	form.WriteField("text", "Hello, does this work?")
	form.WriteField("sender_ip", "0.0.0.0")
	form.WriteField("envelope", fmt.Sprintf(`{"to":["%s"],"from":"%s"}`, thread.GetEmail(), u1.Email))
	form.WriteField("attachments", "0")
	form.WriteField("subject", thread.Subject)
	form.WriteField("charsets", `{"to":"UTF-8","html":"UTF-8","subject":"UTF-8","from":"UTF-8","text":"UTF-8"}`)
	form.WriteField("SPF", "pass")

	form.Close()

	req, err := http.NewRequest("POST", "/inbound", &b)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", form.FormDataContentType())
	req.WithContext(_ctx)

	rr := httptest.NewRecorder()
	_handler.ServeHTTP(rr, req)

	newMessages, err := _mock.MessageStore.GetMessagesByThread(_ctx, thread)
	if err != nil {
		t.Fatal(err)
	}
	finalMessageCount := len(newMessages)

	assert.Equal(t, rr.Code, http.StatusOK)
	assert.Equal(t, finalMessageCount > initalMessageCount, true)
}
