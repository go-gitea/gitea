// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestGitSmartHTTP(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		testGitSmartHTTP(t, u)
		testRenamedRepoRedirect(t)
	})
}

func testGitSmartHTTP(t *testing.T, u *url.URL) {
	kases := []struct {
		p    string
		code int
	}{
		{
			p:    "user2/repo1/info/refs",
			code: http.StatusOK,
		},
		{
			p:    "user2/repo1/HEAD",
			code: http.StatusOK,
		},
		{
			p:    "user2/repo1/objects/info/alternates",
			code: http.StatusNotFound,
		},
		{
			p:    "user2/repo1/objects/info/http-alternates",
			code: http.StatusNotFound,
		},
		{
			p:    "user2/repo1/../../custom/conf/app.ini",
			code: http.StatusNotFound,
		},
		{
			p:    "user2/repo1/objects/info/../../../../custom/conf/app.ini",
			code: http.StatusNotFound,
		},
		{
			p:    `user2/repo1/objects/info/..\..\..\..\custom\conf\app.ini`,
			code: http.StatusBadRequest,
		},
	}

	for _, kase := range kases {
		t.Run(kase.p, func(t *testing.T) {
			p := u.String() + kase.p
			req, err := http.NewRequest("GET", p, nil)
			assert.NoError(t, err)
			req.SetBasicAuth("user2", userPassword)
			resp, err := http.DefaultClient.Do(req)
			assert.NoError(t, err)
			defer resp.Body.Close()
			assert.EqualValues(t, kase.code, resp.StatusCode)
			_, err = io.ReadAll(resp.Body)
			assert.NoError(t, err)
		})
	}
}

func testRenamedRepoRedirect(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()

	// git client requires to get a 301 redirect response before 401 unauthorized response
	req := NewRequest(t, "GET", "/user2/oldrepo1/info/refs")
	resp := MakeRequest(t, req, http.StatusMovedPermanently)
	redirect := resp.Header().Get("Location")
	assert.Equal(t, "/user2/repo1/info/refs", redirect)

	req = NewRequest(t, "GET", redirect)
	resp = MakeRequest(t, req, http.StatusUnauthorized)
	assert.Equal(t, "Unauthorized\n", resp.Body.String())

	req = NewRequest(t, "GET", redirect).AddBasicAuth("user2")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "65f1bf27bc3bf70f64657658635e66094edbcb4d\trefs/tags/v1.1")
}
