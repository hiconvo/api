package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/handlers"
	"github.com/hiconvo/api/models"
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

	// Set globals to be used by tests below
	tc = ctx
	th = handlers.CreateRouter()
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

	key, err := tclient.Put(tc, u.Key, &u)
	if err != nil {
		t.Fatal(err)
	}

	u.Key = key
	u.ID = key.Encode()
	u.DeriveProperties()

	return u, password
}

func createTestThread(t *testing.T, owner *models.User, users []*models.User) models.Thread {
	// Create the thread.
	thread, tErr := models.NewThread("test", owner, users)
	if tErr != nil {
		t.Fatal(tErr)
	}

	// Save the thread and extract the key and ID.
	key, kErr := tclient.Put(tc, thread.Key, &thread)
	if kErr != nil {
		t.Fatal(kErr)
	}
	thread.ID = key.Encode()
	thread.Key = key

	// Add the thread to the user objects.
	for i := range thread.Users {
		thread.Users[i].AddThread(&thread)
	}

	// Create a slice of keys corresponding to users so that the
	// users can be updated with their new thread membership in the db.
	userKeys := make([]*datastore.Key, len(thread.Users))
	for i := range thread.Users {
		userKeys[i] = thread.Users[i].Key
	}

	// Save the users.
	_, uErr := tclient.PutMulti(tc, userKeys, thread.Users)
	if uErr != nil {
		t.Fatal(uErr)
	}

	return thread
}

func createTestMessage(t *testing.T, user *models.User, thread *models.Thread) models.Message {
	message, merr := models.NewMessage(user, thread, random.String(50))
	if merr != nil {
		t.Fatal(merr)
	}

	key, kErr := tclient.Put(tc, message.Key, &message)
	if kErr != nil {
		t.Fatal(kErr)
	}
	message.Key = key
	message.ID = key.Encode()

	return message
}

func createTestEvent(t *testing.T, owner *models.User, users []*models.User) models.Event {
	// Create the thread.
	event, tErr := models.NewEvent("test", "locKey", "loc", 0.0, 0.0, owner, users)
	if tErr != nil {
		t.Fatal(tErr)
	}

	// Save the event and extract the key and ID.
	key, kErr := tclient.Put(tc, event.Key, &event)
	if kErr != nil {
		t.Fatal(kErr)
	}
	event.ID = key.Encode()
	event.Key = key

	// Add the event to the user objects.
	for i := range users {
		users[i].AddEvent(&event)
	}

	// Create a slice of keys corresponding to users so that the
	// users can be updated with their new event membership in the db.
	userKeys := make([]*datastore.Key, len(users))
	for i := range users {
		userKeys[i] = users[i].Key
	}

	// Save the users.
	_, uErr := tclient.PutMulti(tc, userKeys, users)
	if uErr != nil {
		t.Fatal(uErr)
	}

	return event
}

func getAuthHeader(token string) map[string]string {
	return map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)}
}
