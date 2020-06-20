package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	User   emailType = "User"
	Event  emailType = "Event"
	Thread emailType = "Thread"

	SendInvites        emailAction = "SendInvites"
	SendUpdatedInvites emailAction = "SendUpdatedInvites"
	SendThread         emailAction = "SendThread"
	SendWelcome        emailAction = "SendWelcome"
)

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
	if projectID == "local-convo-api" {
		return NewLogger()
	}

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
	op := errors.Opf("queue.PutEmail(type=%s, action=%s)", payload.Type, payload.Action)

	if payload.Type == Thread && payload.Action != SendThread {
		return errors.E(op, errors.Errorf("'%v' is not a valid action for emailType.Thread", payload.Action))
	} else if payload.Type == Event && !(payload.Action == SendInvites || payload.Action == SendUpdatedInvites) {
		return errors.E(op, errors.Errorf("'%v' is not a valid action for emailType.Event", payload.Action))
	} else if payload.Type == User && payload.Action != SendWelcome {
		return errors.E(op, errors.Errorf("'%v' is not a valid action for emailType.User", payload.Action))
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.E(op, err)
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
		return errors.E(op, err)
	}

	return nil
}

type loggerImpl struct{}

func NewLogger() Client {
	log.Print("queue.NewLogger: USING QUEUE LOGGER FOR LOCAL DEVELOPMENT")
	return &loggerImpl{}
}

func (c *loggerImpl) PutEmail(ctx context.Context, payload EmailPayload) error {
	log.Printf("queue.PutEmail(IDs=[%s], Type=%s, Action=%s)",
		strings.Join(payload.IDs, ", "), payload.Type, payload.Action)
	return nil
}
