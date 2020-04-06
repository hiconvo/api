package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/magic"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/thelpers"
)

//////////////////////
// POST /events Tests
//////////////////////

func TestCreateEvent(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)

	type test struct {
		Name           string
		AuthHeader     map[string]string
		GivenPayload   map[string]interface{}
		ExpectStatus   int
		ExpectOwnerID  string
		ExpectMemberID string
		ExpectHostID   string
	}

	tests := []test{
		{
			Name:       "Good payload",
			AuthHeader: getAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        random.String(10),
				"placeId":     random.String(10),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus:   http.StatusCreated,
			ExpectOwnerID:  u1.ID,
			ExpectMemberID: u2.ID,
		},
		{
			Name:       "Good payload with new email",
			AuthHeader: getAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        random.String(10),
				"placeId":     random.String(10),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
					map[string]string{
						"email": "test@test.com",
					},
				},
			},
			ExpectStatus:   http.StatusCreated,
			ExpectOwnerID:  u1.ID,
			ExpectMemberID: u2.ID,
		},
		{
			Name:       "Good payload with host",
			AuthHeader: getAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        random.String(10),
				"placeId":     random.String(10),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
					map[string]string{
						"email": "test@test.com",
					},
				},
				"hosts": []map[string]string{
					map[string]string{
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
			Name:       "Bad payload",
			AuthHeader: getAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        random.String(10),
				"placeId":     random.String(10),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": "Rudolf Carnap",
					},
				},
			},
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:       "Bad payload with time in past",
			AuthHeader: getAuthHeader(u1.Token),
			GivenPayload: map[string]interface{}{
				"name":        random.String(10),
				"placeId":     random.String(10),
				"timestamp":   "2019-09-08T01:19:20.915Z",
				"description": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:       "Bad headers",
			AuthHeader: map[string]string{"boop": "beep"},
			GivenPayload: map[string]interface{}{
				"name":        random.String(10),
				"placeId":     random.String(10),
				"timestamp":   "2119-09-08T01:19:20.915Z",
				"description": random.String(10),
				"users": []map[string]string{
					map[string]string{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, testCase := range tests {
		encoded, err := json.Marshal(testCase.GivenPayload)
		if err != nil {
			t.Error(err)
		}

		tt := apitest.New(fmt.Sprintf("CreateEvent: %s", testCase.Name)).
			Handler(th).
			Post("/events").
			Headers(testCase.AuthHeader).
			JSON(string(encoded)).
			Expect(t).
			Status(testCase.ExpectStatus)

		if testCase.ExpectStatus < 300 {
			tt.Assert(jsonpath.Equal("$.owner.id", testCase.ExpectOwnerID))
			tt.Assert(jsonpath.Contains("$.users[*].id", testCase.ExpectMemberID))
			if testCase.ExpectHostID != "" {
				tt.Assert(jsonpath.Contains("$.hosts[*].id", testCase.ExpectHostID))
			}
		}

		tt.End()
	}
}

//////////////////////
// GET /events Tests
//////////////////////

func TestGetEvents(t *testing.T) {
	owner, _ := createTestUser(t)
	member1, _ := createTestUser(t)
	member2, _ := createTestUser(t)
	host1, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member1, &member2}, []*models.User{&host1})

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
			thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, host1.ID, member1.ID, member2.ID}, gotEventUsers)
			thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, host1.FullName, member1.FullName, member2.FullName}, gotEventUsers)

			gotEventHosts := gotEvent["hosts"].([]interface{})
			thelpers.AssetObjectsContainKeys(t, "id", []string{host1.ID}, gotEventHosts)
			thelpers.AssetObjectsContainKeys(t, "fullName", []string{host1.FullName}, gotEventHosts)

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
	host, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member}, []*models.User{&host})
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

		gotEventHosts := respData["hosts"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{host.ID}, gotEventHosts)
		thelpers.AssetObjectsContainKeys(t, "fullName", []string{host.FullName}, gotEventHosts)

		gotEventUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{owner.ID, host.ID, member.ID}, gotEventUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", []string{owner.FullName, host.FullName, member.FullName}, gotEventUsers)
	}
}

///////////////////////////
// PATCH /event/{id} Tests
///////////////////////////

func TestUpdateEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	host, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member}, []*models.User{&host})
	url := fmt.Sprintf("/events/%s", event.ID)

	type test struct {
		AuthHeader map[string]string
		OutCode    int
		ShouldPass bool
		InData     map[string]interface{}
	}

	tests := []test{
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusOK, ShouldPass: true, InData: map[string]interface{}{"name": "Ruth Marcus"}},
		{AuthHeader: getAuthHeader(host.Token), OutCode: http.StatusNotFound, ShouldPass: false, InData: map[string]interface{}{"name": "Ruth Marcus"}},
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
	event := createTestEvent(t, &owner, []*models.User{&member}, []*models.User{})
	url := fmt.Sprintf("/events/%s", event.ID)

	type test struct {
		AuthHeader map[string]string
		InBody     map[string]interface{}
		OutCode    int
		ShouldPass bool
	}

	tests := []test{
		{
			AuthHeader: getAuthHeader(member.Token),
			OutCode:    http.StatusNotFound,
			ShouldPass: false,
		},
		{
			AuthHeader: getAuthHeader(nonmember.Token),
			OutCode:    http.StatusNotFound,
			ShouldPass: false,
		},
		{
			AuthHeader: map[string]string{"boop": "beep"},
			OutCode:    http.StatusUnauthorized,
			ShouldPass: false,
		},
		{
			AuthHeader: getAuthHeader(owner.Token),
			InBody:     map[string]interface{}{"message": "had to cancel"},
			OutCode:    http.StatusOK,
			ShouldPass: true,
		},
		{
			AuthHeader: getAuthHeader(owner.Token),
			OutCode:    http.StatusNotFound,
			ShouldPass: true,
		},
	}

	for _, testCase := range tests {
		_, rr, _ := thelpers.TestEndpoint(t, tc, th, "DELETE", url, testCase.InBody, testCase.AuthHeader)

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
	host, _ := createTestUser(t)
	member, _ := createTestUser(t)
	memberToAdd, _ := createTestUser(t)
	secondMemberToAdd, _ := createTestUser(t)
	thridMemberToAdd, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member}, []*models.User{&host})

	eventAllowGuests := createTestEvent(t, &owner, []*models.User{&member}, []*models.User{})
	eventAllowGuests.GuestsCanInvite = true
	if err := eventAllowGuests.Commit(tc); err != nil {
		t.Fatal(err)
	}

	type test struct {
		AuthHeader map[string]string
		OutCode    int
		InID       string
		InEventID  string
		ShouldPass bool
		OutNames   []string
	}

	tests := []test{
		{
			AuthHeader: getAuthHeader(nonmember.Token),
			OutCode:    http.StatusNotFound,
			InID:       memberToAdd.ID,
			InEventID:  event.ID,
		},
		{
			AuthHeader: getAuthHeader(member.Token),
			OutCode:    http.StatusNotFound,
			InID:       memberToAdd.ID,
			InEventID:  event.ID,
		},
		{
			AuthHeader: map[string]string{"boop": "beep"},
			OutCode:    http.StatusUnauthorized,
			InID:       memberToAdd.ID,
			InEventID:  event.ID,
		},
		{
			AuthHeader: getAuthHeader(owner.Token),
			OutCode:    http.StatusOK,
			InID:       memberToAdd.ID,
			OutNames:   []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName},
			InEventID:  event.ID,
		},
		{
			AuthHeader: getAuthHeader(owner.Token),
			OutCode:    http.StatusOK,
			InID:       "addedOnTheFly@again.com",
			OutNames:   []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName, "addedonthefly"},
			InEventID:  event.ID,
		},
		{
			AuthHeader: getAuthHeader(owner.Token),
			OutCode:    http.StatusOK,
			InID:       secondMemberToAdd.Email,
			OutNames:   []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName, "addedonthefly", secondMemberToAdd.FullName},
			InEventID:  event.ID,
		},
		{
			AuthHeader: getAuthHeader(host.Token),
			OutCode:    http.StatusOK,
			InID:       thridMemberToAdd.ID,
			OutNames:   []string{owner.FullName, host.FullName, member.FullName, memberToAdd.FullName, "addedonthefly", secondMemberToAdd.FullName, thridMemberToAdd.FullName},
			InEventID:  event.ID,
		},
		{
			AuthHeader: getAuthHeader(nonmember.Token),
			OutCode:    http.StatusNotFound,
			InID:       memberToAdd.ID,
			InEventID:  eventAllowGuests.ID,
		},
		{
			AuthHeader: getAuthHeader(member.Token),
			OutCode:    http.StatusOK,
			InID:       memberToAdd.ID,
			OutNames:   []string{owner.FullName, member.FullName, memberToAdd.FullName},
			InEventID:  eventAllowGuests.ID,
		},
	}

	for _, testCase := range tests {
		url := fmt.Sprintf("/events/%s/users/%s", testCase.InEventID, testCase.InID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if rr.Code >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], testCase.InEventID)

		gotEventOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotEventOwner["id"], event.Owner.ID)
		thelpers.AssertEqual(t, gotEventOwner["fullName"], event.Owner.FullName)

		// A host was not added for eventAllowGuests
		gotEventHosts := respData["hosts"].([]interface{})
		if testCase.InEventID == event.ID {
			thelpers.AssetObjectsContainKeys(t, "id", []string{host.ID}, gotEventHosts)
			thelpers.AssetObjectsContainKeys(t, "fullName", []string{host.FullName}, gotEventHosts)
		} else {
			thelpers.AssetObjectsContainKeys(t, "id", []string{}, gotEventHosts)
			thelpers.AssetObjectsContainKeys(t, "fullName", []string{}, gotEventHosts)
		}

		gotEventUsers := respData["users"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "fullName", testCase.OutNames, gotEventUsers)
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
	event := createTestEvent(t, &owner, []*models.User{&member, &memberToRemove, &memberToLeave}, []*models.User{})

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

/////////////////////////////////////
// POST /event/{id}/rsvps Tests
/////////////////////////////////////

func TestAddRSVPToEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member}, []*models.User{})

	type test struct {
		AuthHeader map[string]string
		OutCode    int
	}

	tests := []test{
		{AuthHeader: getAuthHeader(nonmember.Token), OutCode: http.StatusNotFound},
		{AuthHeader: map[string]string{"boop": "beep"}, OutCode: http.StatusUnauthorized},
		{AuthHeader: getAuthHeader(owner.Token), OutCode: http.StatusBadRequest},
		{AuthHeader: getAuthHeader(member.Token), OutCode: http.StatusOK},
	}

	for _, testCase := range tests {
		url := fmt.Sprintf("/events/%s/rsvps", event.ID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], event.ID)

		gotEventOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotEventOwner["id"], event.Owner.ID)
		thelpers.AssertEqual(t, gotEventOwner["fullName"], event.Owner.FullName)

		gotEventRSVPs := respData["rsvps"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", []string{member.ID}, gotEventRSVPs)
		thelpers.AssetObjectsContainKeys(t, "fullName", []string{member.FullName}, gotEventRSVPs)
	}
}

///////////////////////////////////////
// DELETE /event/{id}/rsvps Tests
///////////////////////////////////////

func TestRemoveRSVPFromEvent(t *testing.T) {
	owner, _ := createTestUser(t)
	member, _ := createTestUser(t)
	memberToRemove, _ := createTestUser(t)
	nonmember, _ := createTestUser(t)
	event := createTestEvent(t, &owner, []*models.User{&member, &memberToRemove}, []*models.User{})

	if err := event.AddRSVP(&memberToRemove); err != nil {
		t.Fatal(err)
	}

	if err := event.AddRSVP(&member); err != nil {
		t.Fatal(err)
	}

	if err := event.Commit(tc); err != nil {
		t.Fatal(err)
	}

	type test struct {
		AuthHeader     map[string]string
		OutCode        int
		OutMemberIDs   []string
		OutMemberNames []string
	}

	tests := []test{
		{
			AuthHeader: getAuthHeader(nonmember.Token),
			OutCode:    http.StatusNotFound,
		},
		{
			AuthHeader: map[string]string{"boop": "beep"},
			OutCode:    http.StatusUnauthorized,
		},
		{
			AuthHeader: getAuthHeader(owner.Token),
			OutCode:    http.StatusBadRequest,
		},
		{
			AuthHeader:     getAuthHeader(memberToRemove.Token),
			OutCode:        http.StatusOK,
			OutMemberIDs:   []string{member.ID},
			OutMemberNames: []string{member.FullName},
		},
	}

	for _, testCase := range tests {
		url := fmt.Sprintf("/events/%s/rsvps", event.ID)
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "DELETE", url, nil, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], event.ID)

		gotEventOwner := respData["owner"].(map[string]interface{})
		thelpers.AssertEqual(t, gotEventOwner["id"], event.Owner.ID)
		thelpers.AssertEqual(t, gotEventOwner["fullName"], event.Owner.FullName)

		gotEventUsers := respData["rsvps"].([]interface{})
		thelpers.AssetObjectsContainKeys(t, "id", testCase.OutMemberIDs, gotEventUsers)
		thelpers.AssetObjectsContainKeys(t, "fullName", testCase.OutMemberNames, gotEventUsers)
	}
}

/////////////////////////////////////
// POST /event/rsvps Tests
/////////////////////////////////////

func TestMagicRSVP(t *testing.T) {
	existingUser, _ := createTestUser(t)
	existingUser2, _ := createTestUser(t)
	owner, _ := createTestUser(t)

	event := createTestEvent(t, &owner, []*models.User{&existingUser, &existingUser2}, []*models.User{})

	link := magic.NewLink(existingUser.Key, strconv.FormatBool(event.HasRSVP(&existingUser)), "rsvp")
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]

	link2 := magic.NewLink(existingUser2.Key, strconv.FormatBool(event.HasRSVP(&existingUser2)), "rsvp")
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
				"eventID":   event.ID,
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
				"eventID":   event.ID,
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
				"signature": "not a valid signature",
				"timestamp": b64ts2,
				"userID":    kenc2,
				"eventID":   event.ID,
			},
			OutCode:   http.StatusUnauthorized,
			OutPaylod: `{"message":"This link is not valid anymore"}`,
		},
	}

	for _, testCase := range tests {
		_, rr, respData := thelpers.TestEndpoint(t, tc, th, "POST", "/events/rsvps", testCase.InData, testCase.AuthHeader)

		thelpers.AssertStatusCodeEqual(t, rr, testCase.OutCode)

		if testCase.OutCode >= 400 {
			continue
		}

		thelpers.AssertEqual(t, respData["id"], testCase.OutData["id"])
		thelpers.AssertEqual(t, respData["firstName"], testCase.OutData["firstName"])
		thelpers.AssertEqual(t, respData["lastName"], testCase.OutData["lastName"])
		thelpers.AssertEqual(t, respData["token"], testCase.OutData["token"])
		thelpers.AssertEqual(t, respData["verified"], testCase.OutData["verified"])
		thelpers.AssertEqual(t, respData["email"], testCase.OutData["email"])
	}
}

func TestGetMagicLink(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	u4, _ := createTestUser(t)
	event := createTestEvent(t, &u1, []*models.User{&u3}, []*models.User{&u2})

	tests := []struct {
		Name         string
		AuthToken    string
		ExpectStatus int
	}{
		{
			Name:         "Owner",
			AuthToken:    u1.Token,
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Host",
			AuthToken:    u2.Token,
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Guest",
			AuthToken:    u3.Token,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "Random",
			AuthToken:    u4.Token,
			ExpectStatus: http.StatusNotFound,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			apitest.New("GetMagic").
				Handler(th).
				Get(fmt.Sprintf("/events/%s/magic", event.ID)).
				Headers(getAuthHeader(testCase.AuthToken)).
				Expect(t).
				Status(testCase.ExpectStatus).
				End()
		})
	}
}

func TestRollMagicLink(t *testing.T) {
	u1, _ := createTestUser(t)
	u2, _ := createTestUser(t)
	u3, _ := createTestUser(t)
	u4, _ := createTestUser(t)
	event := createTestEvent(t, &u1, []*models.User{&u3}, []*models.User{&u2})

	tests := []struct {
		Name         string
		AuthToken    string
		ExpectStatus int
	}{
		{
			Name:         "Owner",
			AuthToken:    u1.Token,
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Host",
			AuthToken:    u2.Token,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "Guest",
			AuthToken:    u3.Token,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "Random",
			AuthToken:    u4.Token,
			ExpectStatus: http.StatusNotFound,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			apitest.New(fmt.Sprintf("GetMagic: %s", testCase.Name)).
				Handler(th).
				Put(fmt.Sprintf("/events/%s/magic", event.ID)).
				JSON("{}").
				Headers(getAuthHeader(testCase.AuthToken)).
				Expect(t).
				Status(testCase.ExpectStatus).
				End()
		})
	}
}
