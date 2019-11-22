package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/googleapis/gax-go/v2"
	taskspb "google.golang.org/genproto/googleapis/cloud/tasks/v2"
)

type taskMaster interface {
	CreateTask(context.Context, *taskspb.CreateTaskRequest, ...gax.CallOption) (*taskspb.Task, error)
}

type testTaskClient struct {
	cloudtasks.Client
}

func (c testTaskClient) CreateTask(ctx context.Context, req *taskspb.CreateTaskRequest, opts ...gax.CallOption) (*taskspb.Task, error) {
	return &taskspb.Task{}, nil
}

type emailType string

type emailAction string

var (
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

// EmailPayload is a representation of an async email task
type EmailPayload struct {
	// IDs is a slice of strings that are the result of calling *datastore.Key.Encode().
	IDs    []string    `json:"ids"`
	Type   emailType   `json:"type"`
	Action emailAction `json:"action"`
}

var (
	_client    taskMaster
	_queuePath string
)

func init() {
	ctx := context.Background()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "local-convo-api"
	}

	var client taskMaster
	var err error
	if strings.HasSuffix(os.Args[0], ".test") {
		client = testTaskClient{}
	} else {
		client, err = cloudtasks.NewClient(ctx)
	}
	if err != nil {
		panic(err)
	}

	_client = client
	_queuePath = fmt.Sprintf("projects/%s/locations/us-central1/queues/convo-emails", projectID)
}

// PutEmail enqueues an email to be sent
func PutEmail(ctx context.Context, payload EmailPayload) error {
	if payload.Type == Thread && payload.Action != SendThread {
		return fmt.Errorf("'%v' is not a valid action for emailType.Thread", payload.Action)
	} else if payload.Type == Event && !(payload.Action == SendInvites || payload.Action == SendUpdatedInvites) {
		return fmt.Errorf("'%v' is not a valid action for emailType.Event", payload.Action)
	} else if payload.Type == User && payload.Action != SendWelcome {
		return fmt.Errorf("'%v' is not a valid action for emailType.User", payload.Action)
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue.PutEmail: %v", err)
	}

	req := &taskspb.CreateTaskRequest{
		Parent: _queuePath,
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

	_, err = _client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("queue.PutEmail: %v", err)
	}

	return nil
}
