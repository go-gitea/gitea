// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRepoFilesWorkingLockKey(t *testing.T) {
	assert.Equal(t, "repo_files_working_1", getRepoFilesWorkingLockKey(1))
	assert.Equal(t, "repo_files_working_42", getRepoFilesWorkingLockKey(42))

	// different repo IDs must never collide
	assert.NotEqual(t, getRepoFilesWorkingLockKey(1), getRepoFilesWorkingLockKey(2))

	// the key must be stable/deterministic for the same repo ID
	assert.Equal(t, getRepoFilesWorkingLockKey(7), getRepoFilesWorkingLockKey(7))
}
