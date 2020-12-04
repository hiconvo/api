package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/icrowley/fake"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	"github.com/stretchr/testify/assert"

	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/testutil"
)

func TestCreateEvent(t *testing.T) {
	u1, _ := _mock.NewUser(_ctx, t)
	u2, _ := _mock.NewUser(_ctx, t)
	u3, _ := _mock.NewUser(_ctx, t)

	tests := []struct {
		Name           string
		AuthHeader     map[string]string
		GivenPayload   map[string]interface{}
		ExpectStatus   int
		ExpectOwnerID  string
		ExpectMemberID string
		ExpectHostID   string
	}{
		{
			Name:       "good payload",
			AuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        fake.Title(),
				"placeId":     fake.CharactersN(32),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": fake.Paragraph(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus:   http.StatusCreated,
			ExpectOwnerID:  u1.ID,
			ExpectMemberID: u2.ID,
		},
		{
			Name:       "good payload with new email",
			AuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        fake.Title(),
				"placeId":     fake.CharactersN(32),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": fake.Paragraph(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
					{
						"email": "test@test.com",
					},
					{
						"email": "test@lksdjidjflskdjf.com  ",
					},
				},
			},
			ExpectStatus:   http.StatusCreated,
			ExpectOwnerID:  u1.ID,
			ExpectMemberID: u2.ID,
		},
		{
			Name:       "good payload with host",
			AuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        fake.Title(),
				"placeId":     fake.CharactersN(32),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": fake.Paragraph(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
					{
						"email": "test@test.com",
					},
				},
				"hosts": []map[string]string{
					{
						"id": u3.ID,
					},
				},
			},
			ExpectStatus:   http.StatusCreated,
			ExpectOwnerID:  u1.ID,
			ExpectMemberID: u2.ID,
			ExpectHostID:   u3.ID,
		},

		{
			Name:       "bad payload",
			AuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        fake.Title(),
				"placeId":     fake.CharactersN(32),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": fake.Paragraph(),
				"users": []map[string]string{
					{
						"id": "Rudolf Carnap",
					},
				},
			},
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:       "bad payload with time in past",
			AuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        fake.Title(),
				"placeId":     fake.CharactersN(32),
				"timestamp":   "2019-09-08T01:19:20.915Z",
				"description": fake.Paragraph(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:       "bad headers",
			AuthHeader: map[string]string{"boop": "beep"},
			GivenPayload: map[string]interface{}{
				"name":        fake.Title(),
				"placeId":     fake.CharactersN(32),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": fake.Paragraph(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/events").
				Headers(tcase.AuthHeader).
				JSON(tcase.GivenPayload).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusOK {
				tt.Assert(jsonpath.Equal("$.owner.id", tcase.ExpectOwnerID))
				tt.Assert(jsonpath.Contains("$.users[*].id", tcase.ExpectMemberID))
				if tcase.ExpectHostID != "" {
					tt.Assert(jsonpath.Contains("$.hosts[*].id", tcase.ExpectHostID))
				}
			}

			tt.End()
		})
	}
}

func TestGetEvents(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member1, _ := _mock.NewUser(_ctx, t)
	member2, _ := _mock.NewUser(_ctx, t)
	host1, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host1}, []*model.User{member1, member2})

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
		IsEventInRes bool
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			IsEventInRes: true,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member1.Token),
			ExpectStatus: http.StatusOK,
			IsEventInRes: true,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member2.Token),
			ExpectStatus: http.StatusOK,
			IsEventInRes: true,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusOK,
			IsEventInRes: false,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
			IsEventInRes: false,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get("/events").
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.IsEventInRes {
				tt.Assert(jsonpath.Equal("$.events[0].name", event.Name))
				tt.Assert(jsonpath.Equal("$.events[0].owner.id", event.Owner.ID))
				tt.Assert(jsonpath.Contains("$.events[0].users[*].id", member1.ID))
				tt.Assert(jsonpath.Contains("$.events[0].users[*].id", member2.ID))
				tt.Assert(jsonpath.Contains("$.events[0].hosts[*].id", host1.ID))
			}

			tt.End()
		})
	}
}

func TestGetEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	host, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host}, []*model.User{member})
	url := fmt.Sprintf("/events/%s", event.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get(url).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.name", event.Name))
				tt.Assert(jsonpath.Equal("$.owner.id", event.Owner.ID))
				tt.Assert(jsonpath.Contains("$.users[*].id", member.ID))
				tt.Assert(jsonpath.Contains("$.hosts[*].id", host.ID))
			}

			tt.End()
		})
	}
}

func TestDeleteEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	host, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host}, []*model.User{member})
	url := fmt.Sprintf("/events/%s", event.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		GivenBody    map[string]interface{}
		ExpectStatus int
		ShouldPass   bool
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
			ShouldPass:   false,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			GivenBody:    map[string]interface{}{"message": "had to cancel"},
			ExpectStatus: http.StatusOK,
			ShouldPass:   true,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			GivenBody:    map[string]interface{}{},
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   true,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Delete(url).
				JSON(tcase.GivenBody).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)
			tt.End()

			if tcase.ShouldPass {
				var gotEvent model.Event
				err := _dbClient.Get(_ctx, event.Key, &gotEvent)
				assert.Equal(t, datastore.ErrNoSuchEntity, err)
			}
		})
	}
}

func TestGetEventMessages(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member1, _ := _mock.NewUser(_ctx, t)
	member2, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member1, member2})
	message1 := _mock.NewEventMessage(_ctx, t, owner, event)
	message2 := _mock.NewEventMessage(_ctx, t, owner, event)
	url := fmt.Sprintf("/events/%s/messages", event.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
	}{
		// Owner can get messages
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
		},
		// Member can get messages
		{
			AuthHeader:   testutil.GetAuthHeader(member1.Token),
			ExpectStatus: http.StatusOK,
		},
		// NonMember cannot get messages
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
		},
		// Unauthenticated user cannot get messages
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get(url).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.messages[1].id", message1.ID))
				tt.Assert(jsonpath.Equal("$.messages[1].body", message1.Body))
				tt.Assert(jsonpath.Equal("$.messages[0].id", message2.ID))
				tt.Assert(jsonpath.Equal("$.messages[0].body", message2.Body))
			}

			tt.End()
		})
	}
}

func TestAddMessageToEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member1, _ := _mock.NewUser(_ctx, t)
	member2, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member1, member2})
	url := fmt.Sprintf("/events/%s/messages", event.ID)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       string
		GivenAuthor     *model.User
		ExpectCode      int
		ExpectBody      string
		ExpectPhoto     bool
	}{
		// Owner
		{
			GivenAuthHeader: testutil.GetAuthHeader(owner.Token),
			GivenAuthor:     owner,
			GivenBody:       `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q==", "body": "hello"}`,
			ExpectCode:      http.StatusCreated,
			ExpectBody:      "hello",
			ExpectPhoto:     true,
		},
		// Member
		{
			GivenAuthHeader: testutil.GetAuthHeader(member1.Token),
			GivenAuthor:     member1,
			GivenBody:       `{"body": "hello"}`,
			ExpectCode:      http.StatusCreated,
			ExpectBody:      "hello",
			ExpectPhoto:     false,
		},
		// NonMember
		{
			GivenAuthHeader: testutil.GetAuthHeader(nonmember.Token),
			GivenAuthor:     nonmember,
			GivenBody:       `{"body": "hello"}`,
			ExpectCode:      http.StatusNotFound,
			ExpectPhoto:     false,
		},
		// EmptyPayload
		{
			GivenAuthHeader: testutil.GetAuthHeader(member1.Token),
			GivenAuthor:     member1,
			GivenBody:       `{}`,
			ExpectCode:      http.StatusBadRequest,
			ExpectPhoto:     false,
		},
		{
			GivenAuthHeader: testutil.GetAuthHeader(owner.Token),
			GivenAuthor:     owner,
			GivenBody:       `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q=="}`,
			ExpectCode:      http.StatusBadRequest,
			ExpectPhoto:     false,
		},
	}

	for _, tcase := range tests {
		tt := apitest.New(tcase.Name).
			Handler(_handler).
			Post(url).
			JSON(tcase.GivenBody).
			Headers(tcase.GivenAuthHeader).
			Expect(t).
			Status(tcase.ExpectCode)

		if tcase.ExpectCode < 300 {
			tt.
				Assert(jsonpath.Equal("$.parentId", event.ID)).
				Assert(jsonpath.Equal("$.body", tcase.ExpectBody)).
				Assert(jsonpath.Equal("$.user.fullName", tcase.GivenAuthor.FullName)).
				Assert(jsonpath.Equal("$.user.id", tcase.GivenAuthor.ID))
			if tcase.ExpectPhoto {
				tt.Assert(jsonpath.Present("$.photos[0]"))
			} else {
				tt.Assert(jsonpath.NotPresent("$.photos[0]"))
			}
		}

		tt.End()
	}
}

func TestDeleteEventMessage(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member1, _ := _mock.NewUser(_ctx, t)
	member2, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member1, member2})
	message1 := _mock.NewEventMessage(_ctx, t, owner, event)
	message2 := _mock.NewEventMessage(_ctx, t, member1, event)

	// We reduce the time resolution because the test database does not
	// store it with native resolution. When the time is retrieved
	// from the database, the result of json marshaling is a couple fewer
	// digits than marshaling the native time without truncating, which
	// causes the tests to fail.
	message2.CreatedAt = message2.CreatedAt.Truncate(time.Microsecond)
	message2encoded, err := json.Marshal(message2)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenMessageID  string
		ExpectCode      int
		ExpectBody      string
	}{
		{
			Name:            "member attempt to delete message he does not own",
			GivenAuthHeader: testutil.GetAuthHeader(member1.Token),
			GivenMessageID:  message1.ID,
			ExpectCode:      http.StatusNotFound,
			ExpectBody:      `{"message":"The requested resource was not found"}`,
		},
		{
			Name:            "nonmember attempt to delete message",
			GivenAuthHeader: testutil.GetAuthHeader(nonmember.Token),
			GivenMessageID:  message1.ID,
			ExpectCode:      http.StatusNotFound,
			ExpectBody:      `{"message":"The requested resource was not found"}`,
		},
		{
			Name:            "success",
			GivenAuthHeader: testutil.GetAuthHeader(member1.Token),
			GivenMessageID:  message2.ID,
			ExpectCode:      http.StatusOK,
			ExpectBody:      string(message2encoded),
		},
	}

	for _, tcase := range tests {
		apitest.New(tcase.Name).
			Handler(_handler).
			Delete(fmt.Sprintf("/events/%s/messages/%s", event.ID, tcase.GivenMessageID)).
			JSON(`{}`).
			Headers(tcase.GivenAuthHeader).
			Expect(t).
			Status(tcase.ExpectCode).
			Body(tcase.ExpectBody).
			End()

		if tcase.ExpectCode == http.StatusOK {
			var gotMessage model.Message
			err := _dbClient.Get(_ctx, message2.Key, &gotMessage)
			assert.Equal(t, datastore.ErrNoSuchEntity, err)
		}
	}
}

func TestMarkEventAsRead(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member1, _ := _mock.NewUser(_ctx, t)
	member2, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member1, member2})
	_mock.NewEventMessage(_ctx, t, owner, event)
	_mock.NewEventMessage(_ctx, t, member1, event)
	url := fmt.Sprintf("/events/%s/reads", event.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member1.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post(url).
				Header("Content-Type", "application/json").
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", event.ID))
				tt.Assert(jsonpath.Equal("$.reads[0].id", owner.ID))
			}

			tt.End()
		})
	}
}

func TestGetMagicLink(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	host, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host}, []*model.User{member})

	tests := []struct {
		Name         string
		AuthToken    string
		ExpectStatus int
	}{
		{
			Name:         "Owner",
			AuthToken:    owner.Token,
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Host",
			AuthToken:    host.Token,
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Guest",
			AuthToken:    member.Token,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "Random",
			AuthToken:    nonmember.Token,
			ExpectStatus: http.StatusNotFound,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			apitest.New(tcase.Name).
				Handler(_handler).
				Get(fmt.Sprintf("/events/%s/magic", event.ID)).
				Headers(testutil.GetAuthHeader(tcase.AuthToken)).
				Expect(t).
				Status(tcase.ExpectStatus).
				End()
		})
	}
}

func TestUpdateEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	host, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host}, []*model.User{member})
	url := fmt.Sprintf("/events/%s", event.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
		ShouldPass   bool
		GivenBody    map[string]interface{}
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			ShouldPass:   true,
			GivenBody:    map[string]interface{}{"name": "Ruth Marcus"},
		},
		{
			AuthHeader:   testutil.GetAuthHeader(host.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
			GivenBody:    map[string]interface{}{"name": "Ruth Marcus"},
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
			GivenBody:    map[string]interface{}{"name": "Ruth Marcus"},
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
			ShouldPass:   false,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Patch(url).
				JSON(tcase.GivenBody).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus <= http.StatusBadRequest {
				if tcase.ShouldPass {
					tt.Assert(jsonpath.Equal("$.name", tcase.GivenBody["name"]))
				} else {
					tt.Assert(jsonpath.Equal("$.name", event.Name))
				}
			}

			tt.End()
		})
	}
}

func TestAddUserToEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	host, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	memberToAdd, _ := _mock.NewUser(_ctx, t)
	secondMemberToAdd, _ := _mock.NewUser(_ctx, t)
	thridMemberToAdd, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host}, []*model.User{member})

	eventAllowGuests := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member})
	eventAllowGuests.GuestsCanInvite = true
	if _, err := _dbClient.Put(_ctx, eventAllowGuests.Key, eventAllowGuests); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
		GivenUserID  string
		GivenEventID string
		ShouldPass   bool
		ExpectNames  []string
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
			GivenUserID:  memberToAdd.ID,
			GivenEventID: event.ID,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusNotFound,
			GivenUserID:  memberToAdd.ID,
			GivenEventID: event.ID,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
			GivenUserID:  memberToAdd.ID,
			GivenEventID: event.ID,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  memberToAdd.ID,
			ExpectNames:  []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName},
			GivenEventID: event.ID,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  "addedOnTheFly@againanothertime.com",
			ExpectNames:  []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName, "addedonthefly"},
			GivenEventID: event.ID,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  secondMemberToAdd.Email,
			ExpectNames:  []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName, "addedonthefly", secondMemberToAdd.FullName},
			GivenEventID: event.ID,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(host.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  thridMemberToAdd.ID,
			ExpectNames:  []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName, "addedonthefly", secondMemberToAdd.FullName, thridMemberToAdd.FullName},
			GivenEventID: event.ID,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
			GivenUserID:  memberToAdd.ID,
			GivenEventID: eventAllowGuests.ID,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  memberToAdd.ID,
			ExpectNames:  []string{owner.FullName, member.FullName, memberToAdd.FullName},
			GivenEventID: eventAllowGuests.ID,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post(fmt.Sprintf("/events/%s/users/%s", tcase.GivenEventID, tcase.GivenUserID)).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus <= http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", tcase.GivenEventID))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.owner.fullName", owner.FullName))

				for _, name := range tcase.ExpectNames {
					tt.Assert(jsonpath.Contains("$.users[*].fullName", name))
				}
			}

			tt.End()
		})
	}
}

func TestRemoveUserFromEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	memberToRemove, _ := _mock.NewUser(_ctx, t)
	memberToLeave, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member, memberToRemove, memberToLeave})

	tests := []struct {
		Name              string
		AuthHeader        map[string]string
		GivenUserID       string
		ExpectStatus      int
		ExpectMemberIDs   []string
		ExpectMemberNames []string
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			GivenUserID:  member.ID,
			ExpectStatus: http.StatusNotFound,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			GivenUserID:  memberToRemove.ID,
			ExpectStatus: http.StatusNotFound,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			GivenUserID:  member.ID,
			ExpectStatus: http.StatusUnauthorized,
		},
		{
			AuthHeader:        testutil.GetAuthHeader(owner.Token),
			GivenUserID:       memberToRemove.ID,
			ExpectStatus:      http.StatusOK,
			ExpectMemberIDs:   []string{owner.ID, member.ID, memberToLeave.ID},
			ExpectMemberNames: []string{owner.FullName, member.FullName, memberToLeave.FullName},
		},
		{
			AuthHeader:        testutil.GetAuthHeader(memberToLeave.Token),
			GivenUserID:       memberToLeave.ID,
			ExpectStatus:      http.StatusOK,
			ExpectMemberIDs:   []string{owner.ID, member.ID},
			ExpectMemberNames: []string{owner.FullName, member.FullName},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Delete(fmt.Sprintf("/events/%s/users/%s", event.ID, tcase.GivenUserID)).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", event.ID))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.owner.fullName", owner.FullName))

				for _, name := range tcase.ExpectMemberNames {
					tt.Assert(jsonpath.Contains("$.users[*].fullName", name))
				}

				for _, id := range tcase.ExpectMemberIDs {
					tt.Assert(jsonpath.Contains("$.users[*].id", id))
				}

				tt.Assert(jsonpath.Len("$.users", len(tcase.ExpectMemberIDs)))
			}

			tt.End()
		})
	}
}

func TestAddRSVPToEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member})
	url := fmt.Sprintf("/events/%s/rsvps", event.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusBadRequest,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusOK,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post(url).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", event.ID))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.owner.fullName", owner.FullName))
				tt.Assert(jsonpath.Contains("$.rsvps[*].id", member.ID))
				tt.Assert(jsonpath.Contains("$.rsvps[*].fullName", member.FullName))
			}

			tt.End()
		})
	}
}

func TestRemoveRSVPFromEvent(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	memberToRemove, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member, memberToRemove})

	if err := event.AddRSVP(memberToRemove); err != nil {
		t.Fatal(err)
	}

	if err := event.AddRSVP(member); err != nil {
		t.Fatal(err)
	}

	if _, err := _dbClient.Put(_ctx, event.Key, event); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Name              string
		AuthHeader        map[string]string
		ExpectStatus      int
		ExpectMemberIDs   []string
		ExpectMemberNames []string
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusBadRequest,
		},
		{
			AuthHeader:        testutil.GetAuthHeader(memberToRemove.Token),
			ExpectStatus:      http.StatusOK,
			ExpectMemberIDs:   []string{member.ID},
			ExpectMemberNames: []string{member.FullName},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Delete(fmt.Sprintf("/events/%s/rsvps", event.ID)).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", event.ID))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.owner.fullName", owner.FullName))

				for _, name := range tcase.ExpectMemberNames {
					tt.Assert(jsonpath.Contains("$.rsvps[*].fullName", name))
				}

				for _, id := range tcase.ExpectMemberIDs {
					tt.Assert(jsonpath.Contains("$.rsvps[*].id", id))
				}

				tt.Assert(jsonpath.Len("$.rsvps", len(tcase.ExpectMemberIDs)))
			}

			tt.End()
		})
	}
}

func TestMagicInvite(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	host, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host}, []*model.User{member})
	event2 := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member})

	magicClient := magic.NewClient("")
	magicLink := event.GetInviteMagicLink(magicClient)
	eventID, b64ts, sig := testutil.GetMagicLinkParts(magicLink)

	tests := []struct {
		Name         string
		EventID      string
		AuthToken    string
		ExpectStatus int
	}{
		{
			Name:         "Owner",
			EventID:      event.ID,
			AuthToken:    owner.Token,
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:         "Host",
			EventID:      event.ID,
			AuthToken:    host.Token,
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:         "Random",
			EventID:      event.ID,
			AuthToken:    nonmember.Token,
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Unrelated Event",
			EventID:      event2.ID,
			AuthToken:    nonmember.Token,
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			tt := apitest.New(testCase.Name).
				Handler(_handler).
				Post(fmt.Sprintf("/events/%s/magic", testCase.EventID)).
				JSON(fmt.Sprintf(`{"eventId": "%s", "signature": "%s", "timestamp": "%s"}`, eventID, sig, b64ts)).
				Headers(testutil.GetAuthHeader(testCase.AuthToken)).
				Expect(t).
				Status(testCase.ExpectStatus)
			if testCase.ExpectStatus == http.StatusOK {
				tt.Assert(jsonpath.Equal("$.id", event.ID))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Contains("$.users[*].id", nonmember.ID))
			}
			tt.End()
		})
	}
}

func TestRollMagicLink(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	host, _ := _mock.NewUser(_ctx, t)
	member, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{host}, []*model.User{member})

	tests := []struct {
		Name         string
		AuthToken    string
		ExpectStatus int
	}{
		{
			Name:         "Owner",
			AuthToken:    owner.Token,
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Host",
			AuthToken:    host.Token,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "Guest",
			AuthToken:    member.Token,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "Random",
			AuthToken:    nonmember.Token,
			ExpectStatus: http.StatusNotFound,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			apitest.New(testCase.Name).
				Handler(_handler).
				Delete(fmt.Sprintf("/events/%s/magic", event.ID)).
				JSON("{}").
				Headers(testutil.GetAuthHeader(testCase.AuthToken)).
				Expect(t).
				Status(testCase.ExpectStatus).
				End()
		})
	}
}

func TestMagicRSVP(t *testing.T) {
	owner, _ := _mock.NewUser(_ctx, t)
	member1, _ := _mock.NewUser(_ctx, t)
	member2, _ := _mock.NewUser(_ctx, t)
	nonmember, _ := _mock.NewUser(_ctx, t)
	event := _mock.NewEvent(_ctx, t, owner, []*model.User{}, []*model.User{member1, member2})

	link := event.GetRSVPMagicLink(magic.NewClient(""), member1)
	kenc, b64ts, sig := testutil.GetMagicLinkParts(link)

	link2 := event.GetRSVPMagicLink(magic.NewClient(""), member2)
	kenc2, b64ts2, sig2 := testutil.GetMagicLinkParts(link2)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectData   map[string]interface{}
		ExpectError  string
	}{
		{
			GivenBody: map[string]interface{}{
				"signature": sig,
				"timestamp": b64ts,
				"userID":    kenc,
				"eventID":   event.ID,
			},
			ExpectStatus: http.StatusOK,
			ExpectData: map[string]interface{}{
				"id":        member1.ID,
				"firstName": member1.FirstName,
				"lastName":  member1.LastName,
				"token":     member1.Token,
				"verified":  true,
				"email":     member1.Email,
			},
		},
		{
			GivenBody: map[string]interface{}{
				"signature": sig,
				"timestamp": b64ts,
				"userID":    kenc,
				"eventID":   event.ID,
			},
			ExpectStatus: http.StatusOK,
			ExpectData: map[string]interface{}{
				"id":        member1.ID,
				"firstName": member1.FirstName,
				"lastName":  member1.LastName,
				"token":     member1.Token,
				"verified":  true,
				"email":     member1.Email,
			},
		},
		{
			GivenBody: map[string]interface{}{
				"signature": "not a valid signature",
				"timestamp": b64ts2,
				"userID":    kenc2,
				"eventID":   event.ID,
			},
			ExpectStatus: http.StatusUnauthorized,
			ExpectError:  `{"message":"Unauthorized"}`,
		},
		{
			GivenBody: map[string]interface{}{
				"signature": sig2,
				"timestamp": b64ts2,
				"userID":    nonmember.ID,
				"eventID":   event.ID,
			},
			ExpectStatus: http.StatusUnauthorized,
			ExpectError:  `{"message":"Unauthorized"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/events/rsvps").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectError)
			} else {
				tt.Assert(jsonpath.Equal("$.id", member1.ID))
				tt.Assert(jsonpath.Equal("$.firstName", member1.FirstName))
				tt.Assert(jsonpath.Equal("$.lastName", member1.LastName))
				tt.Assert(jsonpath.Equal("$.fullName", member1.FullName))
				tt.Assert(jsonpath.Equal("$.token", member1.Token))
				tt.Assert(jsonpath.Equal("$.verified", member1.Verified))
				tt.Assert(jsonpath.Equal("$.email", member1.Email))
			}

			tt.End()
		})
	}
}
