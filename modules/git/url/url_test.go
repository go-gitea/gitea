// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package url

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseURL(t *testing.T) {
	kases := []struct {
		kase     string
		expected *URL
	}{
		{
			kase: "git@github.com:go-gitea/gitea.git",
			expected: &URL{
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
			kase: "/repositories/go-gitea/gitea.git",
			expected: &URL{
				URL: &url.URL{
					Scheme: "file",
					Path:   "/repositories/go-gitea/gitea.git",
				},
				extraMark: 2,
			},
		},
		{
			kase: "https://github.com/go-gitea/gitea.git",
			expected: &URL{
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
			expected: &URL{
				URL: &url.URL{
					Scheme: "https",
					Host:   "github.com",
					User:   url.UserPassword("git", "git"),
					Path:   "/go-gitea/gitea.git",
				},
				extraMark: 0,
			},
		},
	}

	for _, kase := range kases {
		u, err := Parse(kase.kase)
		assert.NoError(t, err)
		assert.EqualValues(t, kase.expected.extraMark, u.extraMark)
		assert.EqualValues(t, *kase.expected, *u)
	}
}
