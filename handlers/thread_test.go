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

func TestCreateThread(t *testing.T) {
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
				"subject": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
				},
			},
			OutCode:     http.StatusCreated,
			OutOwnerID:  u1.ID,
			OutMemberID: u2.ID,
		},
		// Bad payload
		{
			AuthHeader: getAuthHeader(u1.Token),
			InData: map[string]interface{}{
				"subject": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": "Rudolf Carnap",
					},
				},
			},
			OutCode: http.StatusBadRequest,
		},
		// Bad headers
		{
			AuthHeader: map[string]string{"boop": "beep"},
			InData: map[string]interface{}{
				"subject": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
				},
			},
			OutCode: http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/threads", testCase.InData, testCase.AuthHeader)
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
// GET /threads Tests
//////////////////////

func TestGetThreads(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member1, &member2})

	type test struct {
		AuthHeader    map[string]string
		OutCode       int
		IsThreadInRes bool
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK, IsThreadInRes: true},
		{AuthHeader: getAuthHeader(member1.Token), OutCode: http.StatusOK, IsThreadInRes: true},
		{AuthHeader: getAuthHeader(member2.Token), OutCode: http.StatusOK, IsThreadInRes: true},
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusOK, IsThreadInRes: false},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized, IsThreadInRes: false},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "GET", "/threads", nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		gotThreads := respData["threads"].([]interface{})

		if testCase.IsThreadInRes {
			thelpers.AssetObjectsContainKeys(t, "id", []string{thread.ID}, gotThreads)
			thelpers.AssetObjectsContainKeys(t, "subject", []string{thread.Subject}, gotThreads)

			gotThread := gotThreads[0].(map[string]interface{})

			gotThreadUsers := gotThread["users"].([]interface{})
			thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, member1.ID, member2.ID}, gotThreadUsers)
			thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, member1.FullName, member2.FullName}, gotThreadUsers)

			gotThreadOwner := gotThread["owner"].(map[string]interface{})
			thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
			thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)
		} else {
			thelpers.AssetObjectsContainKeys(t, "id", []string{}, gotThreads)
		}
	}
}

/////////////////////////
// GET /thread/{id} Tests
/////////////////////////

func TestGetThread(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member})
	url := fmt.Sprintf("/threads/%s", thread.ID)

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

		thelpers.AssertEqual(t, respData["id"], thread.ID)

		gotThreadOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
		thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)

		gotThreadUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, member.ID}, gotThreadUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, member.FullName}, gotThreadUsers)
	}
}

///////////////////////////
// PATCH /thread/{id} Tests
///////////////////////////

func TestUpdateThread(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member})
	url := fmt.Sprintf("/threads/%s", thread.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
		ShouldPass bool
		InData     map[string]interface{}
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK, ShouldPass: true, InData: map[string]interface{}{"subject": "Ruth Marcus"}},
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusNotFound, ShouldPass: false, InData: map[string]interface{}{"subject": "Ruth Marcus"}},
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
			thelpers.AssertEqual(t, respData["subject"], testCase.InData["subject"])
		} else {
			thelpers.AssertEqual(t, respData["subject"], thread.Subject)
		}
	}
}

////////////////////////////
// DELETE /thread/{id} Tests
////////////////////////////

func TestDeleteThread(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member})
	url := fmt.Sprintf("/threads/%s", thread.ID)

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
			var gotThread models.Thread
			err := tclient.Get(tc, thread.Key, &gotThread)
			thelpers.AssertEqual(t, err, datastore.ErrNoSuchEntity)
		}
	}
}

/////////////////////////////////////
// POST /thread/{id}/users/{id} Tests
/////////////////////////////////////

func TestAddToThread(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	memberToAdd, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member})

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
		url := fmt.Sprintf("/threads/%s/users/%s", thread.ID, testCase.InID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], thread.ID)

		gotThreadOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
		thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)

		gotThreadUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, member.ID, memberToAdd.ID}, gotThreadUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, member.FullName, memberToAdd.FullName}, gotThreadUsers)
	}
}

///////////////////////////////////////
// DELETE /thread/{id}/users/{id} Tests
///////////////////////////////////////

func TestRemoveFromThread(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	memberToRemove, _ := createTestUser(t)
	memberToLeave, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	thread := createTestThread(t, &owner, []*models.User{&member, &memberToRemove, &memberToLeave})

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
		url := fmt.Sprintf("/threads/%s/users/%s", thread.ID, testCase.InID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], thread.ID)

		gotThreadOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotThreadOwner["id"], thread.Owner.ID)
		thelpers.AssertEqual(t, gotThreadOwner["fullName"], thread.Owner.FullName)

		gotThreadUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", testCase.OutMemberIDs, gotThreadUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", testCase.OutMemberNames, gotThreadUsers)
	}
}
