// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
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
