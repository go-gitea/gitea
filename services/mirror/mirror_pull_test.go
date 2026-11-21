// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

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

func TestPullMirrorUpdatedTime(t *testing.T) {
	const syncTime timeutil.TimeStamp = 1_800_000_000 // Unix seconds.
	past, future := syncTime.Add(-60), syncTime.Add(60)
	m := &repo_model.Mirror{Repo: &repo_model.Repository{UpdatedUnix: past}, LastSyncUnix: syncTime}

	assert.Equal(t, past.AsTime(), pullMirrorUpdatedTime(m, past.AsTime(), true))
	assert.Equal(t, syncTime.AsTime(), pullMirrorUpdatedTime(m, future.AsTime(), true))
	assert.Equal(t, past.AsTime(), pullMirrorUpdatedTime(m, future.AsTime(), false))

	m.Repo.UpdatedUnix = future
	assert.Equal(t, syncTime.AsTime(), pullMirrorUpdatedTime(m, future.AsTime(), false))
}
