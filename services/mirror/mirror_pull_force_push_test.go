// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"

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

func Test_isBackupRefForPrefix(t *testing.T) {
	prefix := "refs/heads/mirror-backup/branch/main-"

	cases := []struct {
		name     string
		refName  string
		expected bool
	}{
		{
			name:     "matching backup ref",
			refName:  "refs/heads/mirror-backup/branch/main-20260604-143012",
			expected: true,
		},
		{
			name:     "matching backup ref with suffix",
			refName:  "refs/heads/mirror-backup/branch/main-20260604-143012-2",
			expected: true,
		},
		{
			name:     "branch name prefix collision",
			refName:  "refs/heads/mirror-backup/branch/main-feature-20260604-143012",
			expected: false,
		},
		{
			name:     "invalid timestamp",
			refName:  "refs/heads/mirror-backup/branch/main-feature-20260604",
			expected: false,
		},
		{
			name:     "invalid numeric suffix",
			refName:  "refs/heads/mirror-backup/branch/main-20260604-143012-next",
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, isBackupRefForPrefix(c.refName, prefix))
		})
	}
}
