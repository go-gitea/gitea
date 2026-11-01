// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/pages"
	setting_module "gitea.dev/modules/setting"

	"github.com/stretchr/testify/require"
)

func TestPagesUploadFormLimitRejectsKnownOversizedRequest(t *testing.T) {
	previousLimit := setting_module.Pages.MaxTarGzSize
	setting_module.Pages.MaxTarGzSize = pages.DefaultMaxCompressedSize
	t.Cleanup(func() { setting_module.Pages.MaxTarGzSize = previousLimit })

	called := false
	handler := PagesUploadFormLimit(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodPost, "/alice/site/settings/pages/versions/upload", strings.NewReader("body"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=pages")
	request.ContentLength = pages.DefaultMaxCompressedSize + pagesUploadFormOverhead + 1
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.False(t, called)
}

func TestPagesUploadFormLimitRejectsUnknownLengthOversizedRequest(t *testing.T) {
	previousLimit := setting_module.Pages.MaxTarGzSize
	setting_module.Pages.MaxTarGzSize = 16
	t.Cleanup(func() { setting_module.Pages.MaxTarGzSize = previousLimit })

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "pages.tar.gz")
	require.NoError(t, err)
	_, err = file.Write(make([]byte, setting_module.Pages.MaxTarGzSize+pagesUploadFormOverhead+1))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	called := false
	handler := PagesUploadFormLimit(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodPost, "/alice/site/settings/pages/versions/upload", bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.ContentLength = -1
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.False(t, called)
}
