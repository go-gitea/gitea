// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
	"context"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestToInternalLFSURL(t *testing.T) {
	defer test.MockVariableValue(&setting.LocalURL, "http://localurl/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	cases := []struct {
		url      string
		expected string
	}{
		{"http://appurl/any", ""},
		{"http://appurl/sub/any", ""},
		{"http://appurl/sub/owner/repo/any", ""},
		{"http://appurl/sub/owner/repo/info/any", ""},
		{"http://appurl/sub/owner/repo/info/lfs/any", "http://localurl/api/internal/repo/owner/repo/info/lfs/any"},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, toInternalLFSURL(c.url), c.url)
	}
}

func TestIsInternalLFSURL(t *testing.T) {
	defer test.MockVariableValue(&setting.LocalURL, "http://localurl/")()
	defer test.MockVariableValue(&setting.InternalToken, "mock-token")()
	cases := []struct {
		url      string
		expected bool
	}{
		{"", false},
		{"http://otherurl/api/internal/repo/owner/repo/info/lfs/any", false},
		{"http://localurl/api/internal/repo/owner/repo/info/lfs/any", true},
		{"http://localurl/api/internal/repo/owner/repo/info", false},
		{"http://localurl/api/internal/misc/owner/repo/info/lfs/any", false},
		{"http://localurl/api/internal/owner/repo/info/lfs/any", false},
		{"http://localurl/api/internal/foo/bar", false},
	}
	for _, c := range cases {
		req := newInternalRequestLFS(context.Background(), c.url, "GET", nil, nil)
		assert.Equal(t, c.expected, req != nil, c.url)
		assert.Equal(t, c.expected, isInternalLFSURL(c.url), c.url)
	}
}
