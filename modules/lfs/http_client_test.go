// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
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
	return nil
}

func lfsTestRoundtripHandler(req *http.Request) *http.Response {
	var batchResponse *BatchResponse
	url := req.URL.String()

	if strings.Contains(url, "status-not-ok") {
		return &http.Response{StatusCode: http.StatusBadRequest}
	} else if strings.Contains(url, "invalid-json-response") {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("invalid json"))}
	} else if strings.Contains(url, "valid-batch-request-download") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Actions: map[string]*Link{
						"download": {},
					},
				},
			},
		}
	} else if strings.Contains(url, "legacy-batch-request-download") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Links: map[string]*Link{
						"download": {},
					},
				},
			},
		}
	} else if strings.Contains(url, "valid-batch-request-upload") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Actions: map[string]*Link{
						"upload": {},
					},
				},
			},
		}
	} else if strings.Contains(url, "response-no-objects") {
		batchResponse = &BatchResponse{Transfer: "dummy"}
	} else if strings.Contains(url, "unknown-transfer-adapter") {
		batchResponse = &BatchResponse{Transfer: "unknown_adapter"}
	} else if strings.Contains(url, "error-in-response-objects") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Error: &ObjectError{
						Code:    http.StatusNotFound,
						Message: "Object not found",
					},
				},
			},
		}
	} else if strings.Contains(url, "empty-actions-map") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Actions: map[string]*Link{},
				},
			},
		}
	} else if strings.Contains(url, "download-actions-map") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Actions: map[string]*Link{
						"download": {},
					},
				},
			},
		}
	} else if strings.Contains(url, "upload-actions-map") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Actions: map[string]*Link{
						"upload": {},
					},
				},
			},
		}
	} else if strings.Contains(url, "verify-actions-map") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Actions: map[string]*Link{
						"verify": {},
					},
				},
			},
		}
	} else if strings.Contains(url, "unknown-actions-map") {
		batchResponse = &BatchResponse{
			Transfer: "dummy",
			Objects: []*ObjectResponse{
				{
					Actions: map[string]*Link{
						"unknown": {},
					},
				},
			},
		}
	} else {
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
	}
	for _, operation := range []struct {
		name  string
		cases []clientCase
	}{
		{
			name: "download",
			cases: []clientCase{
				{"https://status-not-ok.io", io.ErrUnexpectedEOF.Error()},
				{"https://invalid-json-response.io", "/(invalid json|invalid character)/"},
				{"https://valid-batch-request-download.io", ""},
				{"https://response-no-objects.io", ""},
				{"https://unknown-transfer-adapter.io", "TransferAdapter not found: "},
				{"https://error-in-response-objects.io", "Object not found"},
				{"https://empty-actions-map.io", "missing action 'download'"},
				{"https://download-actions-map.io", ""},
				{"https://upload-actions-map.io", "missing action 'download'"},
				{"https://verify-actions-map.io", "missing action 'download'"},
				{"https://unknown-actions-map.io", "missing action 'download'"},
				{"https://legacy-batch-request-download.io", ""},
			},
		},
		{
			name: "upload",
			cases: []clientCase{
				{"https://status-not-ok.io", io.ErrUnexpectedEOF.Error()},
				{"https://invalid-json-response.io", "/(invalid json|invalid character)/"},
				{"https://valid-batch-request-upload.io", ""},
				{"https://response-no-objects.io", ""},
				{"https://unknown-transfer-adapter.io", "TransferAdapter not found: "},
				{"https://error-in-response-objects.io", "Object not found"},
				{"https://empty-actions-map.io", ""},
				{"https://download-actions-map.io", "missing action 'upload'"},
				{"https://upload-actions-map.io", ""},
				{"https://verify-actions-map.io", "missing action 'upload'"},
				{"https://unknown-actions-map.io", "missing action 'upload'"},
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			p := Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6}

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
				assert.Len(t, batchRequest.Objects, 1)
				assert.Equal(t, p.Oid, batchRequest.Objects[0].Oid)
				assert.Equal(t, p.Size, batchRequest.Objects[0].Size)

				return lfsTestRoundtripHandler(req)
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

					var err error
					if operation.name == "download" {
						err = client.Download(t.Context(), "refs/heads/main", []Pointer{p}, func(p Pointer, content io.ReadCloser, objectError error) error {
							if objectError != nil {
								return objectError
							}
							b, err := io.ReadAll(content)
							assert.NoError(t, err)
							assert.Equal(t, []byte("dummy"), b)
							return nil
						})
					} else {
						err = client.Upload(t.Context(), []Pointer{p}, func(p Pointer, objectError error) (io.ReadCloser, error) {
							return io.NopCloser(new(bytes.Buffer)), objectError
						})
					}
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
