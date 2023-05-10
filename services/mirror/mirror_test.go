// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"

	"code.gitea.io/gitea/modules/git"
	"github.com/stretchr/testify/assert"
)

func Test_parseRemoteUpdateOutput(t *testing.T) {
	output := `
 * [new tag]         v0.1.8     -> v0.1.8
 * [new branch]      master     -> origin/master
 - [deleted]         (none)     -> origin/test
 + f895a1e...957a993 test       -> origin/test  (forced update)
`
	results := parseRemoteUpdateOutput(output, "origin")
	assert.Len(t, results, 4)
	assert.EqualValues(t, "refs/tags/v0.1.8", results[0].refName.String())
	assert.EqualValues(t, git.EmptySHA, results[0].oldCommitID)

	assert.EqualValues(t, "refs/heads/master", results[1].refName.String())
	assert.EqualValues(t, git.EmptySHA, results[1].oldCommitID)

	assert.EqualValues(t, "refs/heads/test", results[2].refName.String())
	assert.EqualValues(t, git.EmptySHA, results[2].newCommitID)

	assert.EqualValues(t, "refs/heads/test", results[3].refName.String())
	assert.EqualValues(t, "f895a1e", results[3].oldCommitID)
	assert.EqualValues(t, "957a993", results[3].newCommitID)
}
