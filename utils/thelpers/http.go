package thelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEndpoint sends a request to the given endpoint with the given
// parameters. It automatically binds the request to given context
// and encodes and decodes the request/response payloads.
func TestEndpoint(
	t *testing.T,
	ctx context.Context,
	handler http.Handler,
	method string,
	url string,
	data interface{},
	headers map[string]string,
) (
	*http.Request,
	*httptest.ResponseRecorder,
	map[string]interface{},
) {
	req := createTestRequest(t, ctx, method, url, data, headers)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var respData map[string]interface{}

	err := json.Unmarshal(rr.Body.Bytes(), &respData)
	if err != nil {
		t.Fatal(err)
	}

	return req, rr, respData
}

func createTestRequest(t *testing.T, ctx context.Context, method, path string,
	body interface{}, headers map[string]string) *http.Request {
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.WithContext(ctx)

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	return req
}
