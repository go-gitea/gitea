// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicTransferAdapterName(t *testing.T) {
	a := &BasicTransferAdapter{}

	assert.Equal(t, "basic", a.Name())
}

func TestBasicTransferAdapterDownload(t *testing.T) {
	roundTripHandler := func(req *http.Request) *http.Response {
		url := req.URL.String()
		if strings.Contains(url, "valid-download-request") {
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, "test-value", req.Header.Get("test-header"))

			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewBufferString("dummy"))}
		}

		t.Errorf("Unknown test case: %s", url)

		return nil
	}

	hc := &http.Client{Transport: RoundTripFunc(roundTripHandler)}
	a := &BasicTransferAdapter{hc}

	var cases = []struct {
		response      *ObjectResponse
		expectederror string
	}{
		// case 0
		{
			response:      &ObjectResponse{},
			expectederror: "Action 'download' not found",
		},
		// case 1
		{
			response: &ObjectResponse{
				Actions: map[string]*Link{"upload": nil},
			},
			expectederror: "Action 'download' not found",
		},
		// case 2
		{
			response: &ObjectResponse{
				Actions: map[string]*Link{"download": {
					Href:   "https://valid-download-request.io",
					Header: map[string]string{"test-header": "test-value"},
				}},
			},
			expectederror: "",
		},
	}

	for n, c := range cases {
		_, err := a.Download(context.Background(), c.response)
		if len(c.expectederror) > 0 {
			assert.True(t, strings.Contains(err.Error(), c.expectederror), "case %d: '%s' should contain '%s'", n, err.Error(), c.expectederror)
		} else {
			assert.NoError(t, err, "case %d", n)
		}
	}
}
