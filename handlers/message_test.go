package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/thelpers"
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

	type test struct {
		AuthHeader map[string]string
		Body       string
		Author     models.User
		OutCode    int
	}

	tests := []test{
		// Owner
		{Body: random.String(10), AuthHeader: getAuthHeader(owner.Token), Author: owner, OutCode: http.StatusCreated},
		// Member
		{Body: random.String(10), AuthHeader: getAuthHeader(member1.Token), Author: member1, OutCode: http.StatusCreated},
		// NonMember
		{Body: random.String(10), AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		// EmptyPayload
		{Body: "", AuthHeader: getAuthHeader(member1.Token), OutCode: http.StatusBadRequest},
	}

	for _, testCase := range tests {
		reqData := map[string]interface{}{"body": testCase.Body}

		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", url, reqData, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["body"], testCase.Body)
		thelpers.AssertEqual(t, respData["parentId"], thread.ID)

		gotMessageUser := respData["user"].(map[string]interface{})
		thelpers.AssertEqual(t, gotMessageUser["fullName"], testCase.Author.FullName)
		thelpers.AssertEqual(t, gotMessageUser["id"], testCase.Author.ID)
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
	message1 := createTestMessage(t, &owner, &thread)
	message2 := createTestMessage(t, &member1, &thread)
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

	type test struct {
		AuthHeader map[string]string
		Body       string
		Author     models.User
		OutCode    int
	}

	tests := []test{
		// Owner
		{Body: random.String(10), AuthHeader: getAuthHeader(owner.Token), Author: owner, OutCode: http.StatusCreated},
		// Member
		{Body: random.String(10), AuthHeader: getAuthHeader(member1.Token), Author: member1, OutCode: http.StatusCreated},
		// NonMember
		{Body: random.String(10), AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		// EmptyPayload
		{Body: "", AuthHeader: getAuthHeader(member1.Token), OutCode: http.StatusBadRequest},
	}

	for _, testCase := range tests {
		reqData := map[string]interface{}{"body": testCase.Body}

		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", url, reqData, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["body"], testCase.Body)
		thelpers.AssertEqual(t, respData["parentId"], event.ID)

		gotMessageUser := respData["user"].(map[string]interface{})
		thelpers.AssertEqual(t, gotMessageUser["fullName"], testCase.Author.FullName)
		thelpers.AssertEqual(t, gotMessageUser["id"], testCase.Author.ID)
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
	message1 := createTestEventMessage(t, &owner, &event)
	message2 := createTestEventMessage(t, &member1, &event)
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
