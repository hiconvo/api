package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/thelpers"
)

//////////////////////
// POST /threads/{id}/reads Tests
//////////////////////

func TestMarkThreadAsRead(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member})
	url := fmt.Sprintf("/threads/%s/reads", thread.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK},
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusOK},
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized},
	}

	for _, testCase := range tests {
		_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, testCase.AuthHeader)
		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)
	}
}

//////////////////////
// POST /events/{id}/reads Tests
//////////////////////

func TestMarkEventAsRead(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member}, []*models.User{})
	url := fmt.Sprintf("/events/%s/reads", event.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK},
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusOK},
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized},
	}

	for _, testCase := range tests {
		_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, testCase.AuthHeader)
		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)
	}
}
