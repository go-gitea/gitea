// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func Test_isGitRawOrLFSPath(t *testing.T) {
	tests := []struct {
		path string

		want bool
	}{
		{
			"/owner/repo/git-upload-pack",
			true,
		},
		{
			"/owner/repo/git-receive-pack",
			true,
		},
		{
			"/owner/repo/info/refs",
			true,
		},
		{
			"/owner/repo/HEAD",
			true,
		},
		{
			"/owner/repo/objects/info/alternates",
			true,
		},
		{
			"/owner/repo/objects/info/http-alternates",
			true,
		},
		{
			"/owner/repo/objects/info/packs",
			true,
		},
		{
			"/owner/repo/objects/info/blahahsdhsdkla",
			true,
		},
		{
			"/owner/repo/objects/01/23456789abcdef0123456789abcdef01234567",
			true,
		},
		{
			"/owner/repo/objects/pack/pack-123456789012345678921234567893124567894.pack",
			true,
		},
		{
			"/owner/repo/objects/pack/pack-0123456789abcdef0123456789abcdef0123456.idx",
			true,
		},
		{
			"/owner/repo/raw/branch/foo/fanaso",
			true,
		},
		{
			"/owner/repo/stars",
			false,
		},
		{
			"/notowner",
			false,
		},
		{
			"/owner/repo",
			false,
		},
		{
			"/owner/repo/commit/123456789012345678921234567893124567894",
			false,
		},
		{
			"/owner/repo/releases/download/tag/repo.tar.gz",
			true,
		},
		{
			"/owner/repo/attachments/6d92a9ee-5d8b-4993-97c9-6181bdaa8955",
			true,
		},
	}

	defer test.MockVariableValue(&setting.LFS.StartServer)()
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "http://localhost"+tt.path, nil)
			setting.LFS.StartServer = false
			assert.Equal(t, tt.want, newAuthPathDetector(req).isGitRawOrAttachOrLFSPath())

			setting.LFS.StartServer = true
			assert.Equal(t, tt.want, newAuthPathDetector(req).isGitRawOrAttachOrLFSPath())
		})
	}

	lfsTests := []string{
		"/owner/repo/info/lfs/",
		"/owner/repo/info/lfs/objects/batch",
		"/owner/repo/info/lfs/objects/oid/filename",
		"/owner/repo/info/lfs/objects/oid",
		"/owner/repo/info/lfs/objects",
		"/owner/repo/info/lfs/verify",
		"/owner/repo/info/lfs/locks",
		"/owner/repo/info/lfs/locks/verify",
		"/owner/repo/info/lfs/locks/123/unlock",
	}
	for _, tt := range lfsTests {
		t.Run(tt, func(t *testing.T) {
			req, _ := http.NewRequest("POST", tt, nil)
			setting.LFS.StartServer = false
			got := newAuthPathDetector(req).isGitRawOrAttachOrLFSPath()
			assert.Equalf(t, setting.LFS.StartServer, got, "isGitOrLFSPath(%q) = %v, want %v, %v", tt, got, setting.LFS.StartServer, globalVars().gitRawOrAttachPathRe.MatchString(tt))

			setting.LFS.StartServer = true
			got = newAuthPathDetector(req).isGitRawOrAttachOrLFSPath()
			assert.Equalf(t, setting.LFS.StartServer, got, "isGitOrLFSPath(%q) = %v, want %v", tt, got, setting.LFS.StartServer)
		})
	}
}

func Test_isFeedRequest(t *testing.T) {
	tests := []struct {
		want bool
		path string
	}{
		{true, "/user.rss"},
		{true, "/user/repo.atom"},
		{false, "/user/repo"},
		{false, "/use/repo/file.rss"},

		{true, "/org/repo/rss/branch/xxx"},
		{true, "/org/repo/atom/tag/xxx"},
		{false, "/org/repo/branch/main/rss/any"},
		{false, "/org/atom/any"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://localhost"+tt.path, nil)
			assert.Equal(t, tt.want, newAuthPathDetector(req).isFeedRequest(req))
		})
	}
}
