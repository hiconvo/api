package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hiconvo/api/utils/thelpers"
)

////////////////////////////////////
// GET /contacts Tests
////////////////////////////////////

func TestGetContacts(t *testing.T) {
	user, _ := createTestUser(t)
	contact1, _ := createTestUser(t)
	contact2, _ := createTestUser(t)

	user.AddContact(&contact1)
	user.AddContact(&contact2)

	if err := user.Commit(tc); err != nil {
		t.Fatal(err)
	}

	type test struct {
		AuthHeader      map[string]string
		OutCode         int
		OutContactIDs   []string
		OutContactNames []string
	}

	tests := []test{
		{
			AuthHeader:      getAuthHeader(user.Token),
			OutCode:         http.StatusOK,
			OutContactIDs:   []string{contact1.ID, contact2.ID},
			OutContactNames: []string{contact1.FullName, contact2.FullName},
		},
		{
			AuthHeader:      getAuthHeader(contact1.Token),
			OutCode:         http.StatusOK,
			OutContactIDs:   []string{},
			OutContactNames: []string{},
		},
		{
			AuthHeader: nil,
			OutCode:    http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", "/contacts", nil, testCase.AuthHeader)
		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		contacts := respData["contacts"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", testCase.OutContactIDs, contacts)
		thelpers.AssetObjectsContainKeys(t, "fullName", testCase.OutContactNames, contacts)
	}
}

////////////////////////////////////
// POST /contacts/{id} Tests
////////////////////////////////////

func TestCreateContact(t *testing.T) {
	user, _ := createTestUser(t)
	contact1, _ := createTestUser(t)

	type test struct {
		AuthHeader map[string]string
		URL        string
		OutCode    int
	}

	tests := []test{
		{
			AuthHeader: getAuthHeader(user.Token),
			URL:        fmt.Sprintf("/contacts/%s", contact1.ID),
			OutCode:    http.StatusCreated,
		},
		{
			AuthHeader: getAuthHeader(user.Token),
			URL:        fmt.Sprintf("/contacts/%s", contact1.ID),
			OutCode:    http.StatusBadRequest,
		},
		{
			AuthHeader: getAuthHeader(user.Token),
			URL:        fmt.Sprintf("/contacts/%s", user.ID),
			OutCode:    http.StatusBadRequest,
		},
		{
			AuthHeader: nil,
			URL:        fmt.Sprintf("/contacts/%s", contact1.ID),
			OutCode:    http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", testCase.URL, nil, testCase.AuthHeader)
		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], contact1.ID)
		thelpers.AssertEqual(t, respData["fullName"], contact1.FullName)
	}
}

////////////////////////////////////
// DELETE /contacts/{id} Tests
////////////////////////////////////

func TestDeleteContact(t *testing.T) {
	user, _ := createTestUser(t)
	contact1, _ := createTestUser(t)

	user.AddContact(&contact1)

	if err := user.Commit(tc); err != nil {
		t.Fatal(err)
	}

	type test struct {
		AuthHeader map[string]string
		URL        string
		OutCode    int
	}

	tests := []test{
		{
			AuthHeader: getAuthHeader(user.Token),
			URL:        fmt.Sprintf("/contacts/%s", contact1.ID),
			OutCode:    http.StatusOK,
		},
		{
			AuthHeader: getAuthHeader(user.Token),
			URL:        fmt.Sprintf("/contacts/%s", contact1.ID),
			OutCode:    http.StatusBadRequest,
		},
		{
			AuthHeader: getAuthHeader(contact1.Token),
			URL:        fmt.Sprintf("/contacts/%s", contact1.ID),
			OutCode:    http.StatusBadRequest,
		},
		{
			AuthHeader: getAuthHeader(contact1.Token),
			URL:        fmt.Sprintf("/contacts/%s", user.ID),
			OutCode:    http.StatusBadRequest,
		},
		{
			AuthHeader: nil,
			URL:        fmt.Sprintf("/contacts/%s", contact1.ID),
			OutCode:    http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "DELETE", testCase.URL, nil, testCase.AuthHeader)
		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], contact1.ID)
		thelpers.AssertEqual(t, respData["fullName"], contact1.FullName)
	}
}
