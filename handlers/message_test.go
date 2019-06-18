package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/thelpers"
)

////////////////////////////////////
// POST /threads/{id}/messages Tests
////////////////////////////////////

func TestAddMessageToThreadWithValidPayloadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2, &u3})

	url := fmt.Sprintf("/threads/%s/messages", thread.ID)

	// Owner
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u1.Token)}

	reqData := map[string]interface{}{
		"body": random.String(10),
	}

	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", url, reqData, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusCreated)

	// Participant
	h = map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u2.Token)}

	_, rr, _ = thelpers.TestEndpoint(t, tc, th, "POST", url, reqData, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusCreated)
}

func TestAddMessageToThreadWithInvalidPayloadFails(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	u4, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2, &u3})

	url := fmt.Sprintf("/threads/%s/messages", thread.ID)

	// User who does not belong to thread
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u4.Token)}

	reqData := map[string]interface{}{
		"body": random.String(10),
	}

	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "POST", url, reqData, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusNotFound)

	// Invalid payload
	h = map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u2.Token)}

	reqData = map[string]interface{}{
		"what": random.String(10),
	}

	_, rr, _ = thelpers.TestEndpoint(t, tc, th, "POST", url, reqData, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusBadRequest)
}

///////////////////////////////////
// GET /threads/{id}/messages Tests
///////////////////////////////////

func TestGetMessagesWithValidPayloadSucceeds(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2, &u3})
	m1 := createTestMessage(t, &u1, &thread)
	m2 := createTestMessage(t, &u2, &thread)

	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u1.Token)}
	url := fmt.Sprintf("/threads/%s/messages", thread.ID)
	_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusOK)

	messages := respData["messages"].([]interface{})
	firstMessage := messages[0].(map[string]interface{})
	secondMessage := messages[1].(map[string]interface{})

	// Sorted from new to old
	thelpers.AssertEqual(t, firstMessage["id"], m2.ID)
	thelpers.AssertEqual(t, secondMessage["id"], m1.ID)
}

func TestGetMessagesWithInvalidPayloadFails(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	u4, _ := createTestUser(t)
	thread := createTestThread(t, &u1, []*models.User{&u2, &u3})
	createTestMessage(t, &u1, &thread)
	createTestMessage(t, &u2, &thread)

	url := fmt.Sprintf("/threads/%s/messages", thread.ID)

	// User who does not belong to thread
	h := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", u4.Token)}

	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, h)

	thelpers.AssertStatusCodeEqual(t, rr, http.StatusNotFound)
}
