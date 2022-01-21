// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitSmartHTTP(t *testing.T) {
	onGiteaRun(t, testGitSmartHTTP)
}

func testGitSmartHTTP(t *testing.T, u *url.URL) {
	kases := []struct {
		p    string
		code int
	}{
		{
			p:    "user2/repo1/info/refs",
			code: 200,
		},
		{
			p:    "user2/repo1/HEAD",
			code: 200,
		},
		{
			p:    "user2/repo1/objects/info/alternates",
			code: 404,
		},
		{
			p:    "user2/repo1/objects/info/http-alternates",
			code: 404,
		},
		{
			p:    "user2/repo1/../../custom/conf/app.ini",
			code: 404,
		},
		{
			p:    "user2/repo1/objects/info/../../../../custom/conf/app.ini",
			code: 404,
		},
		{
			p:    `user2/repo1/objects/info/..\..\..\..\custom\conf\app.ini`,
			code: 400,
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
