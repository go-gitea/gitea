// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSyncRepoBranches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	_, err := db.GetEngine(t.Context()).ID(1).Update(&repo_model.Repository{ObjectFormatName: "bad-fmt"})
	assert.NoError(t, db.TruncateBeans(t.Context(), &git_model.Branch{}))
	assert.NoError(t, err)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, "bad-fmt", repo.ObjectFormatName)
	_, err = SyncRepoBranches(t.Context(), 1, 0)
	assert.NoError(t, err)
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, "sha1", repo.ObjectFormatName)
	branch, err := git_model.GetBranch(t.Context(), 1, "master")
	assert.NoError(t, err)
	assert.Equal(t, "master", branch.Name)
}

func TestIsBackupBranchName(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
	}{
		// Backup branches
		{"main-backup-forced-2026-06-03T09-05-57", true},
		{"test-backup-forced-2026-01-01T00-00-00", true},
		{"feature/auth-backup-forced-2026-12-31T23-59-59", true},
		{"main-backup-forced-2026-06-03T09-05-57-2", true},

		// Regular branches
		{"main", false},
		{"test", false},
		{"feature/auth", false},
		{"release/v1.0", false},

		// Similar but not matching patterns
		{"main-backup-deleted-2026-06-03", false}, // old pattern, no longer used
		{"main-backup-2026-06-03", false},         // missing type
		{"main-conflict-2026-06-03", false},       // old conflict pattern
		{"backup-main", false},                    // wrong format
		{"main-backup-forced", false},             // missing timestamp
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, IsBackupBranchName(c.name), "test case: %s", c.name)
		})
	}
}
