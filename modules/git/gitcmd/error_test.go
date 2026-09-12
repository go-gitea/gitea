// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStderr(t *testing.T) {
	t.Run("StderrWildcard", func(t *testing.T) {
		cases := []struct {
			check  StderrWildcard
			stderr string
		}{
			{StderrUnknownRevisionOrPath, "fatal: ambiguous argument 'origin': unknown revision or path not in the working tree...."},
			{StderrNoMergeBase, "fatal: origin/main..HEAD: no merge base...."},
		}
		for _, tc := range cases {
			assert.True(t, IsStderr(&runStdError{stderr: tc.stderr}, tc.check), "stderr: %s", tc.stderr)
		}
	})
	t.Run("StderrPrefix", func(t *testing.T) {
		authStderr := `Cloning into 'repo'...
remote: Invalid username or token.
fatal: Authentication failed for 'https://host/repo.git/'
`
		assert.True(t, IsStderr(&runStdError{stderr: authStderr}, StderrAuthenticationFailed))
		assert.False(t, IsStderr(&runStdError{stderr: authStderr}, StderrCouldNotReadUsername))
	})
}
