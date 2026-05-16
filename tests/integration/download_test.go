// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

type downloadScopeCase struct {
	name         string
	url          string
	withScope    int
	publicOnlyOK bool
}

func TestDownloadRepoContent(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	t.Run("RawBlob", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/raw/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
	})

	t.Run("SVGUsesSecureHeaders", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo2/raw/blob/6395b68e1feebb1e4c657b4f9f6ba2676a283c0b")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", resp.Header().Get("Content-Security-Policy"))
		assert.Equal(t, "image/svg+xml", resp.Header().Get("Content-Type"))
		assert.Equal(t, "nosniff", resp.Header().Get("X-Content-Type-Options"))
	})

	t.Run("MediaBlob", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/media/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
	})

	t.Run("MediaSVGUsesSecureHeaders", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo2/media/blob/6395b68e1feebb1e4c657b4f9f6ba2676a283c0b")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", resp.Header().Get("Content-Security-Policy"))
		assert.Equal(t, "image/svg+xml", resp.Header().Get("Content-Type"))
		assert.Equal(t, "nosniff", resp.Header().Get("X-Content-Type-Options"))
	})

	t.Run("MimeTypeMap", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo2/raw/branch/master/test.xml")
		resp := session.MakeRequest(t, req, http.StatusOK)
		// although the file is a valid XML file, it is served as "text/plain" to avoid site content spamming (the same to "text/html" files)
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))

		defer tests.PrepareTestEnv(t)()
		defer test.MockVariableValue(&setting.MimeTypeMap)()
		setting.MimeTypeMap.Enabled = true

		setting.MimeTypeMap.Map[".xml"] = "text/xml"
		req = NewRequest(t, "GET", "/user2/repo2/raw/branch/master/test.xml")
		resp = session.MakeRequest(t, req, http.StatusOK)
		// respect the mime mapping, and "text/plain" protection isn't used anymore
		assert.Equal(t, "text/xml; charset=utf-8", resp.Header().Get("Content-Type"))
		assert.Equal(t, "inline; filename=test.xml", resp.Header().Get("Content-Disposition"))

		setting.MimeTypeMap.Map[".xml"] = "application/xml"
		req = NewRequest(t, "GET", "/user2/repo2/raw/branch/master/test.xml")
		resp = session.MakeRequest(t, req, http.StatusOK)
		// non-text file don't have "charset"
		assert.Equal(t, "application/xml", resp.Header().Get("Content-Type"))
	})
}

func TestDownloadRepoContentTokenScopes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ownerReadToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	miscToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadMisc)
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)

	cases := []downloadScopeCase{
		{
			name:         "PublicRawBlob",
			url:          "/user2/repo1/raw/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PublicRawBranch",
			url:          "/user2/repo1/raw/branch/master/README.md",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PublicRawTag",
			url:          "/user2/repo1/raw/tag/v1.1/README.md",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PublicRawCommit",
			url:          "/user2/repo1/raw/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d/README.md",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PublicMediaBlob",
			url:          "/user2/repo1/media/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PublicMediaBranch",
			url:          "/user2/repo1/media/branch/master/README.md",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PublicMediaTag",
			url:          "/user2/repo1/media/tag/v1.1/README.md",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PublicMediaCommit",
			url:          "/user2/repo1/media/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d/README.md",
			withScope:    http.StatusOK,
			publicOnlyOK: true,
		},
		{
			name:         "PrivateRawBranch",
			url:          "/user2/repo2/raw/branch/master/test.xml",
			withScope:    http.StatusOK,
			publicOnlyOK: false,
		},
		{
			name:         "PrivateRawBlob",
			url:          "/user2/repo2/raw/blob/6395b68e1feebb1e4c657b4f9f6ba2676a283c0b",
			withScope:    http.StatusOK,
			publicOnlyOK: false,
		},
		{
			name:         "PrivateMediaBranch",
			url:          "/user2/repo2/media/branch/master/test.xml",
			withScope:    http.StatusOK,
			publicOnlyOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			MakeRequest(t, NewRequest(t, "GET", tc.url).AddTokenAuth(miscToken), http.StatusForbidden)
			MakeRequest(t, NewRequest(t, "GET", tc.url).AddTokenAuth(ownerReadToken), tc.withScope)

			publicOnlyStatus := http.StatusForbidden
			if tc.publicOnlyOK {
				publicOnlyStatus = tc.withScope
			}
			MakeRequest(t, NewRequest(t, "GET", tc.url).AddTokenAuth(publicOnlyToken), publicOnlyStatus)
		})
	}
}
