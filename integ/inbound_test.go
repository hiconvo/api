package router_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/thelpers"
)

func TestInboundSucceedsWithValidPayload(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2, &u3})

	messages, err := models.GetMessagesByThread(tc, &thread)
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
	req.WithContext(tc)

	rr := httptest.NewRecorder()
	th.ServeHTTP(rr, req)

	newMessages, err := models.GetMessagesByThread(tc, &thread)
	if err != nil {
		t.Fatal(err)
	}
	finalMessageCount := len(newMessages)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)
	thelpers.AssertEqual(t, finalMessageCount > initalMessageCount, true)
}

// func TestInboundFailsWithInvalidPayload(t *testing.T) {
// 	invalidText := "SOMETHING_INVALID"

// 	var b bytes.Buffer
// 	form := multipart.NewWriter(&b)

// 	form.WriteField("dkim", "{@sendgrid.com : pass}")
// 	form.WriteField("to", invalidText)
// 	form.WriteField("html", "<html><body><p>Hello, does this work?</p></body></html>")
// 	form.WriteField("from", fmt.Sprintf("%s <%s>", invalidText, invalidText))
// 	form.WriteField("text", "Hello, does this work?")
// 	form.WriteField("sender_ip", "0.0.0.0")
// 	form.WriteField("envelope", fmt.Sprintf(`{"to":["%s"],"from":"%s"}`, invalidText, invalidText))
// 	form.WriteField("attachments", "0")
// 	form.WriteField("subject", invalidText)
// 	form.WriteField("charsets", `{"to":"UTF-8","html":"UTF-8","subject":"UTF-8","from":"UTF-8","text":"UTF-8"}`)
// 	form.WriteField("SPF", "pass")

// 	form.Close()

// 	req, err := http.NewRequest("POST", "/inbound", &b)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	req.Header.Add("Content-Type", form.FormDataContentType())
// 	req.WithContext(tc)

// 	rr := httptest.NewRecorder()
// 	th.ServeHTTP(rr, req)

// 	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)
// }
