// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/pages"

	"github.com/stretchr/testify/require"
)

func TestPagesFormLimitRejectsOversizedRequestBeforeParsing(t *testing.T) {
	called := false
	handler := pagesFormLimit(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodPost, "/alice/site/settings/pages", strings.NewReader("body"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=pages")
	request.ContentLength = pages.DefaultMaxCompressedSize + (1 * 1024 * 1024) + 1
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.False(t, called)
}
