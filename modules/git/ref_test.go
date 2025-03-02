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
	assert.True(t, RefName("refs/pull/1/head").IsPull())
	assert.True(t, RefName("refs/pull/1/merge").IsPull())
	assert.Equal(t, "my/pull", RefName("refs/pull/my/pull/head").PullName())

	// Test for branch names
	assert.Equal(t, "main", RefName("refs/for/main").ForBranchName())
	assert.Equal(t, "my/branch", RefName("refs/for/my/branch").ForBranchName())

	// Test commit hashes.
	assert.Equal(t, "c0ffee", RefName("c0ffee").ShortName())
}

func TestRefWebLinkPath(t *testing.T) {
	assert.Equal(t, "branch/foo", RefName("refs/heads/foo").RefWebLinkPath())
	assert.Equal(t, "tag/foo", RefName("refs/tags/foo").RefWebLinkPath())
	assert.Equal(t, "commit/c0ffee", RefName("c0ffee").RefWebLinkPath())
}
