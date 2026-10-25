// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"gitea.dev/modules/typesniffer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeUserContent(t *testing.T) {
	data := "0123456789abcdef"
	for _, source := range []string{"reader", "file"} {
		t.Run(source, func(t *testing.T) {
			var tmpFile string
			if source == "file" {
				tmpFile = t.TempDir() + "/test"
				assert.NoError(t, os.WriteFile(tmpFile, []byte(data), 0o644))
			}
			for _, tc := range []struct {
				rangeStr           string
				expectedStatusCode int
				expectedContent    string
			}{
				{"", http.StatusOK, data},
				{"0-", http.StatusPartialContent, data},
				{"0-15", http.StatusPartialContent, data},
				{"1-", http.StatusPartialContent, data[1:]},
				{"1-3", http.StatusPartialContent, data[1 : 3+1]},
				{"16-", http.StatusRequestedRangeNotSatisfiable, ""},
				{"1-99999", http.StatusPartialContent, data[1:]},
			} {
				t.Run("_range_"+tc.rangeStr, func(t *testing.T) {
					r := &http.Request{Header: http.Header{}, Form: url.Values{}}
					if tc.rangeStr != "" {
						r.Header.Set("Range", "bytes="+tc.rangeStr)
					}
					w := httptest.NewRecorder()
					if source == "file" {
						seekReader, err := os.OpenFile(tmpFile, os.O_RDONLY, 0o644)
						require.NoError(t, err)
						defer seekReader.Close()
						ServeUserContentByFile(r, w, seekReader, ServeHeaderOptions{})
					} else {
						ServeUserContentByReader(r, w, int64(len(data)), strings.NewReader(data), ServeHeaderOptions{})
					}
					assert.Equal(t, tc.expectedStatusCode, w.Code)
					if tc.expectedStatusCode == http.StatusPartialContent || tc.expectedStatusCode == http.StatusOK {
						assert.Equal(t, strconv.Itoa(len(tc.expectedContent)), w.Header().Get("Content-Length"))
						assert.Equal(t, tc.expectedContent, w.Body.String())
					}
				})
			}
		})
	}
}

func TestServeSetHeaderContentRelated(t *testing.T) {
	cases := []struct {
		contentType string
		csp         string
	}{
		{"", serveHeaderCspDefault},
		{"any", serveHeaderCspDefault},
		{"application/pdf", serveHeaderCspPdf},
		{"application/pdf; charset=binary", serveHeaderCspPdf},
		{"audio/mp4", serveHeaderCspAudioVideo},
		{"video/ogg; codecs=theora", serveHeaderCspAudioVideo},
		{"video/mp4;,text/html", serveHeaderCspDefault},
		{typesniffer.MimeTypeImageSvg, serveHeaderCspDefault},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		serveSetHeaderContentRelated(w, c.contentType)
		csp := w.Header().Get("Content-Security-Policy")
		assert.Equal(t, c.csp, csp, "content-type: %s", c.contentType)
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options")) // it should always be there
	}

	// make sure sandboxed
	require.Contains(t, serveHeaderCspDefault, "sandbox")
	require.NotContains(t, serveHeaderCspDefault, "allow-same-origin")
}

func TestServeSetHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	ServeSetHeaders(w, ServeHeaderOptions{Filename: "foo.zip"})
	assert.Equal(t, "attachment; filename=foo.zip", w.Header().Get("Content-Disposition"))
	ServeSetHeaders(w, ServeHeaderOptions{Filename: "foo.zip", ContentDisposition: ContentDispositionInline})
	assert.Equal(t, "inline; filename=foo.zip", w.Header().Get("Content-Disposition"))
}
