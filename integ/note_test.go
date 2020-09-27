package handler_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/icrowley/fake"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	"github.com/hiconvo/api/model"
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
				if tcase.Name != "success with derive title" {
					tt.Assert(jsonpath.Equal("$.name", tcase.GivenBody["name"]))
				}

				tt.Assert(jsonpath.Equal("$.url", tcase.GivenBody["url"]))
				tt.Assert(jsonpath.Equal("$.favicon", tcase.GivenBody["favicon"]))
				tt.Assert(jsonpath.GreaterThan("$.id", 1))
			}
			tt.End()
		})
	}
}

func TestGetNote(t *testing.T) {
	u1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	u2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	n1 := testutil.NewNote(_ctx, t, _dbClient, _searchClient, u1)

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
			URL:             fmt.Sprintf("/notes/%s", n1.ID),
			GivenAuthHeader: map[string]string{"boop": "beep"},
			ExpectStatus:    http.StatusUnauthorized,
		},
		{
			Name:            "wrong person",
			URL:             fmt.Sprintf("/notes/%s", n1.ID),
			GivenAuthHeader: testutil.GetAuthHeader(u2.Token),
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
			tt.End()
		})
	}
}

func TestGetNotes(t *testing.T) {
	u1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	n1 := testutil.NewNote(_ctx, t, _dbClient, _searchClient, u1)
	time.Sleep(time.Millisecond * 100)
	n2 := testutil.NewNote(_ctx, t, _dbClient, _searchClient, u1)
	time.Sleep(time.Millisecond * 100)
	n3 := testutil.NewNote(_ctx, t, _dbClient, _searchClient, u1)

	tests := []struct {
		Name            string
		URL             string
		GivenAuthHeader map[string]string
		ExpectStatus    int
		Check           func(tt *apitest.Response)
	}{
		{
			Name:            "success",
			URL:             "/notes",
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			ExpectStatus:    http.StatusOK,
			Check: func(tt *apitest.Response) {
				for i, n := range []*model.Note{n3, n2, n1} {
					fmt.Printf("%d createdAt: %s", i, n.CreatedAt)
					tt.Assert(jsonpath.Equal(fmt.Sprintf("$.notes[%d].name", i), n.Name))
					tt.Assert(jsonpath.Equal(fmt.Sprintf("$.notes[%d].url", i), n.URL))
					tt.Assert(jsonpath.Equal(fmt.Sprintf("$.notes[%d].favicon", i), n.Favicon))
					tt.Assert(jsonpath.Equal(fmt.Sprintf("$.notes[%d].id", i), n.ID))
				}
			},
		},
		// {
		// 	Name:            "search",
		// 	URL:             fmt.Sprintf("/notes?search=%s", url.QueryEscape(n1.Body)),
		// 	GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
		// 	ExpectStatus:    http.StatusOK,
		// 	Check: func(tt *apitest.Response) {
		// 		tt.Assert(jsonpath.Equal("$.notes[0].name", n1.Name))
		// 		tt.Assert(jsonpath.Equal("$.notes[0].url", n1.URL))
		// 		tt.Assert(jsonpath.Equal("$.notes[0].favicon", n1.Favicon))
		// 		tt.Assert(jsonpath.Equal("$.notes[0].id", n1.ID))
		// 	},
		// },
		{
			Name:            "bad headers",
			URL:             "/notes",
			GivenAuthHeader: map[string]string{"boop": "beep"},
			ExpectStatus:    http.StatusUnauthorized,
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
				tcase.Check(tt)
			}

			tt.End()
		})
	}
}

func TestUpdateNote(t *testing.T) {
	u1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	u2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	n1 := testutil.NewNote(_ctx, t, _dbClient, _searchClient, u1)

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
				"name":    "test update",
				"url":     "https://convo.events",
				"favicon": "https://convo.events/favicon.ico",
			},
			ExpectStatus: http.StatusOK,
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
		{
			Name:            "wrong person",
			GivenAuthHeader: testutil.GetAuthHeader(u2.Token),
			ExpectStatus:    http.StatusNotFound,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Patch(fmt.Sprintf("/notes/%s", n1.ID)).
				JSON(tcase.GivenBody).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)
			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.name", tcase.GivenBody["name"]))
				tt.Assert(jsonpath.Equal("$.url", tcase.GivenBody["url"]))
				tt.Assert(jsonpath.Equal("$.favicon", tcase.GivenBody["favicon"]))
				tt.Assert(jsonpath.Equal("$.id", n1.ID))
			}
			tt.End()
		})
	}
}

func TestDeleteNote(t *testing.T) {
	u1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	u2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	n1 := testutil.NewNote(_ctx, t, _dbClient, _searchClient, u1)

	tests := []struct {
		Name            string
		URL             string
		GivenAuthHeader map[string]string
		ExpectStatus    int
	}{
		{
			Name:            "wrong person",
			URL:             fmt.Sprintf("/notes/%s", n1.ID),
			GivenAuthHeader: testutil.GetAuthHeader(u2.Token),
			ExpectStatus:    http.StatusNotFound,
		},
		{
			Name:            "success",
			URL:             fmt.Sprintf("/notes/%s", n1.ID),
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			ExpectStatus:    http.StatusOK,
		},
		{
			Name:            "deleted",
			URL:             fmt.Sprintf("/notes/%s", n1.ID),
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			ExpectStatus:    http.StatusNotFound,
		},
		{
			Name:            "bad headers",
			URL:             fmt.Sprintf("/notes/%s", n1.ID),
			GivenAuthHeader: map[string]string{"boop": "beep"},
			ExpectStatus:    http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Delete(tcase.URL).
				JSON(`{}`).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)
			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.name", n1.Name))
				tt.Assert(jsonpath.Equal("$.url", n1.URL))
				tt.Assert(jsonpath.Equal("$.favicon", n1.Favicon))
				tt.Assert(jsonpath.Equal("$.id", n1.ID))
			}

			tt.End()
		})
	}
}
