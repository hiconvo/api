package handlers_test

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/thelpers"
)

////////////////////
// POST /users Tests
////////////////////

func TestCreateUser(t *testing.T) {
	existingUser, _ := createTestUser(t)

	type test struct {
		InData    map[string]interface{}
		OutCode   int
		OutPaylod string
	}

	tests := []test{
		{
			InData: map[string]interface{}{
				"email":     "ruth.marcus@yale.edu",
				"firstName": "Ruth",
				"lastName":  "Marcus",
				"password":  "the comma is a giveaway",
			},
			OutCode:   http.StatusCreated,
			OutPaylod: "",
		},
		{
			InData: map[string]interface{}{
				"email":    "rudolf.carnap@charles.cz",
				"lastName": "Carnap",
				"password": "",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"firstName":"This field is required","password":"Must be at least 8 characters long"}`,
		},
		{
			InData: map[string]interface{}{
				"email":     "kit.fine@nyu.edu",
				"firstName": true,
				"password":  "Reality is constituted by tensed facts",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"message":"type mismatch on FirstName field: found bool, expected string"}`,
		},
		{
			InData: map[string]interface{}{
				"email":     existingUser.Email,
				"firstName": "Ruth",
				"lastName":  "Millikan",
				"password":  "Language and thought are biological categories",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"message":"This email has already been registered"}`,
		},
		{
			InData: map[string]interface{}{
				"email":     "it's all in my mind",
				"firstName": "George",
				"lastName":  "Berkeley",
				"password":  "Ordinary objects are ideas",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"email":"This is not a valid email"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users", testCase.InData, nil)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
		} else {
			thelpers.AssertEqual(t, respData["email"], testCase.InData["email"])
			thelpers.AssertEqual(t, respData["firstName"], testCase.InData["firstName"])
			thelpers.AssertEqual(t, respData["lastName"], testCase.InData["lastName"])
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
		AuthHeader map[string]string
		OutCode    int
		OutPaylod  string
	}

	tests := []test{
		{
			AuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			OutCode:    http.StatusOK,
		},
		{
			AuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			OutCode:    http.StatusUnauthorized,
			OutPaylod:  `{"message":"Unauthorized"}`,
		},
		{
			AuthHeader: map[string]string{"everything": "is what it is"},
			OutCode:    http.StatusUnauthorized,
			OutPaylod:  `{"message":"Unauthorized"}`,
		},
		{
			AuthHeader: nil,
			OutCode:    http.StatusUnauthorized,
			OutPaylod:  `{"message":"Unauthorized"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", "/users", nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
		} else {
			thelpers.AssertEqual(t, respData["id"], existingUser.ID)
			thelpers.AssertEqual(t, respData["firstName"], existingUser.FirstName)
			thelpers.AssertEqual(t, respData["lastName"], existingUser.LastName)
			thelpers.AssertEqual(t, respData["token"], existingUser.Token)
			thelpers.AssertEqual(t, respData["verified"], existingUser.Verified)
			thelpers.AssertEqual(t, respData["email"], existingUser.Email)
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
		AuthHeader map[string]string
		URL        string
		OutCode    int
		OutPaylod  string
	}

	tests := []test{
		{
			AuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:        fmt.Sprintf("/users/%s", user1.ID),
			OutCode:    http.StatusOK,
		},
		{
			AuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			URL:        fmt.Sprintf("/users/%s", user1.ID),
			OutCode:    http.StatusUnauthorized,
			OutPaylod:  `{"message":"Unauthorized"}`,
		},
		{
			AuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:        fmt.Sprintf("/users/%s", "somenonsense"),
			OutCode:    http.StatusNotFound,
			OutPaylod:  `{"message":"Could not get user"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", testCase.URL, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
		} else {
			thelpers.AssertEqual(t, respData["id"], user1.ID)
			thelpers.AssertEqual(t, respData["firstName"], user1.FirstName)
			thelpers.AssertEqual(t, respData["lastName"], user1.LastName)
			thelpers.AssertEqual(t, respData["fullName"], user1.FullName)
			thelpers.AssertEqual(t, respData["token"], nil)
			thelpers.AssertEqual(t, respData["email"], nil)
		}
	}
}

/////////////////////////
// POST /users/auth Tests
/////////////////////////

func TestAuthenticateUser(t *testing.T) {
	existingUser, password := createTestUser(t)

	type test struct {
		InData    map[string]interface{}
		OutCode   int
		OutPaylod string
	}

	tests := []test{
		{
			InData: map[string]interface{}{
				"email":    existingUser.Email,
				"password": password,
			},
			OutCode: http.StatusOK,
		},
		{
			InData: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "123456789",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"message":"Invalid credentials"}`,
		},
		{
			InData: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"password":"This field is required"}`,
		},
		{
			InData: map[string]interface{}{
				"email":    "santa@northpole.com",
				"password": "have you been naughty or nice?",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"message":"Invalid credentials"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/auth", testCase.InData, nil)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
		} else {
			thelpers.AssertEqual(t, respData["id"], existingUser.ID)
			thelpers.AssertEqual(t, respData["firstName"], existingUser.FirstName)
			thelpers.AssertEqual(t, respData["lastName"], existingUser.LastName)
			thelpers.AssertEqual(t, respData["token"], existingUser.Token)
			thelpers.AssertEqual(t, respData["verified"], existingUser.Verified)
			thelpers.AssertEqual(t, respData["email"], existingUser.Email)
		}
	}
}

/////////////////////
// PATCH /users Tests
/////////////////////

// TODO: Test update email and password fields.

func TestUpdateUser(t *testing.T) {
	existingUser, _ := createTestUser(t)

	type test struct {
		AuthHeader map[string]string
		InData     map[string]interface{}
		OutCode    int
		OutData    map[string]interface{}
		OutPaylod  string
	}

	tests := []test{
		{
			AuthHeader: getAuthHeader(existingUser.Token),
			InData: map[string]interface{}{
				"firstName": "Sir",
				"lastName":  "Malebranche",
			},
			OutCode: http.StatusOK,
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
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "PATCH", "/users", testCase.InData, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
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
		AuthHeader map[string]string
		InData     map[string]interface{}
		OutCode    int
		OutData    map[string]interface{}
		OutPaylod  string
	}

	tests := []test{
		{
			InData: map[string]interface{}{
				"signature": sig,
				"timestamp": b64ts,
				"userID":    kenc,
				"password":  "12345678",
			},
			OutCode: http.StatusOK,
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
			InData: map[string]interface{}{
				"signature": sig,
				"timestamp": b64ts,
				"userID":    kenc,
				"password":  "12345678",
			},
			OutCode:   http.StatusUnauthorized,
			OutPaylod: `{"message":"This link is not valid anymore"}`,
		},
		{
			InData: map[string]interface{}{
				"signature": "not a valid signature",
				"timestamp": b64ts2,
				"userID":    kenc2,
				"password":  "12345678",
			},
			OutCode:   http.StatusUnauthorized,
			OutPaylod: `{"message":"This link is not valid anymore"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/password", testCase.InData, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
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

	type test struct {
		AuthHeader map[string]string
		InData     map[string]interface{}
		OutCode    int
		OutData    map[string]interface{}
		OutPaylod  string
	}

	tests := []test{
		// Standard case
		{
			InData: map[string]interface{}{
				"signature": sig1,
				"timestamp": b64ts1,
				"userID":    kenc1,
				"email":     existingUser1.Email,
			},
			OutCode: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser1.ID,
				"firstName": existingUser1.FirstName,
				"lastName":  existingUser1.LastName,
				"token":     existingUser1.Token,
				"verified":  true,
				"email":     existingUser1.Email,
				"emails":    []string{existingUser1.Email},
			},
		},

		// Already used magic link (applying standard case second time)
		{
			InData: map[string]interface{}{
				"signature": sig1,
				"timestamp": b64ts1,
				"userID":    kenc1,
				"email":     existingUser1.Email,
			},
			OutCode:   http.StatusUnauthorized,
			OutPaylod: `{"message":"This link is not valid anymore"}`,
		},

		// Bad signature
		{
			InData: map[string]interface{}{
				"signature": "not a valid signature",
				"timestamp": b64ts2,
				"userID":    kenc2,
				"email":     existingUser2.Email,
			},
			OutCode:   http.StatusUnauthorized,
			OutPaylod: `{"message":"This link is not valid anymore"}`,
		},

		// Adding an email
		{
			InData: map[string]interface{}{
				"signature": sig3,
				"timestamp": b64ts3,
				"userID":    kenc3,
				"email":     "new@email.com",
			},
			OutCode: http.StatusOK,
			OutData: map[string]interface{}{
				"id":        existingUser3.ID,
				"firstName": existingUser3.FirstName,
				"lastName":  existingUser3.LastName,
				"token":     existingUser3.Token,
				"verified":  true,
				"email":     existingUser3.Email,
				"emails":    []string{existingUser3.Email, "new@email.com"},
			},
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/verify", testCase.InData, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
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
		}
	}
}

///////////////////////
// POST /users/forgot
///////////////////////

func TestForgotPassword(t *testing.T) {
	existingUser, _ := createTestUser(t)

	type test struct {
		InData    map[string]interface{}
		OutCode   int
		OutPaylod string
	}

	tests := []test{
		{
			InData: map[string]interface{}{
				"email": existingUser.Email,
			},
			OutCode:   http.StatusOK,
			OutPaylod: `{"message":"Check your email for a link to reset your password"}`,
		},
		{
			InData: map[string]interface{}{
				"email": "plato@greece.edu",
			},
			OutCode:   http.StatusOK,
			OutPaylod: `{"message":"Check your email for a link to reset your password"}`,
		},
		{
			InData: map[string]interface{}{
				"email": "asdf",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"email":"This is not a valid email"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", "/users/forgot", testCase.InData, nil)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		thelpers.AssertEqual(t, rr.Body.String(), testCase.OutPaylod)
	}
}
