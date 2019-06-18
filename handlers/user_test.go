package handlers_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/thelpers"
)

////////////////////
// POST /users Tests
////////////////////

func TestCreateUserWithValidPayloadSucceeds(t *testing.T) {
	reqData := map[string]string{
		"email":     strings.ToLower(fmt.Sprintf("%s@test.com", random.String(14))),
		"firstName": random.String(10),
		"lastName":  random.String(10),
		"password":  random.String(10),
	}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users", reqData, nil)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusCreated)

	for k, v := range reqData {
		if k == "password" {
			continue
		}

		thelpers.AssertEqual(t, respData[k], v)
	}
}

func TestCreateUserWithIncompletePayloadFails(t *testing.T) {
	reqData := map[string]string{
		"email": fmt.Sprintf("%s@test.com", random.String(11)),
	}

	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", "/users", reqData, nil)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)

	got := rr.Body.String()
	want := `{"firstName":"This field is required","password":"Must be at least 8 characters long"}`
	thelpers.AssertEqual(t, got, want)
}

func TestCreateUserWithInvalidPayloadFails(t *testing.T) {
	reqData := map[string]interface{}{
		"email":     random.String(10),
		"firstName": true,
		"lastName":  random.String(10),
		"password":  random.String(10),
	}

	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", "/users", reqData, nil)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)

	got := rr.Body.String()
	want := `{"message":"type mismatch on FirstName field: found bool, expected string"}`
	thelpers.AssertEqual(t, got, want)
}

func TestCreateUserWithExistingEmailFails(t *testing.T) {
	// Add user to db
	u, _ := createTestUser(t)

	reqData := map[string]string{
		"email":     u.Email,
		"firstName": random.String(10),
		"lastName":  random.String(10),
		"password":  random.String(10),
	}

	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", "/users", reqData, nil)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)

	got := rr.Body.String()
	want := `{"message":"This email has already been registered"}`
	thelpers.AssertEqual(t, got, want)
}

////////////////////
// GET /users Tests
////////////////////

func TestGetUserWithValidTokenSucceeds(t *testing.T) {
	u, _ := createTestUser(t)

	// Create a couple more users to create the possibility that a wrong user
	// be retrieved
	createTestUser(t)
	createTestUser(t)

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", "/users", nil, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", u.Token),
	})

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	thelpers.AssertEqual(t, respData["id"], u.ID)
	thelpers.AssertEqual(t, respData["firstName"], u.FirstName)
	thelpers.AssertEqual(t, respData["lastName"], u.LastName)
	thelpers.AssertEqual(t, respData["token"], u.Token)
	thelpers.AssertEqual(t, respData["verified"], u.Verified)
	thelpers.AssertEqual(t, respData["email"], u.Email)
}

func TestGetUserWithInvalidTokenFails(t *testing.T) {
	// Create a user to make sure that there is something in the db that could
	// be retreived
	createTestUser(t)

	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "GET", "/users", nil, map[string]string{
		"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz",
	})

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusUnauthorized)
}

func TestGetUserWithInvalidHeaderFails(t *testing.T) {
	// Create a user to make sure that there is something in the db that could
	// be retreived
	createTestUser(t)

	// Invalid header
	_, rr1, _ := thelpers.TestEndpoint(t, tc, th, "GET", "/users", nil, map[string]string{
		"rudolf": "carnap",
	})
	thelpers.AssertStatusCodeEqual(t, rr1, http.StatusUnauthorized)

	// No header
	_, rr2, _ := thelpers.TestEndpoint(t, tc, th, "GET", "/users", nil, nil)
	thelpers.AssertStatusCodeEqual(t, rr2, http.StatusUnauthorized)
}

/////////////////////////
// POST /users/auth Tests
/////////////////////////

func TestAuthenticateUserWithValidPayloadSucceeds(t *testing.T) {
	u, p := createTestUser(t)

	reqData := map[string]string{
		"email":    u.Email,
		"password": p,
	}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/auth", reqData, nil)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	thelpers.AssertEqual(t, respData["id"], u.ID)
	thelpers.AssertEqual(t, respData["firstName"], u.FirstName)
	thelpers.AssertEqual(t, respData["lastName"], u.LastName)
	thelpers.AssertEqual(t, respData["token"], u.Token)
	thelpers.AssertEqual(t, respData["verified"], u.Verified)
	thelpers.AssertEqual(t, respData["email"], u.Email)
}

func TestAuthenticateUserWithInvalidPayloadFails(t *testing.T) {
	u, _ := createTestUser(t)

	// Valid email, wrong password
	reqData := map[string]string{"email": u.Email, "password": "123456789"}
	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/auth", reqData, nil)
	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)
	thelpers.AssertEqual(t, respData["message"], "Invalid credentials")

	// Valid email, missing password
	reqData = map[string]string{"email": u.Email}
	_, rr, respData = thelpers.TestEndpoint(t, tc, th, "POST", "/users/auth", reqData, nil)
	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)
	thelpers.AssertEqual(t, respData["password"], "This field is required")

	// Invalid email
	reqData = map[string]string{"email": "does.not@exist.com", "password": "123456789"}
	_, rr, respData = thelpers.TestEndpoint(t, tc, th, "POST", "/users/auth", reqData, nil)
	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)
	thelpers.AssertEqual(t, respData["message"], "Invalid credentials")
}

/////////////////////
// PATCH /users Tests
/////////////////////

// TODO: Test update email and password fields.

func TestUpdateUserWithValidPayloadSucceeds(t *testing.T) {
	u, _ := createTestUser(t)

	fname := "Rudolf"
	lname := "Carnap"

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
	d := map[string]string{"firstName": fname, "lastName": lname}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "PATCH", "/users", d, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	thelpers.AssertEqual(t, respData["id"], u.ID)
	thelpers.AssertEqual(t, respData["firstName"], fname)
	thelpers.AssertEqual(t, respData["lastName"], lname)
	thelpers.AssertEqual(t, respData["token"], u.Token)
	thelpers.AssertEqual(t, respData["verified"], u.Verified)
	thelpers.AssertEqual(t, respData["email"], u.Email)
}

func TestUpdateUserWithInvalidPayloadFails(t *testing.T) {
	u, _ := createTestUser(t)

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
	d := map[string]string{"email": "asdf"}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "PATCH", "/users", d, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)

	thelpers.AssertEqual(t, respData["email"], "This is not a valid email")
}

///////////////////////
// POST /users/password
///////////////////////

func TestUpdatePasswordWithValidPayloadSucceeds(t *testing.T) {
	u, _ := createTestUser(t)
	link := magic.NewLink(u.Key, u.PasswordDigest, "reset")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
	d := map[string]string{
		"signature": sig,
		"timestamp": b64ts,
		"userID":    kenc,
		"password":  "12345678",
	}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/password", d, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	thelpers.AssertEqual(t, respData["id"], u.ID)
	thelpers.AssertEqual(t, respData["firstName"], u.FirstName)
	thelpers.AssertEqual(t, respData["lastName"], u.LastName)
	thelpers.AssertEqual(t, respData["token"], u.Token)
	thelpers.AssertEqual(t, respData["verified"], u.Verified)
	thelpers.AssertEqual(t, respData["email"], u.Email)

	// Make sure link does not work after use
	_, rr, respData = thelpers.TestEndpoint(t, tc, th, "POST", "/users/password", d, h)
	thelpers.AssertStatusCodeEqual(t, rr, http.StatusUnauthorized)
	thelpers.AssertEqual(t, respData["message"], "This link is not valid anymore")
}

func TestUpdatePasswordWithInvalidPayloadFails(t *testing.T) {
	u, _ := createTestUser(t)
	link := magic.NewLink(u.Key, u.PasswordDigest, "reset")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
	d := map[string]string{
		"signature": random.String(20),
		"timestamp": b64ts,
		"userID":    kenc,
		"password":  "12345678",
	}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/password", d, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusUnauthorized)
	thelpers.AssertEqual(t, respData["message"], "This link is not valid anymore")
}

///////////////////////
// POST /users/verify
///////////////////////

func TestVerifyEmailWithValidPayloadSucceeds(t *testing.T) {
	u, _ := createTestUser(t)
	link := magic.NewLink(u.Key, strconv.FormatBool(u.Verified), "verify")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
	d := map[string]string{
		"signature": sig,
		"timestamp": b64ts,
		"userID":    kenc,
	}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/verify", d, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	thelpers.AssertEqual(t, respData["id"], u.ID)
	thelpers.AssertEqual(t, respData["firstName"], u.FirstName)
	thelpers.AssertEqual(t, respData["lastName"], u.LastName)
	thelpers.AssertEqual(t, respData["token"], u.Token)
	thelpers.AssertEqual(t, respData["verified"], true)
	thelpers.AssertEqual(t, respData["email"], u.Email)

	// Make sure link does not work after use
	_, rr, respData = thelpers.TestEndpoint(t, tc, th, "POST", "/users/verify", d, h)
	thelpers.AssertStatusCodeEqual(t, rr, http.StatusUnauthorized)
	thelpers.AssertEqual(t, respData["message"], "This link is not valid anymore")
}

func TestVerifyEmailWithInvalidPayloadFails(t *testing.T) {
	u, _ := createTestUser(t)
	link := magic.NewLink(u.Key, strconv.FormatBool(u.Verified), "verify")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
	d := map[string]string{
		"signature": random.String(20),
		"timestamp": b64ts,
		"userID":    kenc,
	}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/users/verify", d, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusUnauthorized)
	thelpers.AssertEqual(t, respData["message"], "This link is not valid anymore")
}
