package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	"github.com/hiconvo/api/clients/magic"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/testutil"
)

func TestCreateUser(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	incompleteUser := testutil.NewIncompleteUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name         string
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}{
		{
			Name: "success",
			GivenBody: map[string]interface{}{
				"email":     "ruth.marcus@yale.edu",
				"firstName": "Ruth",
				"lastName":  "Marcus",
				"password":  "the comma is a giveaway",
			},
			ExpectStatus: http.StatusCreated,
			ExpectBody:   "",
		},
		{
			Name: "need to verify",
			GivenBody: map[string]interface{}{
				"email":     incompleteUser.Email,
				"firstName": "Thomas",
				"lastName":  "Aquinas",
				"password":  "angels are real!",
			},
			ExpectStatus: http.StatusOK,
			ExpectBody:   `{"message": "Please verify your email to proceed"}`,
		},
		{
			Name: "missing name and password",
			GivenBody: map[string]interface{}{
				"email":    "rudolf.carnap@charles.cz",
				"lastName": "Carnap",
				"password": "",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"firstName":"This field is required","password":"Must be at least 8 characters long"}`,
		},
		{
			Name: "type mismatch",
			GivenBody: map[string]interface{}{
				"email":     "kit.fine@nyu.edu",
				"firstName": true,
				"password":  "Reality is constituted by tensed facts",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Could not decode JSON"}`,
		},
		{
			Name: "already registered",
			GivenBody: map[string]interface{}{
				"email":     existingUser.Email,
				"firstName": "Ruth",
				"lastName":  "Millikan",
				"password":  "Language and thought are biological categories",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"This email has already been registered"}`,
		},
		{
			Name: "invalid email",
			GivenBody: map[string]interface{}{
				"email":     "it's all in my mind",
				"firstName": "George",
				"lastName":  "Berkeley",
				"password":  "Ordinary objects are ideas",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"email":"This is not a valid email"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/users").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus == http.StatusOK {
				tt.Body(tcase.ExpectBody)
			} else if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.email", tcase.GivenBody["email"].(string))).
					Assert(jsonpath.Equal("$.firstName", tcase.GivenBody["firstName"].(string))).
					Assert(jsonpath.Equal("$.lastName", tcase.GivenBody["lastName"].(string)))
			} else {
				tt.Body(tcase.ExpectBody)
			}

			tt.End()
		})
	}
}

func TestAuthenticateUser(t *testing.T) {
	existingUser, password := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name         string
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}{
		{
			Name: "success",
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": password,
			},
			ExpectStatus: http.StatusOK,
		},
		{
			Name: "invalid password",
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "123456789",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Invalid credentials"}`,
		},
		{
			Name: "missing password",
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"password":"This field is required"}`,
		},
		{
			Name: "invalid password again",
			GivenBody: map[string]interface{}{
				"email":    "santa@northpole.com",
				"password": "have you been naughty or nice?",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Invalid credentials"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/users/auth").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", existingUser.ID))
				tt.Assert(jsonpath.Equal("$.firstName", existingUser.FirstName))
				tt.Assert(jsonpath.Equal("$.lastName", existingUser.LastName))
				tt.Assert(jsonpath.Equal("$.fullName", existingUser.FullName))
				tt.Assert(jsonpath.Equal("$.token", existingUser.Token))
				tt.Assert(jsonpath.Equal("$.verified", existingUser.Verified))
				tt.Assert(jsonpath.Equal("$.email", existingUser.Email))
			}

			tt.End()
		})
	}
}

func TestGetCurrentUser(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		ExpectStatus    int
		ExpectBody      string
	}{
		{
			Name:            "success",
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			ExpectStatus:    http.StatusOK,
		},
		{
			Name:            "bad token",
			GivenAuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			Name:            "invalid header",
			GivenAuthHeader: map[string]string{"everything": "is what it is"},
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			Name:            "missing header",
			GivenAuthHeader: nil,
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get("/users").
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", existingUser.ID))
				tt.Assert(jsonpath.Equal("$.firstName", existingUser.FirstName))
				tt.Assert(jsonpath.Equal("$.lastName", existingUser.LastName))
				tt.Assert(jsonpath.Equal("$.token", existingUser.Token))
				tt.Assert(jsonpath.Equal("$.verified", existingUser.Verified))
				tt.Assert(jsonpath.Equal("$.email", existingUser.Email))
			}

			tt.End()
		})
	}
}

func TestGetUser(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	user1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		URL             string
		ExpectStatus    int
		ExpectBody      string
	}{
		{
			Name:            "success",
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:             fmt.Sprintf("/users/%s", user1.ID),
			ExpectStatus:    http.StatusOK,
		},
		{
			Name:            "bad token",
			GivenAuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			URL:             fmt.Sprintf("/users/%s", user1.ID),
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			Name:            "bad url",
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:             fmt.Sprintf("/users/%s", "somenonsense"),
			ExpectStatus:    http.StatusNotFound,
			ExpectBody:      `{"message":"The requested resource was not found"}`,
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

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", user1.ID))
				tt.Assert(jsonpath.Equal("$.firstName", user1.FirstName))
				tt.Assert(jsonpath.Equal("$.lastName", user1.LastName))
				tt.Assert(jsonpath.Equal("$.fullName", user1.FullName))
				tt.Assert(jsonpath.NotPresent("$.token"))
				tt.Assert(jsonpath.NotPresent("$.verified"))
				tt.Assert(jsonpath.NotPresent("$.email"))
			}

			tt.End()
		})
	}
}

func TestOAuth(t *testing.T) {
	existingUser1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2.PasswordDigest = ""
	existingUser2.Verified = false
	if _, err := _dbClient.Put(_ctx, existingUser2.Key, existingUser2); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Name            string
		GivenBody       string
		GivenOAuthToken string
		GivenEmail      string
		ExpectStatus    int
		ExpectFirstName string
		ExpectLastName  string
		Token           string
	}{
		{
			Name:            "success",
			GivenOAuthToken: "123",
			GivenEmail:      "bob.kennedy@whitehouse.gov",
			GivenBody:       `{"provider": "google", "token": "123"}`,
			ExpectStatus:    200,
			ExpectFirstName: "John",
			ExpectLastName:  "Kennedy",
		},
		{
			Name:            "success",
			GivenOAuthToken: "123",
			GivenEmail:      "bob.kennedy@whitehouse.gov",
			GivenBody:       `{"provider": "google", "token": "123"}`,
			ExpectStatus:    200,
			ExpectFirstName: "John",
			ExpectLastName:  "Kennedy",
		},
		{
			Name:            "success with existing user",
			GivenOAuthToken: "456",
			GivenEmail:      existingUser1.Email,
			GivenBody:       `{"provider": "google", "token": "456"}`,
			ExpectStatus:    200,
			ExpectFirstName: existingUser1.FirstName,
			ExpectLastName:  existingUser1.LastName,
		},
		{
			Name:            "success and merge with existing user",
			GivenOAuthToken: "789",
			GivenEmail:      "merge@me.com",
			GivenBody:       `{"provider": "google", "token": "789"}`,
			ExpectStatus:    200,
			ExpectFirstName: existingUser2.FirstName,
			ExpectLastName:  existingUser2.LastName,
			Token:           existingUser2.Token,
		},
		{
			Name:            "invalid token",
			GivenOAuthToken: "789",
			GivenEmail:      "merge@me.com",
			GivenBody:       `{"provider": "notvalid", "token": "notvalid"}`,
			ExpectStatus:    400,
			Token:           existingUser2.Token,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			oauthMock := apitest.NewMock().
				Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", tcase.GivenOAuthToken)).
				RespondWith().
				Body(fmt.Sprintf(`{
					"aud": "",
					"sub": "%s",
					"email": "%s",
					"given_name": "%s",
					"family_name": "%s",
					"picture": ""
				}`, tcase.GivenEmail, tcase.GivenEmail, tcase.ExpectFirstName, tcase.ExpectLastName)).
				Status(200).
				End()

			headers := map[string]string{"Content-Type": "application/json"}

			if tcase.Token != "" {
				headers["Authorization"] = fmt.Sprintf("Bearer %s", tcase.Token)
			}

			tt := apitest.New("OAuth").
				Mocks(oauthMock).
				Handler(_handler).
				Post("/users/oauth").
				Headers(headers).
				Body(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)
			if tcase.ExpectStatus < 300 {
				tt.Assert(jsonpath.Equal("$.email", tcase.GivenEmail))
				tt.Assert(jsonpath.Equal("$.firstName", tcase.ExpectFirstName))
				tt.Assert(jsonpath.Equal("$.lastName", tcase.ExpectLastName))
			}

			tt.End()

		})
	}
}

func TestUpdatePassword(t *testing.T) {
	magicClient := magic.NewClient("")

	existingUser1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	kenc, b64ts, sig := testutil.GetMagicLinkParts(existingUser1.GetPasswordResetMagicLink(magicClient))

	existingUser2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	kenc2, b64ts2, _ := testutil.GetMagicLinkParts(existingUser2.GetPasswordResetMagicLink(magicClient))

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
		OutData         map[string]interface{}
		ExpectBody      string
	}{
		{
			GivenBody: map[string]interface{}{
				"signature": sig,
				"timestamp": b64ts,
				"userID":    kenc,
				"password":  "12345678",
			},
			ExpectStatus: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser1.ID,
				"firstName": existingUser1.FirstName,
				"lastName":  existingUser1.LastName,
				"token":     existingUser1.Token,
				"verified":  existingUser1.Verified,
				"email":     existingUser1.Email,
			},
		},
		{
			GivenBody: map[string]interface{}{
				"signature": sig,
				"timestamp": b64ts,
				"userID":    kenc,
				"password":  "12345678",
			},
			ExpectStatus: http.StatusUnauthorized,
			ExpectBody:   `{"message":"Unauthorized"}`,
		},
		{
			GivenBody: map[string]interface{}{
				"signature": "not a valid signature",
				"timestamp": b64ts2,
				"userID":    kenc2,
				"password":  "12345678",
			},
			ExpectStatus: http.StatusUnauthorized,
			ExpectBody:   `{"message":"Unauthorized"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/users/password").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", tcase.OutData["id"]))
				tt.Assert(jsonpath.Equal("$.firstName", tcase.OutData["firstName"]))
				tt.Assert(jsonpath.Equal("$.lastName", tcase.OutData["lastName"]))
				tt.Assert(jsonpath.Equal("$.token", tcase.OutData["token"]))
				tt.Assert(jsonpath.Equal("$.verified", tcase.OutData["verified"]))
				tt.Assert(jsonpath.Equal("$.email", tcase.OutData["email"]))
			}

			tt.End()
		})
	}
}

func TestVerifyEmail(t *testing.T) {
	magicClient := magic.NewClient("")
	us := testutil.NewUserStore(_ctx, t, _dbClient, _searchClient)
	ms := testutil.NewMessageStore(_ctx, t, _dbClient)
	es := testutil.NewEventStore(_ctx, t, _dbClient)
	ts := testutil.NewThreadStore(_ctx, t, _dbClient)

	// Standard case of verifying email after account creation
	existingUser1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser1.Emails = []string{}
	existingUser1.Verified = false
	_dbClient.Put(_ctx, existingUser1.Key, existingUser1)
	kenc1, b64ts1, sig1 := testutil.GetMagicLinkParts(existingUser1.GetVerifyEmailMagicLink(magicClient, existingUser1.Email))

	// Bad signature case
	existingUser2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	kenc2, b64ts2, _ := testutil.GetMagicLinkParts(existingUser2.GetVerifyEmailMagicLink(magicClient, existingUser2.Email))

	// Adding a new email
	existingUser3, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	kenc3, b64ts3, sig3 := testutil.GetMagicLinkParts(existingUser3.GetVerifyEmailMagicLink(magicClient, "new@email.com"))

	// Merging accounts
	existingUser4, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient) // User to merge into
	existingUser5, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient) // User to be merged
	// Assign test event, thread, and messages to user to be merged
	event := testutil.NewEvent(_ctx, t, _dbClient, existingUser2, []*model.User{}, []*model.User{existingUser5})
	eventMessage := testutil.NewEventMessage(_ctx, t, _dbClient, existingUser5, event)
	thread := testutil.NewThread(_ctx, t, _dbClient, existingUser5, []*model.User{existingUser4, existingUser3})
	threadMessage := testutil.NewThreadMessage(_ctx, t, _dbClient, existingUser5, thread)
	// Add reference to user to be merged in existingUser2's contacts
	existingUser2.AddContact(existingUser5)
	if err := us.Commit(_ctx, existingUser2); err != nil {
		t.Error(err)
	}
	kenc4, b64ts4, sig4 := testutil.GetMagicLinkParts(existingUser4.GetVerifyEmailMagicLink(magicClient, existingUser5.Email))

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
		OutData         map[string]interface{}
		ExpectBody      string
		VerifyFunc      func() bool
	}{
		// Standard case
		{
			Name: "success",
			GivenBody: map[string]interface{}{
				"signature": sig1,
				"timestamp": b64ts1,
				"userID":    kenc1,
				"email":     existingUser1.Email,
			},
			ExpectStatus: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser1.ID,
				"firstName": existingUser1.FirstName,
				"lastName":  existingUser1.LastName,
				"token":     existingUser1.Token,
				"verified":  true,
				"email":     existingUser1.Email,
				"emails":    []string{existingUser1.Email},
			},
			VerifyFunc: func() bool { return true },
		},

		// Already used magic link (applying standard case second time)
		{
			Name: "already used magic link",
			GivenBody: map[string]interface{}{
				"signature": sig1,
				"timestamp": b64ts1,
				"userID":    kenc1,
				"email":     existingUser1.Email,
			},
			ExpectStatus: http.StatusUnauthorized,
			ExpectBody:   `{"message":"Unauthorized"}`,
		},

		// Bad signature
		{
			Name: "bad signature",
			GivenBody: map[string]interface{}{
				"signature": "not a valid signature",
				"timestamp": b64ts2,
				"userID":    kenc2,
				"email":     existingUser2.Email,
			},
			ExpectStatus: http.StatusUnauthorized,
			ExpectBody:   `{"message":"Unauthorized"}`,
			VerifyFunc:   func() bool { return true },
		},

		// Adding an email
		{
			Name: "adding email",
			GivenBody: map[string]interface{}{
				"signature": sig3,
				"timestamp": b64ts3,
				"userID":    kenc3,
				"email":     "new@email.com",
			},
			ExpectStatus: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser3.ID,
				"firstName": existingUser3.FirstName,
				"lastName":  existingUser3.LastName,
				"token":     existingUser3.Token,
				"verified":  true,
				"email":     existingUser3.Email,
				"emails":    []string{existingUser3.Email, "new@email.com"},
			},
			VerifyFunc: func() bool { return true },
		},

		// Merging accounts
		{
			Name: "merging accounts",
			GivenBody: map[string]interface{}{
				"signature": sig4,
				"timestamp": b64ts4,
				"userID":    kenc4,
				"email":     existingUser5.Email,
			},
			ExpectStatus: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser4.ID,
				"firstName": existingUser4.FirstName,
				"lastName":  existingUser4.LastName,
				"token":     existingUser4.Token,
				"verified":  true,
				"email":     existingUser4.Email,
				"emails":    []string{existingUser4.Email, existingUser5.Email},
			},
			VerifyFunc: func() bool {
				// Make sure that existingUser5 was deleted
				_, err := us.GetUserByID(_ctx, existingUser5.ID)
				if err == nil {
					return false
				}

				// Make sure that existingUser5's events were transfered to
				// existingUser4
				events, err := es.GetEventsByUser(_ctx, existingUser4, &model.Pagination{Size: -1})
				if err != nil {
					return false
				}

				found := false
				for i := range events {
					if events[i].ID == event.ID {
						found = true
					}
				}
				if !found {
					return false
				}

				// Make sure that existingUser5's threads were transfered to
				// existingUser4
				threads, err := ts.GetThreadsByUser(_ctx, existingUser4, &model.Pagination{Size: -1})
				if err != nil {
					return false
				}

				found = false
				for i := range threads {
					if threads[i].ID == thread.ID {
						found = true
					}
				}
				if !found {
					return false
				}

				// Make sure that existingUser5's messages were transferred too
				messages, err := ms.GetUnhydratedMessagesByUser(_ctx, existingUser4, &model.Pagination{Size: -1})
				if err != nil {
					return false
				}

				foundEventMessage := false
				fountThreadMessage := false
				for i := range messages {
					if messages[i].Key.Equal(eventMessage.Key) {
						foundEventMessage = true
					}
					if messages[i].Key.Equal(threadMessage.Key) {
						fountThreadMessage = true
					}
				}
				if !foundEventMessage || !fountThreadMessage {
					return false
				}

				// Make sure that existingUser2's contacts were updated
				refreshedExistingUser2, err := us.GetUserByID(_ctx, existingUser2.ID)
				if err != nil {
					return false
				}
				contacts, err := us.GetContactsByUser(_ctx, refreshedExistingUser2)
				if err != nil {
					return false
				}

				found = false
				for i := range contacts {
					if contacts[i].ID == existingUser4.ID {
						found = true
					}
				}
				if !found {
					return false
				}

				return true
			},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/users/verify").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", tcase.OutData["id"]))
				tt.Assert(jsonpath.Equal("$.firstName", tcase.OutData["firstName"]))
				tt.Assert(jsonpath.Equal("$.lastName", tcase.OutData["lastName"]))
				tt.Assert(jsonpath.Equal("$.token", tcase.OutData["token"]))
				tt.Assert(jsonpath.Equal("$.verified", tcase.OutData["verified"]))
				tt.Assert(jsonpath.Equal("$.email", tcase.OutData["email"]))
				for _, email := range tcase.OutData["emails"].([]string) {
					tt.Assert(jsonpath.Contains("$.emails", email))
				}
			}

			tt.End()

			if tcase.VerifyFunc != nil && !tcase.VerifyFunc() {
				t.Errorf("Custom verifier failed")
			}
		})
	}
}

func TestForgotPassword(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name         string
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}{
		{
			GivenBody: map[string]interface{}{
				"email": existingUser.Email,
			},
			ExpectStatus: http.StatusOK,
			ExpectBody:   `{"message":"Check your email for a link to reset your password"}`,
		},
		{
			GivenBody: map[string]interface{}{
				"email": "plato@greece.edu",
			},
			ExpectStatus: http.StatusOK,
			ExpectBody:   `{"message":"Check your email for a link to reset your password"}`,
		},
		{
			GivenBody: map[string]interface{}{
				"email": "asdf",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"email":"This is not a valid email"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			apitest.New("forgot").
				Handler(_handler).
				Post("/users/forgot").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus).
				Body(tcase.ExpectBody).
				End()
		})
	}
}

func TestMagicLogin(t *testing.T) {
	magicClient := magic.NewClient("")
	user, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	kenc, b64ts, sig := testutil.GetMagicLinkParts(user.GetMagicLoginMagicLink(magicClient))

	tests := []struct {
		Name         string
		GivenBody    string
		ExpectStatus int
	}{
		{
			GivenBody:    fmt.Sprintf(`{"signature": "%s", "timestamp": "%s", "userId": "%s"}`, sig, b64ts, kenc),
			ExpectStatus: http.StatusOK,
		},
		{
			GivenBody:    `{}`,
			ExpectStatus: http.StatusBadRequest,
		},
		{
			GivenBody:    fmt.Sprintf(`{"signature": "random", "timestamp": "%s", "userId": "%s"}`, b64ts, kenc),
			ExpectStatus: http.StatusUnauthorized,
		},
		{
			GivenBody:    fmt.Sprintf(`{"signature": "%s", "timestamp": "%s", "userId": "nonsense"}`, sig, b64ts),
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			apitest.New("MagicLogin").
				Handler(_handler).
				Post("/users/magic").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus).
				End()
		})
	}
}

func TestUpdateUser(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
		OutData         map[string]interface{}
	}{
		{
			GivenAuthHeader: testutil.GetAuthHeader(existingUser.Token),
			GivenBody: map[string]interface{}{
				"firstName": "Sir",
				"lastName":  "Malebranche",
			},
			ExpectStatus: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser.ID,
				"firstName": "Sir",
				"lastName":  "Malebranche",
				"token":     existingUser.Token,
				"verified":  existingUser.Verified,
				"email":     existingUser.Email,
			},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New("UpdateUser").
				Handler(_handler).
				Patch("/users").
				JSON(tcase.GivenBody).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				// tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", tcase.OutData["id"]))
				tt.Assert(jsonpath.Equal("$.firstName", tcase.OutData["firstName"]))
				tt.Assert(jsonpath.Equal("$.lastName", tcase.OutData["lastName"]))
				tt.Assert(jsonpath.Equal("$.token", tcase.OutData["token"]))
				tt.Assert(jsonpath.Equal("$.verified", tcase.OutData["verified"]))
				tt.Assert(jsonpath.Equal("$.email", tcase.OutData["email"]))
			}

			tt.End()
		})
	}
}

func TestUploadAvatar(t *testing.T) {
	payload := `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q==","x":0,"y":0,"size":9.195831298828125}`

	user, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	apitest.New("UploadAvatar").
		Handler(_handler).
		Post("/users/avatar").
		JSON(payload).
		Headers(testutil.GetAuthHeader(user.Token)).
		Expect(t).
		Status(http.StatusOK).
		End()
}

func TestAddEmail(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
	}{
		{
			GivenAuthHeader: testutil.GetAuthHeader(existingUser.Token),
			GivenBody: map[string]interface{}{
				"email": "somenewemail@mail.com",
			},
			ExpectStatus: http.StatusOK,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			apitest.New(tcase.Name).
				Handler(_handler).
				Post("/users/emails").
				JSON(tcase.GivenBody).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus).
				End()
		})
	}
}

func TestRemoveEmail(t *testing.T) {
	const emailToRemove = "testemailfun@mail.com"
	existingUser1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2.AddEmail(emailToRemove)
	_dbClient.Put(_ctx, existingUser2.Key, existingUser2)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
	}{
		{
			Name:            "cannot remove primary email",
			GivenAuthHeader: testutil.GetAuthHeader(existingUser1.Token),
			GivenBody: map[string]interface{}{
				"email": existingUser1.Email,
			},
			ExpectStatus: http.StatusBadRequest,
		},

		{
			Name:            "success",
			GivenAuthHeader: testutil.GetAuthHeader(existingUser2.Token),
			GivenBody: map[string]interface{}{
				"email": emailToRemove,
			},
			ExpectStatus: http.StatusOK,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			apitest.New(tcase.Name).
				Handler(_handler).
				Delete("/users/emails").
				JSON(tcase.GivenBody).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus).
				End()
		})
	}
}

func TestMakeEmailPrimary(t *testing.T) {
	const emailToMakePrimary = "myprimary@mail.com"
	existingUser1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2.AddEmail(emailToMakePrimary)
	_dbClient.Put(_ctx, existingUser2.Key, existingUser2)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
	}{
		{
			Name:            "cannot make unverified email primary",
			GivenAuthHeader: testutil.GetAuthHeader(existingUser1.Token),
			GivenBody: map[string]interface{}{
				"email": "nonverifiedemail@mail.com",
			},
			ExpectStatus: http.StatusBadRequest,
		},

		{
			Name:            "success",
			GivenAuthHeader: testutil.GetAuthHeader(existingUser2.Token),
			GivenBody: map[string]interface{}{
				"email": emailToMakePrimary,
			},
			ExpectStatus: http.StatusOK,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			apitest.New(tcase.Name).
				Handler(_handler).
				Patch("/users/emails").
				JSON(tcase.GivenBody).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus).
				End()
		})
	}
}
