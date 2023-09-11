// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeContentByReader(t *testing.T) {
	data := "0123456789abcdef"

	test := func(t *testing.T, expectedStatusCode int, expectedContent string) {
		_, rangeStr, _ := strings.Cut(t.Name(), "_range_")
		r := &http.Request{Header: http.Header{}, Form: url.Values{}}
		if rangeStr != "" {
			r.Header.Set("Range", fmt.Sprintf("bytes=%s", rangeStr))
		}
		reader := strings.NewReader(data)
		w := httptest.NewRecorder()
		ServeContentByReader(r, w, "test", int64(len(data)), reader)
		assert.Equal(t, expectedStatusCode, w.Code)
		if expectedStatusCode == http.StatusPartialContent || expectedStatusCode == http.StatusOK {
			assert.Equal(t, fmt.Sprint(len(expectedContent)), w.Header().Get("Content-Length"))
			assert.Equal(t, expectedContent, w.Body.String())
		}
	}

	t.Run("_range_", func(t *testing.T) {
		test(t, http.StatusOK, data)
	})
	t.Run("_range_0-", func(t *testing.T) {
		test(t, http.StatusPartialContent, data)
	})
	t.Run("_range_0-15", func(t *testing.T) {
		test(t, http.StatusPartialContent, data)
	})
	t.Run("_range_1-", func(t *testing.T) {
		test(t, http.StatusPartialContent, data[1:])
	})
	t.Run("_range_1-3", func(t *testing.T) {
		test(t, http.StatusPartialContent, data[1:3+1])
	})
	t.Run("_range_16-", func(t *testing.T) {
		test(t, http.StatusRequestedRangeNotSatisfiable, "")
	})
	t.Run("_range_1-99999", func(t *testing.T) {
		test(t, http.StatusPartialContent, data[1:])
	})
}

func TestServeContentByReadSeeker(t *testing.T) {
	data := "0123456789abcdef"
	tmpFile := t.TempDir() + "/test"
	err := os.WriteFile(tmpFile, []byte(data), 0o644)
	assert.NoError(t, err)

	test := func(t *testing.T, expectedStatusCode int, expectedContent string) {
		_, rangeStr, _ := strings.Cut(t.Name(), "_range_")
		r := &http.Request{Header: http.Header{}, Form: url.Values{}}
		if rangeStr != "" {
			r.Header.Set("Range", fmt.Sprintf("bytes=%s", rangeStr))
		}

		seekReader, err := os.OpenFile(tmpFile, os.O_RDONLY, 0o644)
		if !assert.NoError(t, err) {
			return
		}
		defer seekReader.Close()

		w := httptest.NewRecorder()
		ServeContentByReadSeeker(r, w, "test", nil, seekReader)
		assert.Equal(t, expectedStatusCode, w.Code)
		if expectedStatusCode == http.StatusPartialContent || expectedStatusCode == http.StatusOK {
			assert.Equal(t, fmt.Sprint(len(expectedContent)), w.Header().Get("Content-Length"))
			assert.Equal(t, expectedContent, w.Body.String())
		}
	}

	t.Run("_range_", func(t *testing.T) {
		test(t, http.StatusOK, data)
	})
	t.Run("_range_0-", func(t *testing.T) {
		test(t, http.StatusPartialContent, data)
	})
	t.Run("_range_0-15", func(t *testing.T) {
		test(t, http.StatusPartialContent, data)
	})
	t.Run("_range_1-", func(t *testing.T) {
		test(t, http.StatusPartialContent, data[1:])
	})
	t.Run("_range_1-3", func(t *testing.T) {
		test(t, http.StatusPartialContent, data[1:3+1])
	})
	t.Run("_range_16-", func(t *testing.T) {
		test(t, http.StatusRequestedRangeNotSatisfiable, "")
	})
	t.Run("_range_1-99999", func(t *testing.T) {
		test(t, http.StatusPartialContent, data[1:])
	})
}
