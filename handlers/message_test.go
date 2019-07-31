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
		Body       string
		Headers    map[string]string
		StatusCode int
	}

	tests := []test{
		// Owner
		{Body: random.String(10), Headers: getAuthHeader(owner.Token), StatusCode: http.StatusCreated},
		// Member
		{Body: random.String(10), Headers: getAuthHeader(member1.Token), StatusCode: http.StatusCreated},
		// NonMember
		{Body: random.String(10), Headers: getAuthHeader(nonmember.Token), StatusCode: http.StatusNotFound},
		// EmptyPayload
		{Body: "", Headers: getAuthHeader(member1.Token), StatusCode: http.StatusBadRequest},
	}

	for _, testCase := range tests {
		reqData := map[string]interface{}{"body": testCase.Body}

		_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", url, reqData, testCase.Headers)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.StatusCode)
	}
}

///////////////////////////////////
// GET /threads/{id}/messages Tests
///////////////////////////////////

func TestGetMessages(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member1, &member2})
	message1 := createTestMessage(t, &owner, &thread)
	message2 := createTestMessage(t, &member1, &thread)
	url := fmt.Sprintf("/threads/%s/messages", thread.ID)

	type test struct {
		Headers    map[string]string
		StatusCode int
	}

	tests := []test{
		// Owner can get messages
		{Headers: getAuthHeader(owner.Token), StatusCode: http.StatusOK},
		// Member can get messages
		{Headers: getAuthHeader(member1.Token), StatusCode: http.StatusOK},
		// NonMember cannot get messages
		{Headers: getAuthHeader(nonmember.Token), StatusCode: http.StatusNotFound},
		// Unauthenticated user cannot get messages
		{Headers: map[string]string{"boop": "beep"}, StatusCode: http.StatusUnauthorized},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, testCase.Headers)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.StatusCode)

		if testCase.StatusCode == http.StatusOK {
			messages := respData["messages"].([]interface{})
			thelpers.AssertObjectsContainIDs(t, messages, []string{message1.ID, message2.ID})
		}
	}
}
