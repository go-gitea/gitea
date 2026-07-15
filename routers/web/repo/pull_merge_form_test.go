// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinSquashMergeMessage(t *testing.T) {
	const mergeBody = "Reviewed-on: https://gitea.com/gitea/runner/pulls/1084\nReviewed-by: Zettat123 <39446+zettat123@noreply.gitea.com>"

	// commit messages without a trailing newline must not be concatenated onto the trailers
	assert.Equal(t,
		"Fixes #496\nFixes #552\n"+mergeBody,
		joinSquashMergeMessage("Fixes #496\nFixes #552", mergeBody),
	)

	// empty commit messages: only the merge body, no leading newline
	assert.Equal(t, mergeBody, joinSquashMergeMessage("", mergeBody))

	// empty merge body: only the commit messages, no trailing newline
	assert.Equal(t, "Fixes #496", joinSquashMergeMessage("Fixes #496", ""))

	// both empty
	assert.Empty(t, joinSquashMergeMessage("", ""))
}
