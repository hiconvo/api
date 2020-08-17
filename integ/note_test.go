package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/icrowley/fake"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	"github.com/hiconvo/api/testutil"
)

func TestCreateNote(t *testing.T) {
	u1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       map[string]string
		ExpectStatus    int
	}{
		{
			Name:            "success",
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenBody: map[string]string{
				"name":    fake.Title(),
				"url":     "https://convo.events",
				"favicon": "https://convo.events/favicon.ico",
			},
			ExpectStatus: http.StatusCreated,
		},
		{
			Name:            "success with derive title",
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenBody: map[string]string{
				"body": fake.Paragraph(),
			},
			ExpectStatus: http.StatusCreated,
		},
		{
			Name:            "bad url",
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenBody: map[string]string{
				"name": fake.Title(),
				"url":  "convoevents",
			},
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:            "bad headers",
			GivenAuthHeader: map[string]string{"boop": "beep"},
			GivenBody: map[string]string{
				"name":    fake.Title(),
				"url":     "https://convo.events",
				"favicon": "https://convo.events/favicon.ico",
			},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/notes").
				JSON(tcase.GivenBody).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)
			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.name", tcase.GivenBody["name"]))
				tt.Assert(jsonpath.Equal("$.url", tcase.GivenBody["url"]))
				tt.Assert(jsonpath.Equal("$.favicon", tcase.GivenBody["favicon"]))
				tt.Assert(jsonpath.GreaterThan("$.id", 1))
			}
		})
	}
}

func TestGetNote(t *testing.T) {
	u1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	n1 := testutil.NewNote(_ctx, t, _mongoClient, u1)

	tests := []struct {
		Name            string
		URL             string
		GivenAuthHeader map[string]string
		ExpectStatus    int
	}{
		{
			Name:            "success",
			URL:             fmt.Sprintf("/notes/%s", n1.ID),
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			ExpectStatus:    http.StatusOK,
		},
		{
			Name:            "bad id",
			URL:             fmt.Sprintf("/notes/%s", "random"),
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			ExpectStatus:    http.StatusNotFound,
		},
		{
			Name:            "bad headers",
			GivenAuthHeader: map[string]string{"boop": "beep"},
			ExpectStatus:    http.StatusNotFound,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get(tcase.URL).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)
			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.name", n1.Name))
				tt.Assert(jsonpath.Equal("$.url", n1.URL))
				tt.Assert(jsonpath.Equal("$.favicon", n1.Favicon))
				tt.Assert(jsonpath.Equal("$.id", n1.ID))
			}
		})
	}
}
