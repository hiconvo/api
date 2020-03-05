package router_test

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/steinfletcher/apitest"
	"github.com/steinfletcher/apitest-jsonpath"
	"github.com/stretchr/testify/assert"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/thelpers"
)

////////////////////
// POST /users Tests
////////////////////

func TestCreateUser(t *testing.T) {
	existingUser, _ := createTestUser(t)

	tests := []struct {
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}{
		{
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
			GivenBody: map[string]interface{}{
				"email":    "rudolf.carnap@charles.cz",
				"lastName": "Carnap",
				"password": "",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"firstName":"This field is required","password":"Must be at least 8 characters long"}`,
		},
		{
			GivenBody: map[string]interface{}{
				"email":     "kit.fine@nyu.edu",
				"firstName": true,
				"password":  "Reality is constituted by tensed facts",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"type mismatch on FirstName field: found bool, expected string"}`,
		},
		{
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

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users", testCase.GivenBody, nil)

		assert.Equal(t, testCase.ExpectStatus, rr.Result().StatusCode)

		if testCase.ExpectStatus >= 400 {
			assert.Equal(t, testCase.ExpectBody, rr.Body.String())
		} else {
			assert.Equal(t, testCase.GivenBody["email"], respData["email"])
			assert.Equal(t, testCase.GivenBody["firstName"], respData["firstName"])
			assert.Equal(t, testCase.GivenBody["lastName"], respData["lastName"])
		}
	}
}

////////////////////
// GET /users Tests
////////////////////

func TestGetCurrentUser(t *testing.T) {
	existingUser, _ := createTestUser(t)
	// Create a couple more users to create the possibility that a wrong user
	// be retrieved
	createTestUser(t)
	createTestUser(t)

	type test struct {
		GivenAuthHeader map[string]string
		ExpectStatus    int
		ExpectBody      string
	}

	tests := []test{
		{
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			ExpectStatus:    http.StatusOK,
		},
		{
			GivenAuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			GivenAuthHeader: map[string]string{"everything": "is what it is"},
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			GivenAuthHeader: nil,
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", "/users", nil, testCase.GivenAuthHeader)

		assert.Equal(t, testCase.ExpectStatus, rr.Result().StatusCode)

		if testCase.ExpectStatus >= 400 {
			assert.Equal(t, testCase.ExpectBody, rr.Body.String())
		} else {
			assert.Equal(t, existingUser.ID, respData["id"])
			assert.Equal(t, existingUser.FirstName, respData["firstName"])
			assert.Equal(t, existingUser.LastName, respData["lastName"])
			assert.Equal(t, existingUser.Token, respData["token"])
			assert.Equal(t, existingUser.Verified, respData["verified"])
			assert.Equal(t, existingUser.Email, respData["email"])
		}
	}
}

////////////////////
// GET /users/{id} Tests
////////////////////

func TestGetUser(t *testing.T) {
	existingUser, _ := createTestUser(t)
	user1, _ := createTestUser(t)
	// Create a couple more users to create the possibility that a wrong user
	// be retrieved
	createTestUser(t)
	createTestUser(t)

	type test struct {
		GivenAuthHeader map[string]string
		URL             string
		ExpectStatus    int
		ExpectBody      string
	}

	tests := []test{
		{
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:             fmt.Sprintf("/users/%s", user1.ID),
			ExpectStatus:    http.StatusOK,
		},
		{
			GivenAuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			URL:             fmt.Sprintf("/users/%s", user1.ID),
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:             fmt.Sprintf("/users/%s", "somenonsense"),
			ExpectStatus:    http.StatusNotFound,
			ExpectBody:      `{"message":"Could not get user"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", testCase.URL, nil, testCase.GivenAuthHeader)

		assert.Equal(t, testCase.ExpectStatus, rr.Result().StatusCode)

		if testCase.ExpectStatus >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.ExpectBody)
		} else {
			assert.Equal(t, user1.ID, respData["id"])
			assert.Equal(t, user1.FirstName, respData["firstName"])
			assert.Equal(t, user1.LastName, respData["lastName"])
			assert.Equal(t, user1.FullName, respData["fullName"])
			assert.Nil(t, respData["token"])
			assert.Nil(t, respData["email"])
		}
	}
}

/////////////////////////
// POST /users/auth Tests
/////////////////////////

func TestAuthenticateUser(t *testing.T) {
	existingUser, password := createTestUser(t)

	type test struct {
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}

	tests := []test{
		{
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": password,
			},
			ExpectStatus: http.StatusOK,
		},
		{
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "123456789",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Invalid credentials"}`,
		},
		{
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"password":"This field is required"}`,
		},
		{
			GivenBody: map[string]interface{}{
				"email":    "santa@northpole.com",
				"password": "have you been naughty or nice?",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Invalid credentials"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/auth", testCase.GivenBody, nil)

		assert.Equal(t, testCase.ExpectStatus, rr.Result().StatusCode)

		if testCase.ExpectStatus >= 400 {
			assert.Equal(t, testCase.ExpectBody, rr.Body.String())
		} else {
			assert.Equal(t, existingUser.ID, respData["id"])
			assert.Equal(t, existingUser.FirstName, respData["firstName"])
			assert.Equal(t, existingUser.LastName, respData["lastName"])
			assert.Equal(t, existingUser.Token, respData["token"])
			assert.Equal(t, existingUser.Verified, respData["verified"])
			assert.Equal(t, existingUser.Email, respData["email"])
		}
	}
}

/////////////////////////
// POST /users/oauth Tests
/////////////////////////

func TestOAuth(t *testing.T) {
	existingUser1, _ := createTestUser(t)

	existingUser2, _ := createTestUser(t)
	existingUser2.PasswordDigest = ""
	existingUser2.Verified = false
	if err := existingUser2.Commit(tc); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		GivenBody       string
		GivenOAuthToken string
		GivenEmail      string
		ExpectStatus    int
		ExpectFirstName string
		ExpectLastName  string
		Token           string
	}{
		{
			GivenOAuthToken: "123",
			GivenEmail:      "bob.kennedy@whitehouse.gov",
			GivenBody:       `{"provider": "google", "token": "123"}`,
			ExpectStatus:    200,
			ExpectFirstName: "John",
			ExpectLastName:  "Kennedy",
		},
		{
			GivenOAuthToken: "123",
			GivenEmail:      "bob.kennedy@whitehouse.gov",
			GivenBody:       `{"provider": "google", "token": "123"}`,
			ExpectStatus:    200,
			ExpectFirstName: "John",
			ExpectLastName:  "Kennedy",
		},
		{
			GivenOAuthToken: "456",
			GivenEmail:      existingUser1.Email,
			GivenBody:       `{"provider": "google", "token": "456"}`,
			ExpectStatus:    200,
			ExpectFirstName: existingUser1.FirstName,
			ExpectLastName:  existingUser1.LastName,
		},
		{
			GivenOAuthToken: "789",
			GivenEmail:      "merge@me.com",
			GivenBody:       `{"provider": "google", "token": "789"}`,
			ExpectStatus:    200,
			ExpectFirstName: existingUser2.FirstName,
			ExpectLastName:  existingUser2.LastName,
			Token:           existingUser2.Token,
		},
		{
			GivenOAuthToken: "789",
			GivenEmail:      "merge@me.com",
			GivenBody:       `{"provider": "notvalid", "token": "notvalid"}`,
			ExpectStatus:    400,
			Token:           existingUser2.Token,
		},
	}

	for _, testCase := range tests {
		oauthMock := apitest.NewMock().
			Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", testCase.GivenOAuthToken)).
			RespondWith().
			Body(fmt.Sprintf(`{
				"aud": "",
				"sub": "%s",
				"email": "%s",
				"given_name": "%s",
				"family_name": "%s",
				"picture": ""
			}`, testCase.GivenEmail, testCase.GivenEmail, testCase.ExpectFirstName, testCase.ExpectLastName)).
			Status(200).
			End()

		headers := map[string]string{"Content-Type": "application/json"}

		if testCase.Token != "" {
			headers["Authorization"] = fmt.Sprintf("Bearer %s", testCase.Token)
		}

		apit := apitest.New("OAuth").
			Mocks(oauthMock).
			Handler(th).
			Post("/users/oauth").
			Headers(headers).
			Body(testCase.GivenBody).
			Expect(t).
			Status(testCase.ExpectStatus)

		if testCase.ExpectStatus < 300 {
			apit.
				Assert(jsonpath.Equal("$.email", testCase.GivenEmail)).
				Assert(jsonpath.Equal("$.firstName", testCase.ExpectFirstName)).
				Assert(jsonpath.Equal("$.lastName", testCase.ExpectLastName))
		}

		apit.End()
	}
}

/////////////////////
// PATCH /users Tests
/////////////////////

// TODO: Test update email and password fields.

func TestUpdateUser(t *testing.T) {
	existingUser, _ := createTestUser(t)

	type test struct {
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
		OutData         map[string]interface{}
		ExpectBody      string
	}

	tests := []test{
		{
			GivenAuthHeader: getAuthHeader(existingUser.Token),
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

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "PATCH", "/users", testCase.GivenBody, testCase.GivenAuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.ExpectStatus)

		if testCase.ExpectStatus >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.ExpectBody)
		} else {
			thelpers.AssertEqual(t, respData["id"], testCase.OutData["id"])
			thelpers.AssertEqual(t, respData["firstName"], testCase.OutData["firstName"])
			thelpers.AssertEqual(t, respData["lastName"], testCase.OutData["lastName"])
			thelpers.AssertEqual(t, respData["token"], testCase.OutData["token"])
			thelpers.AssertEqual(t, respData["verified"], testCase.OutData["verified"])
			thelpers.AssertEqual(t, respData["email"], testCase.OutData["email"])
		}
	}
}

///////////////////////
// POST /users/password
///////////////////////

func TestUpdatePassword(t *testing.T) {
	existingUser, _ := createTestUser(t)
	link := magic.NewLink(existingUser.Key, existingUser.PasswordDigest, "reset")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]

	existingUser2, _ := createTestUser(t)
	link2 := magic.NewLink(existingUser2.Key, existingUser2.PasswordDigest, "reset")
	split2 := strings.Split(link2, "/")
	kenc2 := split2[len(split2)-3]
	b64ts2 := split2[len(split2)-2]

	type test struct {
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
		OutData         map[string]interface{}
		ExpectBody      string
	}

	tests := []test{
		{
			GivenBody: map[string]interface{}{
				"signature": sig,
				"timestamp": b64ts,
				"userID":    kenc,
				"password":  "12345678",
			},
			ExpectStatus: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser.ID,
				"firstName": existingUser.FirstName,
				"lastName":  existingUser.LastName,
				"token":     existingUser.Token,
				"verified":  existingUser.Verified,
				"email":     existingUser.Email,
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

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/password", testCase.GivenBody, testCase.GivenAuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.ExpectStatus)

		if testCase.ExpectStatus >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.ExpectBody)
		} else {
			thelpers.AssertEqual(t, respData["id"], testCase.OutData["id"])
			thelpers.AssertEqual(t, respData["firstName"], testCase.OutData["firstName"])
			thelpers.AssertEqual(t, respData["lastName"], testCase.OutData["lastName"])
			thelpers.AssertEqual(t, respData["token"], testCase.OutData["token"])
			thelpers.AssertEqual(t, respData["verified"], testCase.OutData["verified"])
			thelpers.AssertEqual(t, respData["email"], testCase.OutData["email"])
		}
	}
}

///////////////////////
// POST /users/verify
///////////////////////

func createVerifyLink(u models.User, email string) (string, string, string) {
	salt := email + strconv.FormatBool(u.HasEmail(email))

	link := magic.NewLink(u.Key, salt, "verify")

	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]

	return kenc, b64ts, sig
}

func TestVerifyEmail(t *testing.T) {
	// Standard case of verifying email after account creation
	existingUser1, _ := createTestUser(t)
	existingUser1.Emails = []string{}
	existingUser1.Verified = false
	existingUser1.Commit(tc)
	kenc1, b64ts1, sig1 := createVerifyLink(existingUser1, existingUser1.Email)

	// Bad signature case
	existingUser2, _ := createTestUser(t)
	kenc2, b64ts2, _ := createVerifyLink(existingUser2, existingUser2.Email)

	// Adding a new email
	existingUser3, _ := createTestUser(t)
	kenc3, b64ts3, sig3 := createVerifyLink(existingUser3, "new@email.com")

	// Merging accounts
	existingUser4, _ := createTestUser(t) // User to merge into
	existingUser5, _ := createTestUser(t) // User to be merged
	// Assign test event, thread, and messages to user to be merged
	event := createTestEvent(t, &existingUser2, []*models.User{&existingUser5})
	eventMessage := createTestEventMessage(t, &existingUser5, event)
	thread := createTestThread(t, &existingUser5, []*models.User{&existingUser4, &existingUser3})
	threadMessage := createTestThreadMessage(t, &existingUser5, &thread)
	// Add reference to user to be merged in existingUser2's contacts
	existingUser2.AddContact(&existingUser5)
	if err := existingUser2.Commit(tc); err != nil {
		t.Error(err.Error())
	}
	kenc4, b64ts4, sig4 := createVerifyLink(existingUser4, existingUser5.Email)

	type test struct {
		GivenAuthHeader map[string]string
		GivenBody       map[string]interface{}
		ExpectStatus    int
		OutData         map[string]interface{}
		ExpectBody      string
		VerifyFunc      func() bool
	}

	tests := []test{
		// Standard case
		{
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
				_, err := models.GetUserByID(tc, existingUser5.ID)
				if err == nil {
					return false
				}

				// Make sure that existingUser5's events were transfered to
				// existingUser4
				events, err := models.GetEventsByUser(tc, &existingUser4, &models.Pagination{Size: -1})
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
				threads, err := models.GetThreadsByUser(tc, &existingUser4, &models.Pagination{})
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
				messages, err := models.GetUnhydratedMessagesByUser(tc, &existingUser4)
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
				refreshedExistingUser2, err := models.GetUserByID(tc, existingUser2.ID)
				if err != nil {
					return false
				}
				contacts, err := models.GetContactsByUser(tc, &refreshedExistingUser2)
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

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/verify", testCase.GivenBody, testCase.GivenAuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.ExpectStatus)

		if testCase.ExpectStatus >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.ExpectBody)
		} else {
			thelpers.AssertEqual(t, respData["id"], testCase.OutData["id"])
			thelpers.AssertEqual(t, respData["firstName"], testCase.OutData["firstName"])
			thelpers.AssertEqual(t, respData["lastName"], testCase.OutData["lastName"])
			thelpers.AssertEqual(t, respData["token"], testCase.OutData["token"])
			thelpers.AssertEqual(t, respData["verified"], testCase.OutData["verified"])
			thelpers.AssertEqual(t, respData["email"], testCase.OutData["email"])

			emails := respData["emails"].([]interface{})
			var gotEmails []string
			for _, email := range emails {
				strEmail := email.(string)
				gotEmails = append(gotEmails, strEmail)
			}

			wantedEmails := testCase.OutData["emails"].([]string)

			if !reflect.DeepEqual(gotEmails, wantedEmails) {
				t.Errorf("Expected emails did not match got emails")
			}

			if !testCase.VerifyFunc() {
				t.Errorf("Custom verifier failed")
			}
		}
	}
}

///////////////////////
// POST /users/forgot
///////////////////////

func TestForgotPassword(t *testing.T) {
	existingUser, _ := createTestUser(t)

	type test struct {
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}

	tests := []test{
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

	for _, testCase := range tests {
		_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", "/users/forgot", testCase.GivenBody, nil)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.ExpectStatus)

		thelpers.AssertEqual(t, rr.Body.String(), testCase.ExpectBody)
	}
}

///////////////////////
// POST /users/avatar
///////////////////////

func TestUploadAvatar(t *testing.T) {
	payload := `{"blob":"/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAAKAAoDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgcJ/8QAKBAAAQICCAcBAAAAAAAAAAAAAwQFAAECBhESExQjMQkYISIkVIOT/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAbEQACAQUAAAAAAAAAAAAAAAAAAgMEBRIUcf/aAAwDAQACEQMRAD8AYO3EBMjrTVpEtYnIKUxvMyhsYJgH0cb4xVebmrs+sngNk9taM/X4xk6pgy5aYsRl77lKdG9rG3s3gbnlvuH/AEnDacoVtuhwTh//2Q==","x":0,"y":0,"size":9.195831298828125}`

	user, _ := createTestUser(t)

	apitest.New("UploadAvatar").
		Handler(th).
		Post("/users/avatar").
		JSON(payload).
		Headers(getAuthHeader(user.Token)).
		Expect(t).
		Status(http.StatusOK).
		End()
}

///////////////////////
// POST /users/magic
///////////////////////

func TestMagicLogin(t *testing.T) {
	user, _ := createTestUser(t)
	link := magic.NewLink(user.Key, user.Token, "magic")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]

	tests := []struct {
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

	for _, testCase := range tests {
		apitest.New("MagicLogin").
			Handler(th).
			Post("/users/magic").
			JSON(testCase.GivenBody).
			Expect(t).
			Status(testCase.ExpectStatus).
			End()
	}
}
