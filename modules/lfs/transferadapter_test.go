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

func TestBasicTransferAdapterName(t *testing.T) {
	a := &BasicTransferAdapter{}

	assert.Equal(t, "basic", a.Name())
}

func TestBasicTransferAdapter(t *testing.T) {
	p := Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6}

	roundTripHandler := func(req *http.Request) *http.Response {
		assert.Equal(t, "test-value", req.Header.Get("test-header"))

		url := req.URL.String()
		if strings.Contains(url, "download-request") {
			assert.Equal(t, "GET", req.Method)

			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewBufferString("dummy"))}
		} else if strings.Contains(url, "upload-request") {
			assert.Equal(t, "PUT", req.Method)

			b, err := io.ReadAll(req.Body)
			assert.NoError(t, err)
			assert.Equal(t, "dummy", string(b))

			return &http.Response{StatusCode: http.StatusOK}
		} else if strings.Contains(url, "verify-request") {
			assert.Equal(t, "POST", req.Method)

			var vp Pointer
			err := json.NewDecoder(req.Body).Decode(&vp)
			assert.NoError(t, err)
			assert.Equal(t, p.Oid, vp.Oid)
			assert.Equal(t, p.Size, vp.Size)

			return &http.Response{StatusCode: http.StatusOK}
		} else if strings.Contains(url, "error-response") {
			er := &ErrorResponse{
				Message: "Object not found",
			}
			payload := new(bytes.Buffer)
			json.NewEncoder(payload).Encode(er)

			return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(payload)}
		} else {
			t.Errorf("Unknown test case: %s", url)
			return nil
		}
	}

	hc := &http.Client{Transport: RoundTripFunc(roundTripHandler)}
	a := &BasicTransferAdapter{hc}

	t.Run("Download", func(t *testing.T) {
		cases := []struct {
			link          *Link
			expectederror string
		}{
			// case 0
			{
				link: &Link{
					Href:   "https://download-request.io",
					Header: map[string]string{"test-header": "test-value"},
				},
				expectederror: "",
			},
			// case 1
			{
				link: &Link{
					Href:   "https://error-response.io",
					Header: map[string]string{"test-header": "test-value"},
				},
				expectederror: "Object not found",
			},
		}

		for n, c := range cases {
			_, err := a.Download(context.Background(), c.link)
			if len(c.expectederror) > 0 {
				assert.True(t, strings.Contains(err.Error(), c.expectederror), "case %d: '%s' should contain '%s'", n, err.Error(), c.expectederror)
			} else {
				assert.NoError(t, err, "case %d", n)
			}
		}
	})

	t.Run("Upload", func(t *testing.T) {
		cases := []struct {
			link          *Link
			expectederror string
		}{
			// case 0
			{
				link: &Link{
					Href:   "https://upload-request.io",
					Header: map[string]string{"test-header": "test-value"},
				},
				expectederror: "",
			},
			// case 1
			{
				link: &Link{
					Href:   "https://error-response.io",
					Header: map[string]string{"test-header": "test-value"},
				},
				expectederror: "Object not found",
			},
		}

		for n, c := range cases {
			err := a.Upload(context.Background(), c.link, bytes.NewBuffer([]byte("dummy")))
			if len(c.expectederror) > 0 {
				assert.True(t, strings.Contains(err.Error(), c.expectederror), "case %d: '%s' should contain '%s'", n, err.Error(), c.expectederror)
			} else {
				assert.NoError(t, err, "case %d", n)
			}
		}
	})

	t.Run("Verify", func(t *testing.T) {
		cases := []struct {
			link          *Link
			expectederror string
		}{
			// case 0
			{
				link: &Link{
					Href:   "https://verify-request.io",
					Header: map[string]string{"test-header": "test-value"},
				},
				expectederror: "",
			},
			// case 1
			{
				link: &Link{
					Href:   "https://error-response.io",
					Header: map[string]string{"test-header": "test-value"},
				},
				expectederror: "Object not found",
			},
		}

		for n, c := range cases {
			err := a.Verify(context.Background(), c.link, p)
			if len(c.expectederror) > 0 {
				assert.True(t, strings.Contains(err.Error(), c.expectederror), "case %d: '%s' should contain '%s'", n, err.Error(), c.expectederror)
			} else {
				assert.NoError(t, err, "case %d", n)
			}
		}
	})
}
