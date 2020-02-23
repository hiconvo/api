package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/thelpers"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
)

////////////////////////////////////
// POST /threads/{id}/messages Tests
////////////////////////////////////

func TestAddMessageToThread(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member1, &member2})

	url := fmt.Sprintf("/threads/%s/messages", thread.ID)

	tests := []struct {
		GivenAuthHeader map[string]string
		GivenBody       string
		GivenAuthor     models.User
		ExpectCode      int
		ExpectBody      string
		ExpectPhoto     bool
	}{
		// Owner
		{
			GivenAuthHeader: getAuthHeader(owner.Token),
			GivenAuthor:     owner,
			GivenBody:       `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q==", "body": "hello"}`,
			ExpectCode:      http.StatusCreated,
			ExpectBody:      "hello",
			ExpectPhoto:     true,
		},
		// Member
		{
			GivenAuthHeader: getAuthHeader(member1.Token),
			GivenAuthor:     member1,
			GivenBody:       `{"body": "hello"}`,
			ExpectCode:      http.StatusCreated,
			ExpectBody:      "hello",
			ExpectPhoto:     false,
		},
		// NonMember
		{
			GivenAuthHeader: getAuthHeader(nonmember.Token),
			GivenAuthor:     nonmember,
			GivenBody:       `{"body": "hello"}`,
			ExpectCode:      http.StatusNotFound,
			ExpectPhoto:     false,
		},
		// EmptyPayload
		{
			GivenAuthHeader: getAuthHeader(member1.Token),
			GivenAuthor:     member1,
			GivenBody:       `{}`,
			ExpectCode:      http.StatusBadRequest,
			ExpectPhoto:     false,
		},
		{
			GivenAuthHeader: getAuthHeader(owner.Token),
			GivenAuthor:     owner,
			GivenBody:       `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q=="}`,
			ExpectCode:      http.StatusBadRequest,
			ExpectPhoto:     false,
		},
	}

	for _, testCase := range tests {
		tt := apitest.New("CreateThreadMessage").
			Handler(th).
			Post(url).
			JSON(testCase.GivenBody).
			Headers(testCase.GivenAuthHeader).
			Expect(t).
			Status(testCase.ExpectCode)

		if testCase.ExpectCode < 300 {
			tt.
				Assert(jsonpath.Equal("$.parentId", thread.ID)).
				Assert(jsonpath.Equal("$.body", testCase.ExpectBody)).
				Assert(jsonpath.Equal("$.user.fullName", testCase.GivenAuthor.FullName)).
				Assert(jsonpath.Equal("$.user.id", testCase.GivenAuthor.ID))
			if testCase.ExpectPhoto {
				tt.Assert(jsonpath.Present("$.photos[0]"))
			} else {
				tt.Assert(jsonpath.NotPresent("$.photos[0]"))
			}
		}

		tt.End()
	}
}

////////////////////////////////////
// DELETE /threads/{id}/messages/{id} Tests
////////////////////////////////////

func TestDeleteThreadMessage(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member1, &member2})
	message1 := createTestThreadMessage(t, &owner, &thread)
	message2 := createTestThreadMessage(t, &member1, &thread)

	message2encoded, err := json.Marshal(message2)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		GivenAuthHeader map[string]string
		GivenMessageID  string
		ExpectCode      int
		ExpectBody      string
	}{
		// Owner tries to delete top message
		{
			GivenAuthHeader: getAuthHeader(owner.Token),
			GivenMessageID:  message1.ID,
			ExpectCode:      http.StatusBadRequest,
			ExpectBody:      `{"message":"You cannot delete this message"}`,
		},
		// Member tries to delete message he does not own
		{
			GivenAuthHeader: getAuthHeader(member1.Token),
			GivenMessageID:  message1.ID,
			ExpectCode:      http.StatusNotFound,
			ExpectBody:      `{"message":"The requested resource was not found"}`,
		},
		// NonMember
		{
			GivenAuthHeader: getAuthHeader(nonmember.Token),
			GivenMessageID:  message1.ID,
			ExpectCode:      http.StatusNotFound,
			ExpectBody:      `{"message":"The requested resource was not found"}`,
		},
		// EmptyPayload
		{
			GivenAuthHeader: getAuthHeader(member1.Token),
			GivenMessageID:  message2.ID,
			ExpectCode:      http.StatusOK,
			ExpectBody:      string(message2encoded),
		},
	}

	for _, testCase := range tests {
		apitest.New("DeleteThreadMessage").
			Handler(th).
			Delete(fmt.Sprintf("/threads/%s/messages/%s", thread.ID, testCase.GivenMessageID)).
			JSON(`{}`).
			Headers(testCase.GivenAuthHeader).
			Expect(t).
			Status(testCase.ExpectCode).
			Body(testCase.ExpectBody).
			End()
	}
}

///////////////////////////////////
// GET /threads/{id}/messages Tests
///////////////////////////////////

func TestGetThreadMessages(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member1, &member2})
	message1 := createTestThreadMessage(t, &owner, &thread)
	message2 := createTestThreadMessage(t, &member1, &thread)
	url := fmt.Sprintf("/threads/%s/messages", thread.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
	}

	tests := []test{
		// Owner can get messages
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK},
		// Member can get messages
		{AuthHeader: getAuthHeader(member1.Token), OutCode: http.StatusOK},
		// NonMember cannot get messages
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		// Unauthenticated user cannot get messages
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		messages := respData["messages"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{message1.ID, message2.ID}, messages)
		thelpers.AssetObjectsContainKeys(t, "body", []string{message1.Body, message2.Body}, messages)
	}
}

////////////////////////////////////
// POST /events/{id}/messages Tests
////////////////////////////////////

func TestAddMessageToEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member1, &member2})

	url := fmt.Sprintf("/events/%s/messages", event.ID)

	tests := []struct {
		GivenAuthHeader map[string]string
		GivenBody       string
		GivenAuthor     models.User
		ExpectCode      int
		ExpectBody      string
		ExpectPhoto     bool
	}{
		// Owner
		{
			GivenAuthHeader: getAuthHeader(owner.Token),
			GivenAuthor:     owner,
			GivenBody:       `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q==", "body": "hello"}`,
			ExpectCode:      http.StatusCreated,
			ExpectBody:      "hello",
			ExpectPhoto:     true,
		},
		// Member
		{
			GivenAuthHeader: getAuthHeader(member1.Token),
			GivenAuthor:     member1,
			GivenBody:       `{"body": "hello"}`,
			ExpectCode:      http.StatusCreated,
			ExpectBody:      "hello",
			ExpectPhoto:     false,
		},
		// NonMember
		{
			GivenAuthHeader: getAuthHeader(nonmember.Token),
			GivenAuthor:     nonmember,
			GivenBody:       `{"body": "hello"}`,
			ExpectCode:      http.StatusNotFound,
			ExpectPhoto:     false,
		},
		// EmptyPayload
		{
			GivenAuthHeader: getAuthHeader(member1.Token),
			GivenAuthor:     member1,
			GivenBody:       `{}`,
			ExpectCode:      http.StatusBadRequest,
			ExpectPhoto:     false,
		},
		{
			GivenAuthHeader: getAuthHeader(owner.Token),
			GivenAuthor:     owner,
			GivenBody:       `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q=="}`,
			ExpectCode:      http.StatusBadRequest,
			ExpectPhoto:     false,
		},
	}

	for _, testCase := range tests {
		tt := apitest.New("CreateThreadMessage").
			Handler(th).
			Post(url).
			JSON(testCase.GivenBody).
			Headers(testCase.GivenAuthHeader).
			Expect(t).
			Status(testCase.ExpectCode)

		if testCase.ExpectCode < 300 {
			tt.
				Assert(jsonpath.Equal("$.parentId", event.ID)).
				Assert(jsonpath.Equal("$.body", testCase.ExpectBody)).
				Assert(jsonpath.Equal("$.user.fullName", testCase.GivenAuthor.FullName)).
				Assert(jsonpath.Equal("$.user.id", testCase.GivenAuthor.ID))
			if testCase.ExpectPhoto {
				tt.Assert(jsonpath.Present("$.photos[0]"))
			} else {
				tt.Assert(jsonpath.NotPresent("$.photos[0]"))
			}
		}

		tt.End()
	}
}

///////////////////////////////////
// GET /threads/{id}/messages Tests
///////////////////////////////////

func TestGetEventMessages(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member1, &member2})
	message1 := createTestEventMessage(t, &owner, event)
	message2 := createTestEventMessage(t, &member1, event)
	url := fmt.Sprintf("/events/%s/messages", event.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
	}

	tests := []test{
		// Owner can get messages
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK},
		// Member can get messages
		{AuthHeader: getAuthHeader(member1.Token), OutCode: http.StatusOK},
		// NonMember cannot get messages
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		// Unauthenticated user cannot get messages
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		messages := respData["messages"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{message1.ID, message2.ID}, messages)
		thelpers.AssetObjectsContainKeys(t, "body", []string{message1.Body, message2.Body}, messages)
	}
}
