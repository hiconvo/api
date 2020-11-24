package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	"github.com/hiconvo/api/testutil"
)

func TestGetContacts(t *testing.T) {
	user, _ := _mock.NewUser(_ctx, t)
	contact1, _ := _mock.NewUser(_ctx, t)
	contact2, _ := _mock.NewUser(_ctx, t)

	if err := user.AddContact(contact1); err != nil {
		t.Fatal(err)
	}
	if err := user.AddContact(contact2); err != nil {
		t.Fatal(err)
	}
	if _, err := _dbClient.Put(_ctx, user.Key, user); err != nil {
		t.Fatal(err)
	}

	t.Log(contact1.ID, contact2.ID)
	t.Log(contact1.FullName, contact2.FullName)

	tests := []struct {
		Name               string
		AuthHeader         map[string]string
		ExpectStatus       int
		ExpectContactIDs   []string
		ExpectContactNames []string
	}{
		{
			AuthHeader:         testutil.GetAuthHeader(user.Token),
			ExpectStatus:       http.StatusOK,
			ExpectContactIDs:   []string{contact1.ID, contact2.ID},
			ExpectContactNames: []string{contact1.FullName, contact2.FullName},
		},
		{
			AuthHeader:         testutil.GetAuthHeader(contact1.Token),
			ExpectStatus:       http.StatusOK,
			ExpectContactIDs:   []string{},
			ExpectContactNames: []string{},
		},
		{
			AuthHeader:   nil,
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get("/contacts").
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < 400 {
				tt.Assert(jsonpath.Len("$.contacts[*].id", len(tcase.ExpectContactNames)))

				for _, name := range tcase.ExpectContactNames {
					tt.Assert(jsonpath.Contains("$.contacts[*].fullName", name))
				}

				for _, id := range tcase.ExpectContactIDs {
					tt.Assert(jsonpath.Contains("$.contacts[*].id", id))
				}
			}

			tt.End()
		})
	}
}

func TestCreateContact(t *testing.T) {
	user, _ := _mock.NewUser(_ctx, t)
	contact1, _ := _mock.NewUser(_ctx, t)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		URL          string
		ExpectStatus int
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(user.Token),
			URL:          fmt.Sprintf("/contacts/%s", contact1.ID),
			ExpectStatus: http.StatusCreated,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(user.Token),
			URL:          fmt.Sprintf("/contacts/%s", contact1.ID),
			ExpectStatus: http.StatusBadRequest,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(user.Token),
			URL:          fmt.Sprintf("/contacts/%s", user.ID),
			ExpectStatus: http.StatusBadRequest,
		},
		{
			AuthHeader:   nil,
			URL:          fmt.Sprintf("/contacts/%s", contact1.ID),
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post(tcase.URL).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < 400 {
				tt.Assert(jsonpath.Equal("$.id", contact1.ID))
				tt.Assert(jsonpath.Equal("$.fullName", contact1.FullName))
				tt.Assert(jsonpath.NotPresent("$.email"))
				tt.Assert(jsonpath.NotPresent("$.token"))
			}

			tt.End()
		})
	}
}

////////////////////////////////////
// DELETE /contacts/{id} Tests
////////////////////////////////////

func TestDeleteContact(t *testing.T) {
	user, _ := _mock.NewUser(_ctx, t)
	contact1, _ := _mock.NewUser(_ctx, t)

	user.AddContact(contact1)

	if _, err := _dbClient.Put(_ctx, user.Key, user); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		URL          string
		ExpectStatus int
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(user.Token),
			URL:          fmt.Sprintf("/contacts/%s", contact1.ID),
			ExpectStatus: http.StatusOK,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(user.Token),
			URL:          fmt.Sprintf("/contacts/%s", contact1.ID),
			ExpectStatus: http.StatusBadRequest,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(contact1.Token),
			URL:          fmt.Sprintf("/contacts/%s", contact1.ID),
			ExpectStatus: http.StatusBadRequest,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(contact1.Token),
			URL:          fmt.Sprintf("/contacts/%s", user.ID),
			ExpectStatus: http.StatusBadRequest,
		},
		{
			AuthHeader:   nil,
			URL:          fmt.Sprintf("/contacts/%s", contact1.ID),
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Delete(tcase.URL).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < 400 {
				tt.Assert(jsonpath.Equal("$.id", contact1.ID))
				tt.Assert(jsonpath.Equal("$.fullName", contact1.FullName))
				tt.Assert(jsonpath.NotPresent("$.email"))
				tt.Assert(jsonpath.NotPresent("$.token"))
			}

			tt.End()
		})
	}
}
