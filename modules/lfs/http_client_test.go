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

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

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
	return io.NopCloser(bytes.NewBufferString("dummy")), nil
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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("invalid json"))}
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

func TestHTTPClientDownload(t *testing.T) {
	p := Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6}

	hc := &http.Client{Transport: RoundTripFunc(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, MediaType, req.Header.Get("Content-type"))
		assert.Equal(t, AcceptHeader, req.Header.Get("Accept"))

		var batchRequest BatchRequest
		err := json.NewDecoder(req.Body).Decode(&batchRequest)
		assert.NoError(t, err)

		assert.Equal(t, "download", batchRequest.Operation)
		assert.Len(t, batchRequest.Objects, 1)
		assert.Equal(t, p.Oid, batchRequest.Objects[0].Oid)
		assert.Equal(t, p.Size, batchRequest.Objects[0].Size)

		return lfsTestRoundtripHandler(req)
	})}
	dummy := &DummyTransferAdapter{}

	cases := []struct {
		endpoint      string
		expectedError string
	}{
		{
			endpoint:      "https://status-not-ok.io",
			expectedError: io.ErrUnexpectedEOF.Error(),
		},
		{
			endpoint:      "https://invalid-json-response.io",
			expectedError: "invalid json",
		},
		{
			endpoint:      "https://valid-batch-request-download.io",
			expectedError: "",
		},
		{
			endpoint:      "https://response-no-objects.io",
			expectedError: "",
		},
		{
			endpoint:      "https://unknown-transfer-adapter.io",
			expectedError: "TransferAdapter not found: ",
		},
		{
			endpoint:      "https://error-in-response-objects.io",
			expectedError: "Object not found",
		},
		{
			endpoint:      "https://empty-actions-map.io",
			expectedError: "missing action 'download'",
		},
		{
			endpoint:      "https://download-actions-map.io",
			expectedError: "",
		},
		{
			endpoint:      "https://upload-actions-map.io",
			expectedError: "missing action 'download'",
		},
		{
			endpoint:      "https://verify-actions-map.io",
			expectedError: "missing action 'download'",
		},
		{
			endpoint:      "https://unknown-actions-map.io",
			expectedError: "missing action 'download'",
		},
		{
			endpoint:      "https://legacy-batch-request-download.io",
			expectedError: "",
		},
	}

	defer test.MockVariableValue(&setting.LFSClient.BatchOperationConcurrency, 8)()
	for _, c := range cases {
		t.Run(c.endpoint, func(t *testing.T) {
			client := &HTTPClient{
				client:   hc,
				endpoint: c.endpoint,
				transfers: map[string]TransferAdapter{
					"dummy": dummy,
				},
			}

			err := client.Download(context.Background(), []Pointer{p}, func(p Pointer, content io.ReadCloser, objectError error) error {
				if objectError != nil {
					return objectError
				}
				b, err := io.ReadAll(content)
				assert.NoError(t, err)
				assert.Equal(t, []byte("dummy"), b)
				return nil
			})
			if c.expectedError != "" {
				assert.ErrorContains(t, err, c.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClientUpload(t *testing.T) {
	p := Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6}

	hc := &http.Client{Transport: RoundTripFunc(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, MediaType, req.Header.Get("Content-type"))
		assert.Equal(t, AcceptHeader, req.Header.Get("Accept"))

		var batchRequest BatchRequest
		err := json.NewDecoder(req.Body).Decode(&batchRequest)
		assert.NoError(t, err)

		assert.Equal(t, "upload", batchRequest.Operation)
		assert.Len(t, batchRequest.Objects, 1)
		assert.Equal(t, p.Oid, batchRequest.Objects[0].Oid)
		assert.Equal(t, p.Size, batchRequest.Objects[0].Size)

		return lfsTestRoundtripHandler(req)
	})}
	dummy := &DummyTransferAdapter{}

	cases := []struct {
		endpoint      string
		expectedError string
	}{
		{
			endpoint:      "https://status-not-ok.io",
			expectedError: io.ErrUnexpectedEOF.Error(),
		},
		{
			endpoint:      "https://invalid-json-response.io",
			expectedError: "invalid json",
		},
		{
			endpoint:      "https://valid-batch-request-upload.io",
			expectedError: "",
		},
		{
			endpoint:      "https://response-no-objects.io",
			expectedError: "",
		},
		{
			endpoint:      "https://unknown-transfer-adapter.io",
			expectedError: "TransferAdapter not found: ",
		},
		{
			endpoint:      "https://error-in-response-objects.io",
			expectedError: "Object not found",
		},
		{
			endpoint:      "https://empty-actions-map.io",
			expectedError: "",
		},
		{
			endpoint:      "https://download-actions-map.io",
			expectedError: "missing action 'upload'",
		},
		{
			endpoint:      "https://upload-actions-map.io",
			expectedError: "",
		},
		{
			endpoint:      "https://verify-actions-map.io",
			expectedError: "missing action 'upload'",
		},
		{
			endpoint:      "https://unknown-actions-map.io",
			expectedError: "missing action 'upload'",
		},
	}

	defer test.MockVariableValue(&setting.LFSClient.BatchOperationConcurrency, 8)()
	for _, c := range cases {
		t.Run(c.endpoint, func(t *testing.T) {
			client := &HTTPClient{
				client:   hc,
				endpoint: c.endpoint,
				transfers: map[string]TransferAdapter{
					"dummy": dummy,
				},
			}

			err := client.Upload(context.Background(), []Pointer{p}, func(p Pointer, objectError error) (io.ReadCloser, error) {
				return io.NopCloser(new(bytes.Buffer)), objectError
			})
			if c.expectedError != "" {
				assert.ErrorContains(t, err, c.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
