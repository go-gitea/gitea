// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRefEndName(t *testing.T) {
	// Test branch names (with and without slash).
	assert.Equal(t, "foo", RefEndName("refs/heads/foo"))
	assert.Equal(t, "feature/foo", RefEndName("refs/heads/feature/foo"))

	// Test tag names (with and without slash).
	assert.Equal(t, "foo", RefEndName("refs/tags/foo"))
	assert.Equal(t, "release/foo", RefEndName("refs/tags/release/foo"))

	// Test commit hashes.
	assert.Equal(t, "c0ffee", RefEndName("c0ffee"))
}

func TestRefURL(t *testing.T) {
	repoURL := "/user/repo"
	assert.Equal(t, repoURL+"/src/branch/foo", RefURL(repoURL, "refs/heads/foo"))
	assert.Equal(t, repoURL+"/src/tag/foo", RefURL(repoURL, "refs/tags/foo"))
	assert.Equal(t, repoURL+"/src/commit/c0ffee", RefURL(repoURL, "c0ffee"))
}
