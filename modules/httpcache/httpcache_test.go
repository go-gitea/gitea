// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httpcache

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockFileInfo struct{}

func (m mockFileInfo) Name() string       { return "gitea.test" }
func (m mockFileInfo) Size() int64        { return int64(10) }
func (m mockFileInfo) Mode() os.FileMode  { return os.ModePerm }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() interface{}   { return nil }

func countFormalHeaders(h http.Header) (c int) {
	for k := range h {
		// ignore our headers for internal usage
		if strings.HasPrefix(k, "X-Gitea-") {
			continue
		}
		c++
	}
	return c
}

func TestHandleFileETagCache(t *testing.T) {
	fi := mockFileInfo{}
	etag := `"MTBnaXRlYS50ZXN0TW9uLCAwMSBKYW4gMDAwMSAwMDowMDowMCBHTVQ="`

	t.Run("No_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		handled := HandleFileETagCache(req, w, fi)

		assert.False(t, handled)
		assert.Equal(t, 2, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Cache-Control")
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
	})
	t.Run("Wrong_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		req.Header.Set("If-None-Match", `"wrong etag"`)

		handled := HandleFileETagCache(req, w, fi)

		assert.False(t, handled)
		assert.Equal(t, 2, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Cache-Control")
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
	})
	t.Run("Correct_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		req.Header.Set("If-None-Match", etag)

		handled := HandleFileETagCache(req, w, fi)

		assert.True(t, handled)
		assert.Equal(t, 1, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
		assert.Equal(t, http.StatusNotModified, w.Code)
	})
}

func TestHandleGenericETagCache(t *testing.T) {
	etag := `"test"`

	t.Run("No_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		handled := HandleGenericETagCache(req, w, etag)

		assert.False(t, handled)
		assert.Equal(t, 2, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Cache-Control")
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
	})
	t.Run("Wrong_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		req.Header.Set("If-None-Match", `"wrong etag"`)

		handled := HandleGenericETagCache(req, w, etag)

		assert.False(t, handled)
		assert.Equal(t, 2, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Cache-Control")
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
	})
	t.Run("Correct_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		req.Header.Set("If-None-Match", etag)

		handled := HandleGenericETagCache(req, w, etag)

		assert.True(t, handled)
		assert.Equal(t, 1, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
		assert.Equal(t, http.StatusNotModified, w.Code)
	})
	t.Run("Multiple_Wrong_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		req.Header.Set("If-None-Match", `"wrong etag", "wrong etag "`)

		handled := HandleGenericETagCache(req, w, etag)

		assert.False(t, handled)
		assert.Equal(t, 2, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Cache-Control")
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
	})
	t.Run("Multiple_Correct_If-None-Match", func(t *testing.T) {
		req := &http.Request{Header: make(http.Header)}
		w := httptest.NewRecorder()

		req.Header.Set("If-None-Match", `"wrong etag", `+etag)

		handled := HandleGenericETagCache(req, w, etag)

		assert.True(t, handled)
		assert.Equal(t, 1, countFormalHeaders(w.Header()))
		assert.Contains(t, w.Header(), "Etag")
		assert.Equal(t, etag, w.Header().Get("Etag"))
		assert.Equal(t, http.StatusNotModified, w.Code)
	})
}
