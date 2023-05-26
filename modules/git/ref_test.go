// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRefName(t *testing.T) {
	// Test branch names (with and without slash).
	assert.Equal(t, "foo", RefName("refs/heads/foo").BranchName())
	assert.Equal(t, "feature/foo", RefName("refs/heads/feature/foo").BranchName())

	// Test tag names (with and without slash).
	assert.Equal(t, "foo", RefName("refs/tags/foo").TagName())
	assert.Equal(t, "release/foo", RefName("refs/tags/release/foo").TagName())

	// Test pull names
	assert.Equal(t, "1", RefName("refs/pull/1/head").PullName())
	assert.Equal(t, "my/pull", RefName("refs/pull/my/pull/head").PullName())

	// Test for branch names
	assert.Equal(t, "main", RefName("refs/for/main").ForBranchName())
	assert.Equal(t, "my/branch", RefName("refs/for/my/branch").ForBranchName())

	// Test commit hashes.
	assert.Equal(t, "c0ffee", RefName("c0ffee").ShortName())
}

func TestRefURL(t *testing.T) {
	repoURL := "/user/repo"
	assert.Equal(t, repoURL+"/src/branch/foo", RefURL(repoURL, "refs/heads/foo"))
	assert.Equal(t, repoURL+"/src/tag/foo", RefURL(repoURL, "refs/tags/foo"))
	assert.Equal(t, repoURL+"/src/commit/c0ffee", RefURL(repoURL, "c0ffee"))
}
