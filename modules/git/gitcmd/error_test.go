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
}
