package router_test

import (
	"fmt"
	"testing"

	"github.com/steinfletcher/apitest"

	"github.com/hiconvo/api/models"
)

func TestSendEmailsAsync(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member1, &member2})
	thread := createTestThread(t, &owner, []*models.User{&member1, &member2})

	tests := []struct {
		GivenBody    string
		GivenHeaders map[string]string
		ExpectStatus int
	}{
		{
			GivenBody: fmt.Sprintf(`{ "ids": ["%v"], "type": "Event", "action": "SendInvites" }`, event.ID),
			GivenHeaders: map[string]string{
				"Content-Type":          "application/json",
				"X-Appengine-Queuename": "convo-emails",
			},
			ExpectStatus: 200,
		},
		{
			GivenBody: fmt.Sprintf(`{ "ids": ["%v"], "type": "Event", "action": "SendUpdatedInvites" }`, event.ID),
			GivenHeaders: map[string]string{
				"Content-Type":          "application/json",
				"X-Appengine-Queuename": "convo-emails",
			},
			ExpectStatus: 200,
		},
		{
			GivenBody: fmt.Sprintf(`{ "ids": ["%v"], "type": "Thread", "action": "SendThread" }`, thread.ID),
			GivenHeaders: map[string]string{
				"Content-Type":          "application/json",
				"X-Appengine-Queuename": "convo-emails",
			},
			ExpectStatus: 200,
		},
		{
			GivenBody: fmt.Sprintf(`{ "ids": ["%v", "%v", "%v"], "type": "User", "action": "SendWelcome" }`, owner.ID, member1.ID, member2.ID),
			GivenHeaders: map[string]string{
				"Content-Type":          "application/json",
				"X-Appengine-Queuename": "convo-emails",
			},
			ExpectStatus: 200,
		},
		// Invalid payload
		{
			GivenBody:    fmt.Sprintf(`{ "ids": ["%v"], "type": "Thread", "action": "SendInvites" }`, event.ID),
			ExpectStatus: 400,
		},
		// Missing header
		{
			GivenBody: fmt.Sprintf(`{ "ids": ["%v"], "type": "Event", "action": "SendUpdatedInvites" }`, event.ID),
			GivenHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			ExpectStatus: 400,
		},
	}

	for _, testCase := range tests {
		apitest.New("SendEmailsAsync").
			Handler(th).
			Post("/tasks/emails").
			Headers(testCase.GivenHeaders).
			Body(testCase.GivenBody).
			Expect(t).
			Status(testCase.ExpectStatus)
	}
}
