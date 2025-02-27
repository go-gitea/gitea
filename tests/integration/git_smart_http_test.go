// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitSmartHTTP(t *testing.T) {
	onGiteaRun(t, testGitSmartHTTP)
}

func testGitSmartHTTP(t *testing.T, u *url.URL) {
	kases := []struct {
		method, path string
		code         int
	}{
		{
			path: "user2/repo1/info/refs",
			code: http.StatusOK,
		},
		{
			method: "HEAD",
			path:   "user2/repo1/info/refs",
			code:   http.StatusOK,
		},
		{
			path: "user2/repo1/HEAD",
			code: http.StatusOK,
		},
		{
			path: "user2/repo1/objects/info/alternates",
			code: http.StatusNotFound,
		},
		{
			path: "user2/repo1/objects/info/http-alternates",
			code: http.StatusNotFound,
		},
		{
			path: "user2/repo1/../../custom/conf/app.ini",
			code: http.StatusNotFound,
		},
		{
			path: "user2/repo1/objects/info/../../../../custom/conf/app.ini",
			code: http.StatusNotFound,
		},
		{
			path: `user2/repo1/objects/info/..\..\..\..\custom\conf\app.ini`,
			code: http.StatusBadRequest,
		},
	}

	for _, kase := range kases {
		t.Run(kase.path, func(t *testing.T) {
			req, err := http.NewRequest(util.IfZero(kase.method, "GET"), u.String()+kase.path, nil)
			require.NoError(t, err)
			req.SetBasicAuth("user2", userPassword)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.EqualValues(t, kase.code, resp.StatusCode)
			_, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
		})
	}
}
