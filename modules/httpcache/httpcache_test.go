// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httpcache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestHandleGenericETagCache(t *testing.T) {
	matchedEtag := `"matched-etag"`
	lastModifiedTime := new(time.Date(2021, time.January, 2, 15, 4, 5, 0, time.FixedZone("test-zone", 8*3600)))
	lastModified := lastModifiedTime.UTC().Format(http.TimeFormat)
	cacheControl := "max-age=0, private, must-revalidate, no-transform"
	type testCase struct {
		name        string
		reqHeaders  map[string]string
		wantHandled bool
		wantHeaders map[string]string
		wantStatus  int
	}
	cases := []testCase{
		{
			name:        "No If-None-Match",
			wantHandled: false,
			wantHeaders: map[string]string{"Last-Modified": lastModified, "Cache-Control": cacheControl, "Etag": matchedEtag},
		},
		{
			name:        "Mismatched If-None-Match",
			reqHeaders:  map[string]string{"If-None-Match": `"mismatched-etag"`},
			wantHandled: false,
			wantHeaders: map[string]string{"Last-Modified": lastModified, "Cache-Control": cacheControl, "Etag": matchedEtag},
		},
		{
			name:        "Matched If-None-Match",
			reqHeaders:  map[string]string{"If-None-Match": matchedEtag},
			wantHandled: true,
			wantHeaders: map[string]string{"Last-Modified": lastModified, "Cache-Control": "", "Etag": matchedEtag},
			wantStatus:  http.StatusNotModified,
		},
		{
			name:        "Multiple Mismatched If-None-Match",
			reqHeaders:  map[string]string{"If-None-Match": `"mismatched-etag1", "mismatched-etag2"`},
			wantHandled: false,
			wantHeaders: map[string]string{"Last-Modified": lastModified, "Cache-Control": cacheControl, "Etag": matchedEtag},
		},
		{
			name:        "Multiple Matched If-None-Match",
			reqHeaders:  map[string]string{"If-None-Match": `"mismatched-etag", ` + matchedEtag},
			wantHandled: true,
			wantHeaders: map[string]string{"Last-Modified": lastModified, "Cache-Control": "", "Etag": matchedEtag},
			wantStatus:  http.StatusNotModified,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
			for k, v := range tc.reqHeaders {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			assert.Equal(t, tc.wantHandled, HandleGenericETagPrivateCache(req, w, matchedEtag, lastModifiedTime))
			resp := w.Result()
			for k, v := range tc.wantHeaders {
				assert.Equal(t, v, resp.Header.Get(k))
			}
			assert.Equal(t, tc.wantStatus, util.Iif(resp.StatusCode == http.StatusOK, 0, resp.StatusCode))
		})
	}
}
