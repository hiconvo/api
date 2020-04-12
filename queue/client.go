package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "google.golang.org/genproto/googleapis/cloud/tasks/v2"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
)

type (
	emailType   string
	emailAction string
)

const (
	// User denotes a User object type, for use in an EmailPayload
	User emailType = "User"
	// Event denotes a Event object type, for use in an EmailPayload
	Event emailType = "Event"
	// Thread denotes a Thread object type, for use in an EmailPayload
	Thread emailType = "Thread"

	// SendInvites denotes a SendInvites actoin, for use in an EmailPayload.
	// It can only be used when Event is the type.
	SendInvites emailAction = "SendInvites"
	// SendUpdatedInvites denotes a SendUpdatedInvites actoin, for use in an EmailPayload.
	// It can only be used when Event is the type.
	SendUpdatedInvites emailAction = "SendUpdatedInvites"
	// SendThread denotes a SendThread actoin, for use in an EmailPayload.
	// It can only be used when Thread is the type.
	SendThread emailAction = "SendThread"
	// SendWelcome denotes a SendWelcome actoin, for use in an EmailPayload
	// It can only be used when User is the type.
	SendWelcome emailAction = "SendWelcome"
)

var DefaultClient Client

func init() {
	if projectID := os.Getenv("GOOGLE_CLOUD_PROJECT"); projectID == "local-convo-api" || projectID == "" {
		DefaultClient = NewLocalClient()
	} else {
		DefaultClient = NewClient(context.Background(), projectID)
	}
}

func PutEmail(ctx context.Context, payload EmailPayload) error {
	return DefaultClient.PutEmail(ctx, payload)
}

// EmailPayload is a representation of an async email task.
type EmailPayload struct {
	// IDs is a slice of strings that are the result of calling *datastore.Key.Encode().
	IDs    []string    `json:"ids"`
	Type   emailType   `json:"type"`
	Action emailAction `json:"action"`
}

type Client interface {
	PutEmail(ctx context.Context, payload EmailPayload) error
}

type clientImpl struct {
	client *cloudtasks.Client
	path   string
}

func NewClient(ctx context.Context, projectID string) Client {
	tc, err := cloudtasks.NewClient(ctx)
	if err != nil {
		panic(errors.E(errors.Op("queue.NewClient"), err))
	}

	return &clientImpl{
		client: tc,
		path:   fmt.Sprintf("projects/%s/locations/us-central1/queues/convo-emails", projectID),
	}
}

// PutEmail enqueues an email to be sent.
func (c *clientImpl) PutEmail(ctx context.Context, payload EmailPayload) error {
	if payload.Type == Thread && payload.Action != SendThread {
		return fmt.Errorf("queue.PutEmail: '%v' is not a valid action for emailType.Thread", payload.Action)
	} else if payload.Type == Event && !(payload.Action == SendInvites || payload.Action == SendUpdatedInvites) {
		return fmt.Errorf("queue.PutEmail: '%v' is not a valid action for emailType.Event", payload.Action)
	} else if payload.Type == User && payload.Action != SendWelcome {
		return fmt.Errorf("queue.PutEmail: '%v' is not a valid action for emailType.User", payload.Action)
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue.PutEmail: %v", err)
	}

	req := &taskspb.CreateTaskRequest{
		Parent: c.path,
		Task: &taskspb.Task{
			// https://godoc.org/google.golang.org/genproto/googleapis/cloud/tasks/v2#AppEngineHttpRequest
			MessageType: &taskspb.Task_AppEngineHttpRequest{
				AppEngineHttpRequest: &taskspb.AppEngineHttpRequest{
					HttpMethod:  taskspb.HttpMethod_POST,
					RelativeUri: "/tasks/emails",
					Body:        jsonBytes,
				},
			},
		},
	}

	_, err = c.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("queue.PutEmail: %v", err)
	}

	return nil
}

type localClientImpl struct{}

func NewLocalClient() Client {
	log.Print("queue.NewLocalClient: USING QUEUE LOGGER FOR LOCAL DEVELOPMENT")
	return &localClientImpl{}
}

func (c *localClientImpl) PutEmail(ctx context.Context, payload EmailPayload) error {
	log.Printf("queue.PutEmail(EmailPayload{IDs=[], Type=%s, Action=%s})", payload.Type, payload.Action)
	return nil
}
