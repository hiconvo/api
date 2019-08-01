package handlers_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

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

func TestGetUser(t *testing.T) {
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
			AuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
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
		{
			AuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			InData: map[string]interface{}{
				"email": "asdf",
			},
			OutCode:   http.StatusBadRequest,
			OutPaylod: `{"email":"This is not a valid email"}`,
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

func TestVerifyEmailWithValidPayloadSucceeds(t *testing.T) {
	existingUser, _ := createTestUser(t)
	link := magic.NewLink(existingUser.Key, strconv.FormatBool(existingUser.Verified), "verify")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]

	existingUser2, _ := createTestUser(t)
	link2 := magic.NewLink(existingUser2.Key, strconv.FormatBool(existingUser2.Verified), "verify")
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
				"verified":  true,
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
		}
	}
}
