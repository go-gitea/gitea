// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"

	git_model "gitea.dev/models/git"

	"github.com/stretchr/testify/assert"
)

func Test_checkRecoverableSyncError_ForcePushMessages(t *testing.T) {
	// Test that force-push related error messages are properly handled
	cases := []struct {
		recoverable bool
		message     string
	}{
		// Force push during fetch
		{true, "error: cannot lock ref 'refs/pull/123456/merge': is at 988881adc9fc3655077dc2d4d757d480b5ea0e11 but expected 7f894307ffc9553edbd0b671cab829786866f7b2"},
		// Normal error
		{false, "fatal: Authentication failed for 'https://example.com/foo-does-not-exist/bar.git/'"},
	}

	for _, c := range cases {
		assert.Equal(t, c.recoverable, checkRecoverableSyncError(c.message), "test case: %s", c.message)
	}
}

func Test_shouldRestoreBackupBranch(t *testing.T) {
	cases := []struct {
		name     string
		branch   *git_model.Branch
		expected bool
	}{
		{
			name: "active backup branch",
			branch: &git_model.Branch{
				Name: "main-backup-forced-2026-06-03T09-05-57",
			},
			expected: true,
		},
		{
			name: "deleted backup branch",
			branch: &git_model.Branch{
				Name:      "main-backup-forced-2026-06-03T09-05-57",
				IsDeleted: true,
			},
			expected: false,
		},
		{
			name: "regular branch",
			branch: &git_model.Branch{
				Name: "main",
			},
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, shouldRestoreBackupBranch(c.branch))
		})
	}
}
