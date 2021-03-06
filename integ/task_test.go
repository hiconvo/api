package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"

	"github.com/hiconvo/api/model"
)

func TestSendEmailsAsync(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member1, _ := _mock.NewUser(_ctx, t)
	member2, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member1, member2})
	thread := _mock.NewThread(_ctx, t, owner, []*model.User{member1, member2})
	_ = _mock.NewThreadMessage(_ctx, t, owner, thread)

	tests := []struct {
		Name         string
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
			ExpectStatus: 404,
		},
		// Missing header
		{
			GivenBody: fmt.Sprintf(`{ "ids": ["%v"], "type": "Event", "action": "SendUpdatedInvites" }`, event.ID),
			GivenHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			ExpectStatus: 404,
		},
	}

	for _, testCase := range tests {
		apitest.New(testCase.Name).
			Handler(_handler).
			Post("/tasks/emails").
			Headers(testCase.GivenHeaders).
			Body(testCase.GivenBody).
			Expect(t).
			Status(testCase.ExpectStatus).
			End()
	}
}

func TestDigest(t *testing.T) {
	apitest.New("Digest").
		Handler(_handler).
		Get("/tasks/digest").
		Headers(map[string]string{"X-Appengine-Cron": "true"}).
		Expect(t).
		Status(http.StatusOK).
		End()
}
