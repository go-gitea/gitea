// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseRemoteUpdateOutput(t *testing.T) {
	output := `
 * [new tag]         v0.1.8     -> v0.1.8
 * [new branch]      master     -> origin/master
 - [deleted]         (none)     -> origin/test1
 - [deleted]         (none)     -> tag1
 + f895a1e...957a993 test2      -> origin/test2  (forced update)
   957a993..a87ba5f  test3      -> origin/test3
 * [new ref]               refs/pull/26595/head  -> refs/pull/26595/head
 * [new ref]               refs/pull/26595/merge -> refs/pull/26595/merge
   e0639e38fb..6db2410489  refs/pull/25873/head  -> refs/pull/25873/head
 + 1c97ebc746...976d27d52f refs/pull/25873/merge -> refs/pull/25873/merge  (forced update)
`
	results := parseRemoteUpdateOutput(output, "origin")
	assert.Len(t, results, 10)
	assert.EqualValues(t, "refs/tags/v0.1.8", results[0].refName.String())
	assert.EqualValues(t, gitShortEmptySha, results[0].oldCommitID)
	assert.EqualValues(t, "", results[0].newCommitID)

	assert.EqualValues(t, "refs/heads/master", results[1].refName.String())
	assert.EqualValues(t, gitShortEmptySha, results[1].oldCommitID)
	assert.EqualValues(t, "", results[1].newCommitID)

	assert.EqualValues(t, "refs/heads/test1", results[2].refName.String())
	assert.EqualValues(t, "", results[2].oldCommitID)
	assert.EqualValues(t, gitShortEmptySha, results[2].newCommitID)

	assert.EqualValues(t, "refs/tags/tag1", results[3].refName.String())
	assert.EqualValues(t, "", results[3].oldCommitID)
	assert.EqualValues(t, gitShortEmptySha, results[3].newCommitID)

	assert.EqualValues(t, "refs/heads/test2", results[4].refName.String())
	assert.EqualValues(t, "f895a1e", results[4].oldCommitID)
	assert.EqualValues(t, "957a993", results[4].newCommitID)

	assert.EqualValues(t, "refs/heads/test3", results[5].refName.String())
	assert.EqualValues(t, "957a993", results[5].oldCommitID)
	assert.EqualValues(t, "a87ba5f", results[5].newCommitID)

	assert.EqualValues(t, "refs/pull/26595/head", results[6].refName.String())
	assert.EqualValues(t, gitShortEmptySha, results[6].oldCommitID)
	assert.EqualValues(t, "", results[6].newCommitID)

	assert.EqualValues(t, "refs/pull/26595/merge", results[7].refName.String())
	assert.EqualValues(t, gitShortEmptySha, results[7].oldCommitID)
	assert.EqualValues(t, "", results[7].newCommitID)

	assert.EqualValues(t, "refs/pull/25873/head", results[8].refName.String())
	assert.EqualValues(t, "e0639e38fb", results[8].oldCommitID)
	assert.EqualValues(t, "6db2410489", results[8].newCommitID)

	assert.EqualValues(t, "refs/pull/25873/merge", results[9].refName.String())
	assert.EqualValues(t, "1c97ebc746", results[9].oldCommitID)
	assert.EqualValues(t, "976d27d52f", results[9].newCommitID)
}
