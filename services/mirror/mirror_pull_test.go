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
	assert.Equal(t, "refs/tags/v0.1.8", results[0].refName.String())
	assert.Equal(t, gitShortEmptySha, results[0].oldCommitID)
	assert.Empty(t, results[0].newCommitID)

	assert.Equal(t, "refs/heads/master", results[1].refName.String())
	assert.Equal(t, gitShortEmptySha, results[1].oldCommitID)
	assert.Empty(t, results[1].newCommitID)

	assert.Equal(t, "refs/heads/test1", results[2].refName.String())
	assert.Empty(t, results[2].oldCommitID)
	assert.Equal(t, gitShortEmptySha, results[2].newCommitID)

	assert.Equal(t, "refs/tags/tag1", results[3].refName.String())
	assert.Empty(t, results[3].oldCommitID)
	assert.Equal(t, gitShortEmptySha, results[3].newCommitID)

	assert.Equal(t, "refs/heads/test2", results[4].refName.String())
	assert.Equal(t, "f895a1e", results[4].oldCommitID)
	assert.Equal(t, "957a993", results[4].newCommitID)

	assert.Equal(t, "refs/heads/test3", results[5].refName.String())
	assert.Equal(t, "957a993", results[5].oldCommitID)
	assert.Equal(t, "a87ba5f", results[5].newCommitID)

	assert.Equal(t, "refs/pull/26595/head", results[6].refName.String())
	assert.Equal(t, gitShortEmptySha, results[6].oldCommitID)
	assert.Empty(t, results[6].newCommitID)

	assert.Equal(t, "refs/pull/26595/merge", results[7].refName.String())
	assert.Equal(t, gitShortEmptySha, results[7].oldCommitID)
	assert.Empty(t, results[7].newCommitID)

	assert.Equal(t, "refs/pull/25873/head", results[8].refName.String())
	assert.Equal(t, "e0639e38fb", results[8].oldCommitID)
	assert.Equal(t, "6db2410489", results[8].newCommitID)

	assert.Equal(t, "refs/pull/25873/merge", results[9].refName.String())
	assert.Equal(t, "1c97ebc746", results[9].oldCommitID)
	assert.Equal(t, "976d27d52f", results[9].newCommitID)
}

func Test_checkRecoverableSyncError(t *testing.T) {
	cases := []struct {
		recoverable bool
		message     string
	}{
		// A race condition in http git-fetch where certain refs were listed on the remote and are no longer there, would exit status 128
		{true, "fatal: remote error: upload-pack: not our ref 988881adc9fc3655077dc2d4d757d480b5ea0e11"},
		// A race condition where a local gc/prune removes a named ref during a git-fetch  would exit status 1
		{true, "cannot lock ref 'refs/pull/123456/merge': unable to resolve reference 'refs/pull/134153/merge'"},
		// A race condition in http git-fetch where named refs were listed on the remote and are no longer there
		{true, "error: cannot lock ref 'refs/remotes/origin/foo': unable to resolve reference 'refs/remotes/origin/foo': reference broken"},
		// A race condition in http git-fetch where named refs were force-pushed during the update, would exit status 128
		{true, "error: cannot lock ref 'refs/pull/123456/merge': is at 988881adc9fc3655077dc2d4d757d480b5ea0e11 but expected 7f894307ffc9553edbd0b671cab829786866f7b2"},
		// A race condition with other local git operations, such as git-maintenance, would exit status 128 (well, "Unable" the "U" is uppercase)
		{true, "fatal: Unable to create '/data/gitea-repositories/foo-org/bar-repo.git/./objects/info/commit-graphs/commit-graph-chain.lock': File exists."},
		// Missing or unauthorized credentials, would exit status 128
		{false, "fatal: Authentication failed for 'https://example.com/foo-does-not-exist/bar.git/'"},
		// A non-existent remote repository, would exit status 128
		{false, "fatal: Could not read from remote repository."},
		// A non-functioning proxy, would exit status 128
		{false, "fatal: unable to access 'https://example.com/foo-does-not-exist/bar.git/': Failed to connect to configured-https-proxy port 1080 after 0 ms: Couldn't connect to server"},
	}

	for _, c := range cases {
		assert.Equal(t, c.recoverable, checkRecoverableSyncError(c.message), "test case: %s", c.message)
	}
}
