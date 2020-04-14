package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

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

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		ExpectStatus    int
		ExpectIDs       []string
		ExpectNames     []string
	}{
		{
			Name:            "many contacts",
			GivenAuthHeader: getAuthHeader(user.Token),
			ExpectStatus:    http.StatusOK,
			ExpectIDs:       []string{contact1.ID, contact2.ID},
			ExpectNames:     []string{contact1.FullName, contact2.FullName},
		},
		{
			Name:            "zero contacts",
			GivenAuthHeader: getAuthHeader(contact1.Token),
			ExpectStatus:    http.StatusOK,
			ExpectIDs:       []string{},
			ExpectNames:     []string{},
		},
		{
			Name:            "bad auth",
			GivenAuthHeader: nil,
			ExpectStatus:    http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			tt := apitest.New(testCase.Name).
				Handler(th).
				Get("/contacts").
				Headers(testCase.GivenAuthHeader).
				Expect(t).
				Status(testCase.ExpectStatus)
			if testCase.ExpectStatus == http.StatusOK {
				for i := range testCase.ExpectIDs {
					tt.Assert(jsonpath.Contains("$.contacts[*].id", testCase.ExpectIDs[i]))
					tt.Assert(jsonpath.Contains("$.contacts[*].fullName", testCase.ExpectNames[i]))
				}
			}
			tt.End()
		})
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
		thelpers.AssertEqual(t, respData["token"], nil)
		thelpers.AssertEqual(t, respData["email"], nil)
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
		thelpers.AssertEqual(t, respData["token"], nil)
		thelpers.AssertEqual(t, respData["email"], nil)
	}
}
