// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

type DummyTransferAdapter struct {
}

func (a *DummyTransferAdapter) Name() string {
	return "dummy"
}

func (a *DummyTransferAdapter) Download(ctx context.Context, r *ObjectResponse) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewBufferString("dummy")), nil
}

func TestHTTPClientDownload(t *testing.T) {
	oid := "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041"
	size := int64(6)

	roundTripHandler := func(req *http.Request) *http.Response {
		url := req.URL.String()
		if strings.Contains(url, "status-not-ok") {
			return &http.Response{StatusCode: http.StatusBadRequest}
		}
		if strings.Contains(url, "invalid-json-response") {
			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewBufferString("invalid json"))}
		}
		if strings.Contains(url, "valid-batch-request-download") {
			assert.Equal(t, "POST", req.Method)
			assert.Equal(t, MediaType, req.Header.Get("Content-type"), "case %s: error should match", url)
			assert.Equal(t, MediaType, req.Header.Get("Accept"), "case %s: error should match", url)

			var batchRequest BatchRequest
			err := json.NewDecoder(req.Body).Decode(&batchRequest)
			assert.NoError(t, err)

			assert.Equal(t, "download", batchRequest.Operation)
			assert.Equal(t, 1, len(batchRequest.Objects))
			assert.Equal(t, oid, batchRequest.Objects[0].Oid)
			assert.Equal(t, size, batchRequest.Objects[0].Size)

			batchResponse := &BatchResponse{
				Transfer: "dummy",
				Objects:  make([]*ObjectResponse, 1),
			}

			payload := new(bytes.Buffer)
			json.NewEncoder(payload).Encode(batchResponse)

			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(payload)}
		}
		if strings.Contains(url, "invalid-response-no-objects") {
			batchResponse := &BatchResponse{Transfer: "dummy"}

			payload := new(bytes.Buffer)
			json.NewEncoder(payload).Encode(batchResponse)

			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(payload)}
		}
		if strings.Contains(url, "unknown-transfer-adapter") {
			batchResponse := &BatchResponse{Transfer: "unknown_adapter"}

			payload := new(bytes.Buffer)
			json.NewEncoder(payload).Encode(batchResponse)

			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(payload)}
		}

		t.Errorf("Unknown test case: %s", url)

		return nil
	}

	hc := &http.Client{Transport: RoundTripFunc(roundTripHandler)}
	dummy := &DummyTransferAdapter{}

	var cases = []struct {
		endpoint      string
		expectederror string
	}{
		// case 0
		{
			endpoint:      "https://status-not-ok.io",
			expectederror: "Unexpected servers response: ",
		},
		// case 1
		{
			endpoint:      "https://invalid-json-response.io",
			expectederror: "json.Decode: ",
		},
		// case 2
		{
			endpoint:      "https://valid-batch-request-download.io",
			expectederror: "",
		},
		// case 3
		{
			endpoint:      "https://invalid-response-no-objects.io",
			expectederror: "No objects in result",
		},
		// case 4
		{
			endpoint:      "https://unknown-transfer-adapter.io",
			expectederror: "Transferadapter not found: ",
		},
	}

	for n, c := range cases {
		client := &HTTPClient{
			client:    hc,
			endpoint:  c.endpoint,
			transfers: make(map[string]TransferAdapter),
		}
		client.transfers["dummy"] = dummy

		_, err := client.Download(context.Background(), oid, size)
		if len(c.expectederror) > 0 {
			assert.True(t, strings.Contains(err.Error(), c.expectederror), "case %d: '%s' should contain '%s'", n, err.Error(), c.expectederror)
		} else {
			assert.NoError(t, err, "case %d", n)
		}
	}
}
