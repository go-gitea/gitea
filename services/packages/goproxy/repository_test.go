// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestSplitLocalModulePath(t *testing.T) {
	oldAppURL, oldAppSubURL := setting.AppURL, setting.AppSubURL
	defer func() {
		setting.AppURL, setting.AppSubURL = oldAppURL, oldAppSubURL
	}()

	setting.AppURL = "https://gitea.example.com/"
	setting.AppSubURL = ""

	tests := []struct {
		modulePath string
		owner      string
		repo       string
		subdir     string
		local      bool
	}{
		{"gitea.example.com/user/repo", "user", "repo", "", true},
		{"gitea.example.com/user/repo/sub/pkg", "user", "repo", "sub/pkg", true},
		{"GITEA.EXAMPLE.COM/user/repo", "user", "repo", "", true},
		{"github.com/user/repo", "", "", "", false},
		{"gitea.example.com/user", "", "", "", false},
	}

	for _, test := range tests {
		owner, repo, subdir, local := splitLocalModulePath(test.modulePath)
		assert.Equal(t, test.local, local, test.modulePath)
		assert.Equal(t, test.owner, owner, test.modulePath)
		assert.Equal(t, test.repo, repo, test.modulePath)
		assert.Equal(t, test.subdir, subdir, test.modulePath)
	}
}

func TestSplitLocalModulePathWithSubURL(t *testing.T) {
	oldAppURL, oldAppSubURL := setting.AppURL, setting.AppSubURL
	defer func() {
		setting.AppURL, setting.AppSubURL = oldAppURL, oldAppSubURL
	}()

	setting.AppURL = "https://gitea.example.com/gitea/"
	setting.AppSubURL = "/gitea"

	owner, repo, subdir, local := splitLocalModulePath("gitea.example.com/gitea/user/repo/pkg")
	assert.True(t, local)
	assert.Equal(t, "user", owner)
	assert.Equal(t, "repo", repo)
	assert.Equal(t, "pkg", subdir)

	_, _, _, local = splitLocalModulePath("gitea.example.com/user/repo")
	assert.False(t, local)
}

func TestCanonicalVersion(t *testing.T) {
	tests := []struct {
		tag      string
		version  string
		expected bool
	}{
		{"v1.2.3", "v1.2.3", true},
		{"1.2.3", "v1.2.3", true},
		{"v2.0.0-rc1", "v2.0.0-rc1", true},
		{"release", "", false},
		{"v1.2", "v1.2.0", true},
	}

	for _, test := range tests {
		version, ok := canonicalVersion(test.tag)
		assert.Equal(t, test.expected, ok, test.tag)
		assert.Equal(t, test.version, version, test.tag)
	}
}
