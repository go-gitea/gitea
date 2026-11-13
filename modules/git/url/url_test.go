// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package url

import (
	"net/http"
	"net/url"
	"testing"

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestParseGitURLs(t *testing.T) {
	for _, kase := range []struct {
		kase, scheme, host, path string
		user                     *url.Userinfo
		extraMark                int
	}{
		{"git@127.0.0.1:go-gitea/gitea.git", "ssh", "127.0.0.1", "go-gitea/gitea.git", url.User("git"), 1},
		{"git@[fe80::14fc:cec5:c174:d88%2510]:go-gitea/gitea.git", "ssh", "[fe80::14fc:cec5:c174:d88%10]", "go-gitea/gitea.git", url.User("git"), 1},
		{"git@[::1]:go-gitea/gitea.git", "ssh", "[::1]", "go-gitea/gitea.git", url.User("git"), 1},
		{"git@github.com:go-gitea/gitea.git", "ssh", "github.com", "go-gitea/gitea.git", url.User("git"), 1},
		{"ssh://git@github.com/go-gitea/gitea.git", "ssh", "github.com", "/go-gitea/gitea.git", url.User("git"), 0},
		{"ssh://git@[::1]/go-gitea/gitea.git", "ssh", "[::1]", "/go-gitea/gitea.git", url.User("git"), 0},
		{"/repositories/go-gitea/gitea.git", "file", "", "/repositories/go-gitea/gitea.git", nil, 2},
		{"file:///repositories/go-gitea/gitea.git", "file", "", "/repositories/go-gitea/gitea.git", nil, 0},
		{"https://github.com/go-gitea/gitea.git", "https", "github.com", "/go-gitea/gitea.git", nil, 0},
		{"https://git:git@github.com/go-gitea/gitea.git", "https", "github.com", "/go-gitea/gitea.git", url.UserPassword("git", "git"), 0},
		{"https://[fe80::14fc:cec5:c174:d88%2510]:20/go-gitea/gitea.git", "https", "[fe80::14fc:cec5:c174:d88%10]:20", "/go-gitea/gitea.git", nil, 0},
		{"git://github.com/go-gitea/gitea.git", "git", "github.com", "/go-gitea/gitea.git", nil, 0},
	} {
		t.Run(kase.kase, func(t *testing.T) {
			u, err := ParseGitURL(kase.kase)
			assert.NoError(t, err)
			assert.Equal(t, kase.extraMark, u.extraMark)
			assert.Equal(t, GitURL{
				URL: &url.URL{
					Scheme: kase.scheme,
					User:   kase.user,
					Host:   kase.host,
					Path:   kase.path,
				},
				extraMark: kase.extraMark,
			}, *u)
		})
	}
}

func TestParseRepositoryURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://localhost:3000")()
	defer test.MockVariableValue(&setting.SSH.Domain, "try.gitea.io")()

	ctx := reqctx.NewRequestContextForTest(t)
	ctxURL, _ := url.Parse("https://gitea")
	ctxReq := &http.Request{URL: ctxURL, Header: http.Header{}}
	ctxReq.Host = ctxURL.Host
	ctxReq.Header.Add("X-Forwarded-Proto", ctxURL.Scheme)
	httplib.RequestWithContext(ctxReq, ctx)
	httplib.MarkRequestSupportPublicURL(ctx)
	cases := []struct {
		input                          string
		ownerName, repoName, remaining string
	}{
		{input: "/user/repo"},

		{input: "https://localhost:3000/user/repo", ownerName: "user", repoName: "repo"},
		{input: "https://external:3000/user/repo"},

		{input: "https://localhost:3000/user/repo.git/other", ownerName: "user", repoName: "repo", remaining: "/other"},

		{input: "https://gitea/user/repo", ownerName: "user", repoName: "repo"},
		{input: "https://gitea:3333/user/repo"},

		{input: "ssh://try.gitea.io:2222/user/repo", ownerName: "user", repoName: "repo"},
		{input: "ssh://external:2222/user/repo"},

		{input: "git+ssh://user@try.gitea.io/user/repo.git", ownerName: "user", repoName: "repo"},
		{input: "git+ssh://user@external/user/repo.git"},

		{input: "root@try.gitea.io:user/repo.git", ownerName: "user", repoName: "repo"},
		{input: "root@gitea:user/repo.git", ownerName: "user", repoName: "repo"},
		{input: "root@external:user/repo.git"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			ret, _ := ParseRepositoryURL(ctx, c.input)
			assert.Equal(t, c.ownerName, ret.OwnerName)
			assert.Equal(t, c.repoName, ret.RepoName)
			assert.Equal(t, c.remaining, ret.RemainingPath)
		})
	}

	t.Run("WithSubpath", func(t *testing.T) {
		defer test.MockVariableValue(&setting.AppURL, "https://localhost:3000/subpath")()
		defer test.MockVariableValue(&setting.AppSubURL, "/subpath")()
		cases = []struct {
			input                          string
			ownerName, repoName, remaining string
		}{
			{input: "https://localhost:3000/user/repo"},
			{input: "https://localhost:3000/subpath/user/repo.git/other", ownerName: "user", repoName: "repo", remaining: "/other"},

			{input: "ssh://try.gitea.io:2222/user/repo", ownerName: "user", repoName: "repo"},
			{input: "ssh://external:2222/user/repo"},

			{input: "git+ssh://user@try.gitea.io/user/repo.git", ownerName: "user", repoName: "repo"},
			{input: "git+ssh://user@external/user/repo.git"},

			{input: "root@try.gitea.io:user/repo.git", ownerName: "user", repoName: "repo"},
			{input: "root@external:user/repo.git"},
		}

		for _, c := range cases {
			t.Run(c.input, func(t *testing.T) {
				ret, _ := ParseRepositoryURL(ctx, c.input)
				assert.Equal(t, c.ownerName, ret.OwnerName)
				assert.Equal(t, c.repoName, ret.RepoName)
				assert.Equal(t, c.remaining, ret.RemainingPath)
			})
		}
	})
}

func TestMakeRepositoryBaseLink(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://localhost:3000/subpath")()
	defer test.MockVariableValue(&setting.AppSubURL, "/subpath")()

	for _, kase := range []struct {
		input, expected string
	}{
		{"https://localhost:3000/subpath/user/repo.git", "/subpath/user/repo"},
		{"https://github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo"},
		{"git+ssh://other:123/owner/repo.git", "https://other/owner/repo"},
		{"git+ssh://[::1]/owner/repo.git", "https://[::1]/owner/repo"},
		{"git+ssh://[::1]:2222/owner/repo.git", "https://[::1]/owner/repo"},
	} {
		u, err := ParseRepositoryURL(t.Context(), kase.input)
		assert.NoError(t, err)
		assert.Equal(t, kase.expected, MakeRepositoryWebLink(u))
	}
}
