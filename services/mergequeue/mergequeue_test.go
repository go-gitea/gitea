// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestMergeQueueCheckRerunsAndHeadGreen(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	const branch = "master"
	started := timeutil.TimeStampNow().AddDuration(-time.Hour)
	first := &pull_model.MergeQueueEntry{
		RepoID: repo.ID, BaseBranch: branch, PullID: 2, EnqueuerID: repo.OwnerID, BaseSHA: "base", GroupHeadSHA: strings.Repeat("a", 40),
		GroupRef: "refs/heads/queue-first", ChecksStartedUnix: started,
	}
	next := &pull_model.MergeQueueEntry{
		RepoID: repo.ID, BaseBranch: branch, PullID: 5, EnqueuerID: repo.OwnerID, BaseSHA: first.GroupHeadSHA, GroupHeadSHA: strings.Repeat("b", 40),
		GroupRef: "refs/heads/queue-next", ChecksStartedUnix: timeutil.TimeStampNow(),
	}
	require.NoError(t, db.Insert(ctx, first, next))
	entries, err := GetQueue(ctx, repo, branch)
	require.NoError(t, err)
	s := queueState{repo: repo, branch: branch, entries: entries, rule: &git_model.ProtectedBranch{}}
	s.rule.MergeQueue = git_model.MergeQueueConfig{GroupingStrategy: git_model.MergeQueueGroupingHeadGreen, MaxEntriesToMerge: 2, CheckResponseTimeoutMinutes: 180}

	addToQueue(repo.ID, branch)
	worker, err := queue.NewWorkerPoolQueueWithContext(t.Context(), t.Name(), setting.QueueSettings{Type: "channel", Length: 1}, handleBranches, true)
	require.NoError(t, err)
	defer worker.Cancel()
	branchQueue.Store(worker)
	defer branchQueue.Store(nil)
	new(notifier).CreateCommitStatus(ctx, repo, &repository.PushCommit{Sha1: first.GroupHeadSHA}, nil, &git_model.CommitStatus{State: commitstatus.CommitStatusPending})
	queued, err := worker.Has("1:master")
	require.NoError(t, err)
	assert.True(t, queued)

	for _, tc := range []struct {
		status commitstatus.CommitStatusState
		want   pull_model.MergeQueueEntryState
	}{
		{commitstatus.CommitStatusSuccess, pull_model.MergeQueueStateMergeable},
		{commitstatus.CommitStatusFailure, pull_model.MergeQueueStateUnmergeable},
		{commitstatus.CommitStatusSuccess, pull_model.MergeQueueStateMergeable},
		{commitstatus.CommitStatusPending, pull_model.MergeQueueStateAwaitingChecks},
	} {
		require.NoError(t, git_model.NewCommitStatus(ctx, git_model.NewCommitStatusOptions{
			Repo: repo, Creator: repo.MustOwner(ctx), SHA: git.MustIDFromString(first.GroupHeadSHA),
			CommitStatus: &git_model.CommitStatus{Context: "ci", State: tc.status},
		}))
		require.NoError(t, s.evaluateChecks(ctx, first.BaseSHA))
		unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{ID: first.ID, State: tc.want, ChecksStartedUnix: started})
	}
	unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{ID: next.ID, State: pull_model.MergeQueueStateAwaitingChecks})
	s.rule.MergeQueue.CheckResponseTimeoutMinutes = 30
	require.NoError(t, s.evaluateChecks(ctx, first.BaseSHA))
	unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{ID: first.ID, State: pull_model.MergeQueueStateUnmergeable})
	firstIssueID := s.entries[0].PullRequest.IssueID
	s.rule.MergeQueue.MaxEntriesToMerge = 1
	require.NoError(t, s.evaluateChecks(ctx, first.BaseSHA))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID: firstIssueID, Type: issues_model.CommentTypePRRemovedFromMergeQueue, Content: string(issues_model.MergeQueueRemovedChecksTimeout),
	})
}

func TestProcessQueueBuildFailuresAndDisabledPulls(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	emptyPR := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	conflictingPR := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 5})
	const branch = "queue-build-failures"
	require.NoError(t, git.ForceFastImport(ctx, repo, []git.FastImportCommit{{
		Ref: git.BranchPrefix + branch, Parent: git.BranchPrefix + emptyPR.HeadBranch,
		Files: []git.FastImportFile{{Path: "File-WoW", Content: "conflicting change"}},
	}}))
	defer func() { require.NoError(t, git.RemoveRef(ctx, repo, git.BranchPrefix+branch)) }()
	require.NoError(t, db.Insert(ctx, &git_model.ProtectedBranch{
		RepoID: repo.ID, RuleName: branch,
		ProtectedBranchConfig: git_model.ProtectedBranchConfig{RequireMergeQueue: true, MergeQueue: git_model.DefaultMergeQueueConfig()},
	}))
	enqueue := func(pr *issues_model.PullRequest) *pull_model.MergeQueueEntry {
		t.Helper()
		entry, err := Enqueue(ctx, repo.MustOwner(ctx), pr, false, "")
		require.NoError(t, err)
		return entry
	}
	for _, pr := range []*issues_model.PullRequest{emptyPR, conflictingPR} {
		pr.BaseBranch = branch
		require.NoError(t, pr.UpdateCols(ctx, "base_branch"))
		enqueue(pr)
	}

	t.Run("temporary repository unavailable", func(t *testing.T) {
		defer test.MockVariableValue(&setting.AppDataPath, filepath.Join(t.TempDir(), "missing"))()
		require.ErrorIs(t, processQueue(ctx, repo.ID, branch), fs.ErrNotExist)
		unittest.AssertCount(t, &pull_model.MergeQueueEntry{BaseBranch: branch, State: pull_model.MergeQueueStateQueued}, 2)
	})
	require.NoError(t, processQueue(ctx, repo.ID, branch))
	for pr, reason := range map[*issues_model.PullRequest]issues_model.MergeQueueRemovalReason{
		emptyPR: issues_model.MergeQueueRemovedNotMergeable, conflictingPR: issues_model.MergeQueueRemovedConflict,
	} {
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
			IssueID: pr.IssueID, Type: issues_model.CommentTypePRRemovedFromMergeQueue, Content: string(reason),
		})
	}

	base, err := git.GetBranchCommitID(ctx, repo, branch)
	require.NoError(t, err)
	ref := pull_model.MergeQueueGroupRef(branch, conflictingPR.Index, base)
	for i, archived := range []bool{true, false} {
		entry := enqueue(conflictingPR)
		entry.State = pull_model.MergeQueueStateAwaitingChecks
		entry.BaseSHA, entry.GroupHeadSHA, entry.GroupRef, entry.ChecksStartedUnix = base, entry.HeadSHA, ref, timeutil.TimeStampNow()
		require.NoError(t, git.UpdateRef(ctx, repo, ref, entry.HeadSHA))
		require.NoError(t, pull_model.UpdateMergeQueueEntry(ctx, entry))
		if archived {
			require.NoError(t, repo_model.SetArchiveRepoState(ctx, repo, true))
		} else {
			require.NoError(t, repo_service.UpdateRepositoryUnits(ctx, repo, nil, []unit.Type{unit.TypePullRequests}))
		}
		require.NoError(t, processQueue(ctx, repo.ID, branch))
		assert.False(t, git.IsReferenceExist(ctx, repo, ref))
		unittest.AssertCount(t, &issues_model.Comment{
			IssueID: conflictingPR.IssueID, Type: issues_model.CommentTypePRRemovedFromMergeQueue, Content: string(issues_model.MergeQueueRemovedNotMergeable),
		}, i+1)
		if archived {
			require.NoError(t, repo_model.SetArchiveRepoState(ctx, repo, false))
		}
	}
	unittest.AssertNotExistsBean(t, &pull_model.MergeQueueEntry{BaseBranch: branch})
}

func TestProcessQueueRestoresInterruptedMerge(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, db.Insert(ctx, &git_model.ProtectedBranch{
		RepoID: repo.ID, RuleName: pr.BaseBranch,
		ProtectedBranchConfig: git_model.ProtectedBranchConfig{RequireMergeQueue: true, MergeQueue: git_model.DefaultMergeQueueConfig()},
	}))
	_, err := Enqueue(ctx, repo.MustOwner(ctx), pr, false, "")
	require.NoError(t, err)

	// the queue commit reached the base branch, but the process stopped before the pull request was marked merged
	pr.MergedCommitID, err = git.GetBranchCommitID(ctx, repo, pr.BaseBranch)
	require.NoError(t, err)
	pr.MergedUnix, pr.MergerID = timeutil.TimeStampNow(), repo.OwnerID
	require.NoError(t, pr.UpdateCols(ctx, "merged_commit_id", "merged_unix", "merger_id"))

	require.NoError(t, processQueue(ctx, repo.ID, pr.BaseBranch))
	unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID, HasMerged: true})
	unittest.AssertNotExistsBean(t, &pull_model.MergeQueueEntry{PullID: pr.ID})
}
