// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"errors"
	"fmt"
	"testing"

	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

// TestHandlerRequeuesTransientFailures asserts that a transient failure while
// evaluating a scheduled auto merge is reported back to the queue as
// unhandled — so the queue's retry mechanism requeues it — instead of being
// dropped, which permanently lost the scheduled merge.
func TestHandlerRequeuesTransientFailures(t *testing.T) {
	defer test.MockVariableValue(&handlePullRequestAutoMerge, func(pullID int64, sha string) error {
		if pullID == 1 {
			return errors.New("transient database failure")
		}
		return nil
	})()
	defer clearTransientFailures("1_sha1")

	assert.Equal(t, []string{"1_sha1"}, handler("1_sha1", "2_sha2"))
}

func TestHandlerDoesNotRequeueHandledItems(t *testing.T) {
	calls := 0
	defer test.MockVariableValue(&handlePullRequestAutoMerge, func(pullID int64, sha string) error {
		calls++
		return nil
	})()

	// a handled item and an unparsable item are both terminal: nothing to requeue
	assert.Empty(t, handler("1_sha1", "not-parsable"))
	assert.Equal(t, 1, calls)
}

func TestHandlerGivesUpAfterMaxTransientRetries(t *testing.T) {
	defer test.MockVariableValue(&handlePullRequestAutoMerge, func(pullID int64, sha string) error {
		return errors.New("transient database failure")
	})()
	defer clearTransientFailures("1_sha1")

	for range maxTransientRetries - 1 {
		assert.Equal(t, []string{"1_sha1"}, handler("1_sha1"))
	}
	// the attempt that reaches the bound is dropped instead of requeued,
	// so a persistently failing item cannot spin in the queue forever
	assert.Empty(t, handler("1_sha1"))
	// and a later event for the same item starts counting afresh
	assert.Equal(t, []string{"1_sha1"}, handler("1_sha1"))
}

// TestHandlerRequeuesOnRepositoryAccessFailure exercises the full evaluation
// with nothing mocked but the storage location: a scheduled auto merge whose
// git repository is momentarily inaccessible must be reported back to the
// queue for requeueing. It fails against the previous handler, which reported
// every item as handled and so permanently lost the scheduled merge.
func TestHandlerRequeuesOnRepositoryAccessFailure(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assert.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), doer, pr.ID, repo_model.MergeStyleMerge, "", false))
	defer func() {
		assert.NoError(t, pull_model.DeleteScheduledAutoMerge(t.Context(), pr.ID))
	}()

	// point the repository root at an empty directory so opening the base
	// repository fails the way it would during a storage hiccup
	defer test.MockVariableValue(&setting.RepoRootPath, t.TempDir())()

	item := fmt.Sprintf("%d_%s", pr.ID, "0123456789012345678901234567890123456789")
	defer clearTransientFailures(item)

	assert.Equal(t, []string{item}, handler(item))
}

func TestHandleAutoMergeMissingPullRequestIsTerminal(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// a deleted pull request cannot be cured by retrying: the item must be
	// reported as handled so it is not requeued
	assert.NoError(t, realHandlePullRequestAutoMerge(123456789, "0123456789012345678901234567890123456789"))
}
