package router_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/router"
	og "github.com/hiconvo/api/utils/opengraph"
	"github.com/hiconvo/api/utils/random"
	"github.com/hiconvo/api/utils/thelpers"
)

var (
	tc      context.Context
	th      http.Handler
	tclient *datastore.Client
)

func TestMain(m *testing.M) {
	os.Chdir("..")
	ctx := thelpers.CreateTestContext()
	client := thelpers.CreateTestDatastoreClient(ctx)
	thelpers.ClearDatastore(ctx, client)

	// Set globals to be used by tests below
	tc = ctx
	th = router.New()
	tclient = client

	result := m.Run()

	thelpers.ClearDatastore(ctx, client)

	os.Exit(result)
}

func Test404(t *testing.T) {
	_, rr, _ := thelpers.TestEndpoint(t, tc, th, "GET", fmt.Sprintf("/%s", random.String(8)), nil, nil)
	thelpers.AssertStatusCodeEqual(t, rr, http.StatusNotFound)
}

func createTestUser(t *testing.T) (models.User, string) {
	password := random.String(20)
	u, err := models.NewUserWithPassword(
		strings.ToLower(fmt.Sprintf("%s@test.com", random.String(20))),
		random.String(20),
		random.String(20),
		password,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Mark the user as verified by default
	u.Verified = true

	// Save the user
	if err := u.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return u, password
}

func createTestThread(t *testing.T, owner *models.User, users []*models.User) models.Thread {
	// Create the thread.
	thread, err := models.NewThread("test", owner, users)
	if err != nil {
		t.Fatal(err)
	}

	// Save the thread
	if err := thread.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return thread
}

func createTestThreadMessage(t *testing.T, user *models.User, thread *models.Thread) models.Message {
	message, err := models.NewThreadMessage(user, thread, random.String(50), "", og.LinkData{})
	if err != nil {
		t.Fatal(err)
	}

	// Save the message
	if err := message.Commit(tc); err != nil {
		t.Fatal(err)
	}

	if err := thread.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return message
}

func createTestEvent(t *testing.T, owner *models.User, users []*models.User) *models.Event {
	// Create the thread.
	event, err := models.NewEvent(
		"test",
		"locKey",
		"loc",
		"description",
		0.0,
		0.0,
		time.Now().Add(time.Duration(1000000000000000)),
		-7*60*60,
		owner,
		users,
		false)
	if err != nil {
		t.Fatal(err)
	}

	eptr := &event

	// Save the event.
	if err := eptr.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return eptr
}

func createTestEventMessage(t *testing.T, user *models.User, event *models.Event) models.Message {
	message, err := models.NewEventMessage(user, event, random.String(50), "")
	if err != nil {
		t.Fatal(err)
	}

	// Save the message
	if err := message.Commit(tc); err != nil {
		t.Fatal(err)
	}

	return message
}

func getAuthHeader(token string) map[string]string {
	return map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)}
}
