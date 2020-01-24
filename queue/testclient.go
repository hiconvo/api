package queue

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/googleapis/gax-go/v2"
	taskspb "google.golang.org/genproto/googleapis/cloud/tasks/v2"

	"github.com/hiconvo/api/utils/thelpers"
)

// testTaskClient implements taskCreator in tests.
type testTaskClient struct {
	cloudtasks.Client
}

func (c testTaskClient) CreateTask(ctx context.Context, req *taskspb.CreateTaskRequest, opts ...gax.CallOption) (*taskspb.Task, error) {
	if thelpers.IsTesting() {
		return &taskspb.Task{}, nil
	}

	body := req.GetTask().GetAppEngineHttpRequest().GetBody()
	bytesReader := bytes.NewReader(body)
	httpReq, err := http.NewRequest(http.MethodPost, "http://localhost:8080/tasks/emails", bytesReader)
	if err != nil {
		return &taskspb.Task{}, fmt.Errorf("testTaskClient.CreateTask: %v", err)
	}
	httpReq.Header.Add("content-type", "application/json")
	httpReq.Header.Add("X-Appengine-QueueName", "convo-emails")
	_, err = http.DefaultClient.Do(httpReq)
	if err != nil {
		return &taskspb.Task{}, fmt.Errorf("testTaskClient.CreateTask: %v", err)
	}

	return &taskspb.Task{}, nil
}
