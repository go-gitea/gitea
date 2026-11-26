// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

type DummyTransferAdapter struct{}

func (a *DummyTransferAdapter) Name() string {
	return "dummy"
}

func (a *DummyTransferAdapter) Download(ctx context.Context, l *Link) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("dummy")), nil
}

func (a *DummyTransferAdapter) Upload(ctx context.Context, l *Link, p Pointer, r io.Reader) error {
	return nil
}

func (a *DummyTransferAdapter) Verify(ctx context.Context, l *Link, p Pointer) error {
	if l == nil {
		return errors.New("Verify called with a nil link")
	}
	return nil
}

func lfsTestRoundtripHandler(req *http.Request, pointer Pointer) *http.Response {
	object := &ObjectResponse{Pointer: pointer}
	batchResponse := &BatchResponse{Transfer: "dummy", Objects: []*ObjectResponse{object}}
	switch req.URL.Hostname() {
	case "status-not-ok.io":
		return &http.Response{StatusCode: http.StatusBadRequest}
	case "plaintext-error.io":
		return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("Not Found"))}
	case "invalid-json-response.io":
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("invalid json"))}
	case "duplicate-oid-with-wrong-size.io":
		object.Size++
		batchResponse.Objects = append(batchResponse.Objects, object)
		fallthrough
	case "download-actions-map.io":
		object.Actions = map[string]*Link{"download": {}}
	case "legacy-batch-request-download.io":
		object.Actions = map[string]*Link{"download": nil}
		object.Links = map[string]*Link{"download": {}}
	case "upload-actions-map.io":
		object.Actions = map[string]*Link{"upload": {}}
	case "upload-with-null-verify.io":
		object.Actions = map[string]*Link{"upload": {}, "verify": nil}
	case "response-no-objects.io":
		batchResponse.Objects = nil
	case "unknown-transfer-adapter.io":
		batchResponse.Transfer = "unknown_adapter"
	case "error-in-response-objects.io":
		object.Error = &ObjectError{Code: http.StatusNotFound, Message: "Object not found"}
	case "empty-actions-map.io":
		object.Actions = map[string]*Link{}
	case "verify-actions-map.io":
		object.Actions = map[string]*Link{"verify": {}, "upload": nil}
	case "unknown-actions-map.io":
		object.Actions = map[string]*Link{"unknown": {}}
	case "unknown-oid.io":
		object.Actions = map[string]*Link{"download": {}}
		batchResponse.Objects = append(batchResponse.Objects, &ObjectResponse{Pointer: Pointer{Oid: strings.Repeat("a", 64)}})
	case "null-object.io":
		object.Actions = map[string]*Link{"download": {}}
		batchResponse.Objects = append(batchResponse.Objects, nil)
	case "null-download-link.io":
		object.Actions = map[string]*Link{"download": nil}
		object.Links = map[string]*Link{"download": nil}
	default:
		return nil
	}

	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(batchResponse)

	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(payload)}
}

func TestHTTPClientTransfers(t *testing.T) {
	type clientCase struct {
		endpoint      string
		expectedError string
		callbacks     int32
	}
	for _, operation := range []struct {
		name  string
		cases []clientCase
	}{
		{
			name: "download",
			cases: []clientCase{
				{"https://status-not-ok.io", `LFS server returned error, status: 400, body: ""`, 0},
				{"https://plaintext-error.io", `LFS server returned error, status: 404, body: "Not Found"`, 0},
				{"https://invalid-json-response.io", "/(invalid json|invalid character)/", 0},
				{"https://duplicate-oid-with-wrong-size.io", "", 1},
				{"https://response-no-objects.io", "", 0},
				{"https://unknown-transfer-adapter.io", "TransferAdapter not found: ", 0},
				{"https://error-in-response-objects.io", "Object not found", 1},
				{"https://empty-actions-map.io", "missing action 'download'", 0},
				{"https://download-actions-map.io", "", 1},
				{"https://upload-actions-map.io", "missing action 'download'", 0},
				{"https://verify-actions-map.io", "missing action 'download'", 0},
				{"https://unknown-actions-map.io", "missing action 'download'", 0},
				{"https://legacy-batch-request-download.io", "", 1},
				{"https://unknown-oid.io", "unrequested oid", 0},
				{"https://null-object.io", "null object", 0},
				{"https://null-download-link.io", "missing action 'download'", 0},
			},
		},
		{
			name: "upload",
			cases: []clientCase{
				{"https://status-not-ok.io", `LFS server returned error, status: 400, body: ""`, 0},
				{"https://plaintext-error.io", `LFS server returned error, status: 404, body: "Not Found"`, 0},
				{"https://invalid-json-response.io", "/(invalid json|invalid character)/", 0},
				{"https://upload-with-null-verify.io", "", 1},
				{"https://response-no-objects.io", "", 0},
				{"https://unknown-transfer-adapter.io", "TransferAdapter not found: ", 0},
				{"https://error-in-response-objects.io", "Object not found", 1},
				{"https://empty-actions-map.io", "", 0},
				{"https://download-actions-map.io", "missing action 'upload'", 0},
				{"https://upload-actions-map.io", "", 1},
				{"https://verify-actions-map.io", "missing action 'upload'", 0},
				{"https://unknown-actions-map.io", "missing action 'upload'", 0},
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			requested := Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6}

			hc := &http.Client{Transport: RoundTripFunc(func(req *http.Request) *http.Response {
				assert.Equal(t, "POST", req.Method)
				assert.Equal(t, MediaType, req.Header.Get("Content-type"))
				assert.Equal(t, AcceptHeader, req.Header.Get("Accept"))

				var batchRequest BatchRequest
				err := json.NewDecoder(req.Body).Decode(&batchRequest)
				assert.NoError(t, err)

				assert.Equal(t, operation.name, batchRequest.Operation)
				if operation.name == "download" {
					assert.Equal(t, &Reference{Name: "refs/heads/main"}, batchRequest.Ref)
				} else {
					assert.Nil(t, batchRequest.Ref)
				}
				assert.Equal(t, []Pointer{requested}, batchRequest.Objects)

				return lfsTestRoundtripHandler(req, requested)
			})}
			dummy := &DummyTransferAdapter{}

			defer test.MockVariableValue(&setting.LFSClient.BatchOperationConcurrency, 8)()
			for _, testCase := range operation.cases {
				t.Run(testCase.endpoint, func(t *testing.T) {
					client := &HTTPClient{
						client:   hc,
						endpoint: testCase.endpoint,
						transfers: map[string]TransferAdapter{
							"dummy": dummy,
						},
					}

					var callbacks atomic.Int32
					checkPointer := func(pointer Pointer) {
						callbacks.Add(1)
						assert.Equal(t, requested, pointer)
					}
					var err error
					if operation.name == "download" {
						err = client.Download(t.Context(), "refs/heads/main", []Pointer{requested}, func(pointer Pointer, content io.ReadCloser, objectError error) error {
							checkPointer(pointer)
							if objectError != nil {
								return objectError
							}
							b, err := io.ReadAll(content)
							assert.NoError(t, err)
							assert.Equal(t, []byte("dummy"), b)
							return nil
						})
					} else {
						err = client.Upload(t.Context(), []Pointer{requested}, func(pointer Pointer, objectError error) (io.ReadCloser, error) {
							checkPointer(pointer)
							return io.NopCloser(new(bytes.Buffer)), objectError
						})
					}
					assert.Equal(t, testCase.callbacks, callbacks.Load())
					if testCase.expectedError != "" {
						if strings.HasPrefix(testCase.expectedError, "/") && strings.HasSuffix(testCase.expectedError, "/") {
							assert.Regexp(t, strings.Trim(testCase.expectedError, "/"), err.Error())
						} else {
							assert.ErrorContains(t, err, testCase.expectedError)
						}
					} else {
						assert.NoError(t, err)
					}
				})
			}
		})
	}
}
