// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package url

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestParseGitURLs(t *testing.T) {
	kases := []struct {
		kase     string
		expected *GitURL
	}{
		{
			kase: "git@127.0.0.1:go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "ssh",
					User:   url.User("git"),
					Host:   "127.0.0.1",
					Path:   "go-gitea/gitea.git",
				},
				extraMark: 1,
			},
		},
		{
			kase: "git@[fe80:14fc:cec5:c174:d88%2510]:go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "ssh",
					User:   url.User("git"),
					Host:   "[fe80:14fc:cec5:c174:d88%10]",
					Path:   "go-gitea/gitea.git",
				},
				extraMark: 1,
			},
		},
		{
			kase: "git@[::1]:go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "ssh",
					User:   url.User("git"),
					Host:   "[::1]",
					Path:   "go-gitea/gitea.git",
				},
				extraMark: 1,
			},
		},
		{
			kase: "git@github.com:go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "ssh",
					User:   url.User("git"),
					Host:   "github.com",
					Path:   "go-gitea/gitea.git",
				},
				extraMark: 1,
			},
		},
		{
			kase: "ssh://git@github.com/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "ssh",
					User:   url.User("git"),
					Host:   "github.com",
					Path:   "/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},
		{
			kase: "ssh://git@[::1]/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "ssh",
					User:   url.User("git"),
					Host:   "[::1]",
					Path:   "/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},
		{
			kase: "/repositories/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "file",
					Path:   "/repositories/go-gitea/gitea.git",
				},
				extraMark: 2,
			},
		},
		{
			kase: "file:///repositories/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "file",
					Path:   "/repositories/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},
		{
			kase: "https://github.com/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "https",
					Host:   "github.com",
					Path:   "/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},
		{
			kase: "https://git:git@github.com/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "https",
					Host:   "github.com",
					User:   url.UserPassword("git", "git"),
					Path:   "/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},
		{
			kase: "https://[fe80:14fc:cec5:c174:d88%2510]:20/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "https",
					Host:   "[fe80:14fc:cec5:c174:d88%10]:20",
					Path:   "/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},

		{
			kase: "git://github.com/go-gitea/gitea.git",
			expected: &GitURL{
				URL: &url.URL{
					Scheme: "git",
					Host:   "github.com",
					Path:   "/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},
	}

	for _, kase := range kases {
		t.Run(kase.kase, func(t *testing.T) {
			u, err := ParseGitURL(kase.kase)
			assert.NoError(t, err)
			assert.EqualValues(t, kase.expected.extraMark, u.extraMark)
			assert.EqualValues(t, *kase.expected, *u)
		})
	}
}

func TestParseRepositoryURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://localhost:3000")()
	defer test.MockVariableValue(&setting.SSH.Domain, "try.gitea.io")()

	ctxURL, _ := url.Parse("https://gitea")
	ctxReq := &http.Request{URL: ctxURL, Header: http.Header{}}
	ctxReq.Host = ctxURL.Host
	ctxReq.Header.Add("X-Forwarded-Proto", ctxURL.Scheme)
	ctx := context.WithValue(context.Background(), httplib.RequestContextKey, ctxReq)
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

	u, err := ParseRepositoryURL(context.Background(), "https://localhost:3000/subpath/user/repo.git")
	assert.NoError(t, err)
	assert.Equal(t, "/subpath/user/repo", MakeRepositoryWebLink(u))

	u, err = ParseRepositoryURL(context.Background(), "https://github.com/owner/repo.git")
	assert.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo", MakeRepositoryWebLink(u))

	u, err = ParseRepositoryURL(context.Background(), "git@github.com:owner/repo.git")
	assert.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo", MakeRepositoryWebLink(u))

	u, err = ParseRepositoryURL(context.Background(), "git+ssh://other:123/owner/repo.git")
	assert.NoError(t, err)
	assert.Equal(t, "https://other/owner/repo", MakeRepositoryWebLink(u))
}
