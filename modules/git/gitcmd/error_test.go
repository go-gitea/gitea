// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStderr(t *testing.T) {
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

	authStderr := "Cloning into 'repo'...\nremote: Invalid username or token.\nfatal: Authentication failed for 'https://host/repo.git/'\n"
	assert.True(t, IsStderr(&runStdError{stderr: authStderr}, StderrAuthenticationFailed))
	assert.False(t, IsStderr(&runStdError{stderr: authStderr}, StderrCouldNotReadUsername))
}
