package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/thelpers"
)

//////////////////////
// POST /threads Tests
//////////////////////

func TestCreateThreadWithValidPayloadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u1.Token)}

	reqData := map[string]interface{}{
		"subject": random.String(10),
		"users": []map[string]string{
			map[string]string{
				"id": u2.ID,
			},
		},
	}

	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/threads", reqData, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusCreated)

	gotOwnerID, _ := respData["owner"].(map[string]interface{})["id"].(string)
	thelpers.AssertEqual(t, gotOwnerID, u1.ID)

	gotParticipantID, _ := respData["users"].([]interface{})[0].(map[string]interface{})["id"].(string)
	thelpers.AssertEqual(t, gotParticipantID, u2.ID)
}

func TestCreateThreadWithInvalidPayloadFails(t *testing.T) {
	u1, _ := createTestUser(t)
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u1.Token)}

	reqData := map[string]interface{}{
		"subject": random.String(10),
		"users": []map[string]string{
			map[string]string{
				"id": "Rudolf Carnap",
			},
		},
	}

	_, rr1, _ := thelpers.TestEndpoint(t, tc, th, "POST", "/threads", reqData, h)
	thelpers.AssertStatusCodeEqual(t, rr1, http.StatusBadRequest)

	_, rr2, _ := thelpers.TestEndpoint(t, tc, th, "POST", "/threads", reqData, nil)
	thelpers.AssertStatusCodeEqual(t, rr2, http.StatusUnauthorized)
}

//////////////////////
// GET /threads Tests
//////////////////////

func TestGetThreadsSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2, &u3})

	for _, u := range []models.User{u1, u2, u3} {
		h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", "/threads", nil, h)
		thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

		gotThread := respData["threads"].([]interface{})[0].(map[string]interface{})
		gotThreadID := gotThread["id"].(string)
		thelpers.AssertEqual(t, gotThreadID, thread.ID)

		gotThreadOwner := gotThread["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
		thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)

		gotThreadUsers := gotThread["users"].([]interface{})
		for _, c := range gotThreadUsers {
			typedC := c.(map[string]interface{})
			switch typedC["id"] {
			case u1.ID:
				thelpers.AssertEqual(t, typedC["fullName"], u1.FullName)
			case u2.ID:
				thelpers.AssertEqual(t, typedC["fullName"], u2.FullName)
			case u3.ID:
				thelpers.AssertEqual(t, typedC["fullName"], u3.FullName)
			default:
				t.Errorf("handler returned unexpected id: got %v want any of %v",
					typedC["id"], []string{u2.ID, u3.ID})
			}
		}
	}
}

/////////////////////////
// GET /thread/{id} Tests
/////////////////////////

func TestGetThreadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2})

	url := fmt.Sprintf("/threads/%s", thread.ID)

	for _, u := range []models.User{u1, u2} {
		h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, h)

		thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

		thelpers.AssertEqual(t, respData["id"], thread.ID)

		gotThreadOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
		thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)

		gotThreadParticipants := respData["users"].([]interface{})
		thelpers.AssertObjectsContainIDs(t, gotThreadParticipants, []string{u1.ID, u2.ID})
	}
}

func TestGetThreadFailsWithUnauthorizedUser(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2})

	u3, _ := createTestUser(t)

	url := fmt.Sprintf("/threads/%s", thread.ID)

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u3.Token)}
	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusNotFound)
}

///////////////////////////
// PATCH /thread/{id} Tests
///////////////////////////

func TestUpdateThreadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2})
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u1.Token)}

	subject := "Ruth Marcus"
	reqData := map[string]interface{}{
		"subject": subject,
	}

	url := fmt.Sprintf("/threads/%s", thread.ID)
	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "PATCH", url, reqData, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)
	thelpers.AssertEqual(t, respData["subject"], subject)
}

////////////////////////////
// DELETE /thread/{id} Tests
////////////////////////////

func TestDeleteThreadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2})
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u1.Token)}

	url := fmt.Sprintf("/threads/%s", thread.ID)
	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	var gotThread models.Thread
	err := tclient.Get(tc, thread.Key, &gotThread)
	thelpers.AssertEqual(t, err, datastore.ErrNoSuchEntity)
}

/////////////////////////////////////
// POST /thread/{id}/users/{id} Tests
/////////////////////////////////////

func TestAddToThreadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{})
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u1.Token)}

	url := fmt.Sprintf("/threads/%s/users/%s", thread.ID, u2.ID)
	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	thelpers.AssertEqual(t, respData["id"], thread.ID)

	gotThreadOwner := respData["owner"].(map[string]interface{})
	thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
	thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)

	gotThreadParticipants := respData["users"].([]interface{})
	thelpers.AssertObjectsContainIDs(t, gotThreadParticipants, []string{u1.ID, u2.ID})
}

func TestAddToThreadFailsWithUnauthorizedUser(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{})
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u2.Token)}

	url := fmt.Sprintf("/threads/%s/users/%s", thread.ID, u2.ID)
	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusNotFound)
}

///////////////////////////////////////
// DELETE /thread/{id}/users/{id} Tests
///////////////////////////////////////

func TestRemoveFromThreadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)

	for _, u := range []models.User{u1, u2} {
		thread := createTestThread(t, &u1, []*models.User{&u2})
		// change the token to make sure that both the owner and participant can
		// remove user
		h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u.Token)}

		url := fmt.Sprintf("/threads/%s/users/%s", thread.ID, u2.ID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, h)

		thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

		gotThreadOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
		thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)

		gotThreadParticipants := respData["users"].([]interface{})
		thelpers.AssertObjectsContainIDs(t, gotThreadParticipants, []string{u1.ID})
		thelpers.AssertEqual(t, len(respData["users"].([]interface{})), 1)
	}
}

func TestRemoveFromThreadFailsWithUnauthorizedUser(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	u4, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2, &u3})

	url := fmt.Sprintf("/threads/%s/users/%s", thread.ID, u2.ID)

	// Unrelated user cannot remove user
	h1 := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u4.Token)}
	_, rr1, _ := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, h1)
	thelpers.AssertStatusCodeEqual(t, rr1, http.StatusNotFound)

	// Participant cannot remove another participant
	h2 := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u3.Token)}
	_, rr2, _ := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, h2)
	thelpers.AssertStatusCodeEqual(t, rr2, http.StatusNotFound)

	// Participant cannot remove owner
	url = fmt.Sprintf("/threads/%s/users/%s", thread.ID, u1.ID)
	_, rr3, _ := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, h2)
	thelpers.AssertStatusCodeEqual(t, rr3, http.StatusNotFound)
}
