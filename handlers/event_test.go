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
// POST /events Tests
//////////////////////

func TestCreateEvent(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)

	type test struct {
		AuthHeader  map[string]string
		InData      map[string]interface{}
		OutCode     int
		OutOwnerID  string
		OutMemberID string
	}

	tests := []test{
		// Good payload
		{
			AuthHeader: getAuthHeader(u1.Token),
			InData: map[string]interface{}{
				"name": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
				},
				"location":    random.String(10),
				"locationKey": random.String(10),
			},
			OutCode:     http.StatusCreated,
			OutOwnerID:  u1.ID,
			OutMemberID: u2.ID,
		},
		{
			AuthHeader: getAuthHeader(u1.Token),
			InData: map[string]interface{}{
				"name": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
					map[string]string{
						"email": "test@test.com",
					},
				},
				"location":    random.String(10),
				"locationKey": random.String(10),
			},
			OutCode:     http.StatusCreated,
			OutOwnerID:  u1.ID,
			OutMemberID: u2.ID,
		},
		// Bad payload
		{
			AuthHeader: getAuthHeader(u1.Token),
			InData: map[string]interface{}{
				"name": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": "Rudolf Carnap",
					},
				},
				"location":    random.String(10),
				"locationKey": random.String(10),
			},
			OutCode: http.StatusBadRequest,
		},
		// Bad headers
		{
			AuthHeader: map[string]string{"boop": "beep"},
			InData: map[string]interface{}{
				"name": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
				},
				"location":    random.String(10),
				"locationKey": random.String(10),
			},
			OutCode: http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/events", testCase.InData, testCase.AuthHeader)
		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode < 400 {
			gotOwnerID, _ := respData["owner"].(map[string]interface{})["id"].(string)
			thelpers.AssertEqual(t, gotOwnerID, testCase.OutOwnerID)

			gotParticipantID, _ := respData["users"].([]interface{})[0].(map[string]interface{})["id"].(string)
			thelpers.AssertEqual(t, gotParticipantID, testCase.OutMemberID)
		}
	}
}

//////////////////////
// GET /events Tests
//////////////////////

func TestGetEvents(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member1, &member2})

	type test struct {
		AuthHeader   map[string]string
		OutCode      int
		IsEventInRes bool
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK, IsEventInRes: true},
		{AuthHeader: getAuthHeader(member1.Token), OutCode: http.StatusOK, IsEventInRes: true},
		{AuthHeader: getAuthHeader(member2.Token), OutCode: http.StatusOK, IsEventInRes: true},
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusOK, IsEventInRes: false},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized, IsEventInRes: false},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", "/events", nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		gotEvents := respData["events"].([]interface{})

		if testCase.IsEventInRes {
			thelpers.AssetObjectsContainKeys(t, "id", []string{event.ID}, gotEvents)
			thelpers.AssetObjectsContainKeys(t, "name", []string{event.Name}, gotEvents)

			gotEvent := gotEvents[0].(map[string]interface{})

			gotEventUsers := gotEvent["users"].([]interface{})
			thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, member1.ID, member2.ID}, gotEventUsers)
			thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, member1.FullName, member2.FullName}, gotEventUsers)

			gotEventOwner := gotEvent["owner"].(map[string]interface{})
			thelpers.AssertEqual(t, gotEventOwner["id"], event.Owner.ID)
			thelpers.AssertEqual(t, gotEventOwner["fullName"], event.Owner.FullName)
		} else {
			thelpers.AssetObjectsContainKeys(t, "id", []string{}, gotEvents)
		}
	}
}

/////////////////////////
// GET /event/{id} Tests
/////////////////////////

func TestGetEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member})
	url := fmt.Sprintf("/events/%s", event.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK},
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusOK},
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", url, nil, testCase.AuthHeader)
		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], event.ID)

		gotEventOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotEventOwner["id"], event.Owner.ID)
		thelpers.AssertEqual(t, gotEventOwner["fullName"], event.Owner.FullName)

		gotEventUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, member.ID}, gotEventUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, member.FullName}, gotEventUsers)
	}
}

///////////////////////////
// PATCH /event/{id} Tests
///////////////////////////

func TestUpdateEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member})
	url := fmt.Sprintf("/events/%s", event.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
		ShouldPass bool
		InData     map[string]interface{}
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK, ShouldPass: true, InData: map[string]interface{}{"name": "Ruth Marcus"}},
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusNotFound, ShouldPass: false, InData: map[string]interface{}{"name": "Ruth Marcus"}},
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound, ShouldPass: false},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized, ShouldPass: false},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "PATCH", url, testCase.InData, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		if testCase.ShouldPass {
			thelpers.AssertEqual(t, respData["name"], testCase.InData["name"])
		} else {
			thelpers.AssertEqual(t, respData["name"], event.Name)
		}
	}
}

////////////////////////////
// DELETE /event/{id} Tests
////////////////////////////

func TestDeleteEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member})
	url := fmt.Sprintf("/events/%s", event.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
		ShouldPass bool
	}

	tests := []test{
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusNotFound, ShouldPass: false},
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound, ShouldPass: false},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized, ShouldPass: false},
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK, ShouldPass: true},
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusNotFound, ShouldPass: true},
	}

	for _, testCase := range tests {
		_, rr, _ := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.ShouldPass {
			var gotEvent models.Event
			err := tclient.Get(tc, event.Key, &gotEvent)
			thelpers.AssertEqual(t, err, datastore.ErrNoSuchEntity)
		}
	}
}

/////////////////////////////////////
// POST /event/{id}/users/{id} Tests
/////////////////////////////////////

func TestAddToEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	memberToAdd, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member})

	type test struct {
		AuthHeader map[string]string
		OutCode    int
		InID       string
		ShouldPass bool
	}

	tests := []test{
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound, InID: memberToAdd.ID},
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusNotFound, InID: memberToAdd.ID},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized, InID: memberToAdd.ID},
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK, InID: memberToAdd.ID},
	}

	for _, testCase := range tests {
		url := fmt.Sprintf("/events/%s/users/%s", event.ID, testCase.InID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], event.ID)

		gotEventOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotEventOwner["id"], event.Owner.ID)
		thelpers.AssertEqual(t, gotEventOwner["fullName"], event.Owner.FullName)

		gotEventUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, member.ID, memberToAdd.ID}, gotEventUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, member.FullName, memberToAdd.FullName}, gotEventUsers)
	}
}

///////////////////////////////////////
// DELETE /event/{id}/users/{id} Tests
///////////////////////////////////////

func TestRemoveFromEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	memberToRemove, _ := createTestUser(t)
	memberToLeave, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member, &memberToRemove, &memberToLeave})

	type test struct {
		AuthHeader     map[string]string
		InID           string
		OutCode        int
		OutMemberIDs   []string
		OutMemberNames []string
	}

	tests := []test{
		{
			AuthHeader: getAuthHeader(nonmember.Token),
			InID:       member.ID,
			OutCode:    http.StatusNotFound,
		},
		{
			AuthHeader: getAuthHeader(member.Token),
			InID:       memberToRemove.ID,
			OutCode:    http.StatusNotFound,
		},
		{
			AuthHeader: map[string]string{"boop": "beep"},
			InID:       member.ID,
			OutCode:    http.StatusUnauthorized,
		},
		{
			AuthHeader:     getAuthHeader(owner.Token),
			InID:           memberToRemove.ID,
			OutCode:        http.StatusOK,
			OutMemberIDs:   []string{owner.ID, member.ID, memberToLeave.ID},
			OutMemberNames: []string{owner.FullName, member.FullName, memberToLeave.FullName},
		},
		{
			AuthHeader:     getAuthHeader(memberToLeave.Token),
			InID:           memberToLeave.ID,
			OutCode:        http.StatusOK,
			OutMemberIDs:   []string{owner.ID, member.ID},
			OutMemberNames: []string{owner.FullName, member.FullName},
		},
	}

	for _, testCase := range tests {
		url := fmt.Sprintf("/events/%s/users/%s", event.ID, testCase.InID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], event.ID)

		gotEventOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotEventOwner["id"], event.Owner.ID)
		thelpers.AssertEqual(t, gotEventOwner["fullName"], event.Owner.FullName)

		gotEventUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", testCase.OutMemberIDs, gotEventUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", testCase.OutMemberNames, gotEventUsers)
	}
}
