// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/private"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/agit"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func queueWithPendingChecks(t *testing.T) *queueState {
	t.Helper()
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	const branch = "master"
	first := &pull_model.MergeQueueEntry{
		RepoID: repo.ID, BaseBranch: branch, PullID: 2, EnqueuerID: repo.OwnerID, BaseSHA: "base", GroupHeadSHA: strings.Repeat("a", 40),
		GroupRef: pull_model.MergeQueueGroupRef(branch, 3, "base"), ChecksStartedUnix: timeutil.TimeStampNow().AddDuration(-time.Hour),
	}
	next := &pull_model.MergeQueueEntry{
		RepoID: repo.ID, BaseBranch: branch, PullID: 5, EnqueuerID: repo.OwnerID, BaseSHA: first.GroupHeadSHA, GroupHeadSHA: strings.Repeat("b", 40),
		GroupRef: pull_model.MergeQueueGroupRef(branch, 5, first.GroupHeadSHA), ChecksStartedUnix: timeutil.TimeStampNow(),
	}
	require.NoError(t, db.Insert(ctx, first, next))
	entries, err := GetQueue(ctx, repo, branch)
	require.NoError(t, err)
	return &queueState{repo: repo, branch: branch, entries: entries, rule: &git_model.ProtectedBranch{}}
}

func setQueueCheck(t *testing.T, entry *QueueEntry, status commitstatus.CommitStatusState) {
	t.Helper()
	require.NoError(t, git_model.NewCommitStatus(t.Context(), git_model.NewCommitStatusOptions{
		Repo: entry.PullRequest.BaseRepo, Creator: entry.Enqueuer, SHA: git.MustIDFromString(entry.GroupHeadSHA),
		CommitStatus: &git_model.CommitStatus{Context: "ci", State: status},
	}))
}

func readyGroup(t *testing.T, state *queueState) []*QueueEntry {
	t.Helper()
	group, err := state.readyGroup(t.Context(), state.entries[0].BaseSHA)
	require.NoError(t, err)
	return group
}

func checkQueueStatus(t *testing.T, state *queueState, entry *QueueEntry, status commitstatus.CommitStatusState, want pull_model.MergeQueueEntryState) {
	t.Helper()
	setQueueCheck(t, entry, status)
	readyGroup(t, state)
	unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{ID: entry.ID, State: want})
}

func assertRemoved(t *testing.T, pr *issues_model.PullRequest, reason issues_model.MergeQueueRemovalReason) {
	t.Helper()
	unittest.AssertNotExistsBean(t, &pull_model.MergeQueueEntry{PullID: pr.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr.IssueID, Type: issues_model.CommentTypePRRemovedFromMergeQueue, Content: string(reason)})
}

func TestMergeQueueCheckRerunsAndHeadGreen(t *testing.T) {
	state := queueWithPendingChecks(t)
	first, next := state.entries[0], state.entries[1]
	state.rule.EnableStatusCheck = true
	state.rule.MergeQueue = git_model.MergeQueueConfig{GroupingStrategy: git_model.MergeQueueGroupingHeadGreen, MaxEntriesToMerge: 2, CheckResponseTimeoutMinutes: 180}

	checkQueueStatus(t, state, first, commitstatus.CommitStatusSuccess, pull_model.MergeQueueStateMergeable)
	checkQueueStatus(t, state, first, commitstatus.CommitStatusFailure, pull_model.MergeQueueStateUnmergeable)
	checkQueueStatus(t, state, first, commitstatus.CommitStatusSuccess, pull_model.MergeQueueStateMergeable)
	checkQueueStatus(t, state, first, commitstatus.CommitStatusPending, pull_model.MergeQueueStateAwaitingChecks)
	state.rule.MergeQueue.CheckResponseTimeoutMinutes = 30
	setQueueCheck(t, next, commitstatus.CommitStatusSuccess)
	assert.Len(t, readyGroup(t, state), 2)
	unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{ID: first.ID, State: pull_model.MergeQueueStateUnmergeable})
	state.rule.MergeQueue.MaxEntriesToMerge = 1
	readyGroup(t, state)
	assertRemoved(t, first.PullRequest, issues_model.MergeQueueRemovedChecksTimeout)
}

func TestMergeQueueAllGreen(t *testing.T) {
	state := queueWithPendingChecks(t)
	first, next := state.entries[0], state.entries[1]
	state.rule.MergeQueue = git_model.DefaultMergeQueueConfig()
	state.rule.MergeQueue.CheckResponseTimeoutMinutes = 180

	require.NoError(t, db.Insert(t.Context(), &actions_model.ActionScopedWorkflowSource{
		OwnerID: state.repo.OwnerID, SourceRepoID: state.repo.ID,
		WorkflowConfigs: map[string]*actions_model.ScopedWorkflowConfig{"ci.yaml": {Required: true, Patterns: []string{"ci"}}},
	}))
	checkQueueStatus(t, state, next, commitstatus.CommitStatusFailure, pull_model.MergeQueueStateUnmergeable)
	setQueueCheck(t, first, commitstatus.CommitStatusSuccess)
	assert.Equal(t, []*QueueEntry{first}, readyGroup(t, state))
	assertRemoved(t, next.PullRequest, issues_model.MergeQueueRemovedChecksFailed)
}

func prepareQueuePulls(t *testing.T, branch string, minEntries int) (*git_model.ProtectedBranch, *issues_model.PullRequest, *issues_model.PullRequest) {
	t.Helper()
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	rule := &git_model.ProtectedBranch{
		Repo: repo, RepoID: repo.ID, RuleName: branch,
		ProtectedBranchConfig: git_model.ProtectedBranchConfig{RequireMergeQueue: true, MergeQueue: git_model.DefaultMergeQueueConfig()},
	}
	rule.MergeQueue.MinEntriesToMerge = minEntries
	require.NoError(t, db.Insert(ctx, rule))
	first := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	next := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 5})
	for _, pr := range []*issues_model.PullRequest{first, next} {
		pr.BaseRepo, pr.BaseBranch = repo, branch
		require.NoError(t, pr.UpdateCols(ctx, "base_branch"))
	}
	return rule, first, next
}

func enqueuePull(t *testing.T, pr *issues_model.PullRequest, jump bool) *pull_model.MergeQueueEntry {
	t.Helper()
	entry, _, err := Enqueue(t.Context(), pr.BaseRepo.MustOwner(t.Context()), pr, jump, "")
	require.NoError(t, err)
	return entry
}

func TestProcessQueueBuildFailuresAndDisabledPulls(t *testing.T) {
	ctx := t.Context()
	const branch = "queue-build-failures"
	rule, emptyPR, conflictingPR := prepareQueuePulls(t, branch, 1)
	repo := rule.Repo
	require.NoError(t, git.ForceFastImport(ctx, repo, []git.FastImportCommit{{
		Ref: git.BranchPrefix + branch, Parent: git.BranchPrefix + emptyPR.HeadBranch,
		Files: []git.FastImportFile{{Path: "File-WoW", Content: "conflicting change"}},
	}}))
	defer func() { require.NoError(t, git.RemoveRef(ctx, repo, git.BranchPrefix+branch)) }()
	enqueuePull(t, emptyPR, false)
	enqueuePull(t, conflictingPR, false)

	t.Run("temporary repository unavailable", func(t *testing.T) {
		defer test.MockVariableValue(&setting.AppDataPath, filepath.Join(t.TempDir(), "missing"))()
		require.ErrorIs(t, processQueue(ctx, repo.ID, branch), fs.ErrNotExist)
		unittest.AssertCount(t, &pull_model.MergeQueueEntry{BaseBranch: branch, State: pull_model.MergeQueueStateQueued}, 2)
	})
	require.NoError(t, processQueue(ctx, repo.ID, branch))
	assertRemoved(t, emptyPR, issues_model.MergeQueueRemovedNotMergeable)
	assertRemoved(t, conflictingPR, issues_model.MergeQueueRemovedConflict)

	base, err := git.GetBranchCommitID(ctx, repo, branch)
	require.NoError(t, err)
	entry := enqueuePull(t, conflictingPR, false)
	entry.BaseSHA, entry.GroupHeadSHA, entry.GroupRef = base, entry.HeadSHA, pull_model.MergeQueueGroupRef(branch, conflictingPR.Index, base)
	require.NoError(t, git.UpdateRef(ctx, repo, entry.GroupRef, entry.HeadSHA))
	require.NoError(t, pull_model.UpdateMergeQueueEntry(ctx, entry))
	require.NoError(t, repo_service.UpdateRepositoryUnits(ctx, repo, nil, []unit.Type{unit.TypePullRequests}))
	require.NoError(t, processQueue(ctx, repo.ID, branch))
	assert.False(t, git.IsReferenceExist(ctx, repo, entry.GroupRef))
	assertRemoved(t, conflictingPR, issues_model.MergeQueueRemovedNotMergeable)
}

type mergeQueueEvents struct {
	notify_service.NullNotifier
	branch string
	log    []string
}

func (events *mergeQueueEvents) PullRequestMergeQueueChanged(_ctx context.Context, _doer *user_model.User, pr *issues_model.PullRequest, action api.HookIssueAction, reason issues_model.MergeQueueRemovalReason) {
	if pr.BaseBranch == events.branch && action == api.HookIssueDequeued {
		events.log = append(events.log, fmt.Sprintf("%d %s merged=%t closed=%t", pr.ID, reason, pr.HasMerged, pr.Issue.IsClosed))
	}
}

func (events *mergeQueueEvents) MergeGroupChanged(_ctx context.Context, _doer *user_model.User, _repo *repo_model.Repository, entry *pull_model.MergeQueueEntry, action api.HookMergeGroupAction, reason string) {
	if entry.BaseBranch == events.branch && action == api.HookMergeGroupDestroyed {
		events.log = append(events.log, fmt.Sprintf("%d %s", entry.PullID, reason))
	}
}

func failPushesWithConnectionError(t *testing.T, repo *repo_model.Repository) func() {
	t.Helper()
	hooks := t.TempDir()
	require.NoError(t, git.ManagedConfigSet(t.Context(), repo, "core.hooksPath", hooks))
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-receive"), []byte("#!/bin/sh\necho 'Gitea: "+private.InternalServerConnectionError+"' >&2\nexit 1\n"), 0o755))
	return func() {
		require.NoError(t, gitcmd.NewCommand("config", "--unset", "core.hooksPath").WithRepo(repo).RunWithStderr(t.Context()))
	}
}

func TestProcessQueueRestoresInterruptedMerge(t *testing.T) {
	defer test.MockVariableValue(&setting.Repository.PullRequest.DefaultDeleteBranchAfterMerge, false)()
	ctx := t.Context()
	const branch = "queue-recovery"
	rule, regularPR, priorityPR := prepareQueuePulls(t, branch, 3)
	repo := rule.Repo
	owner := repo.MustOwner(ctx)
	require.NoError(t, git.CreateBranch(ctx, repo, branch, repo.DefaultBranch))
	defer func() { require.NoError(t, git.RemoveRef(ctx, repo, git.BranchPrefix+branch)) }()
	require.NoError(t, pull_model.ScheduleAutoMerge(ctx, owner, regularPR.ID, repo_model.MergeStyleMerge, "", false))
	defer timeutil.MockSet(time.Now())()
	enqueuePull(t, priorityPR, true)
	queued := enqueuePull(t, regularPR, false)
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: regularPR.ID})
	_, _, err := Enqueue(ctx, owner, regularPR, false, "")
	require.ErrorIs(t, err, pull_model.ErrAlreadyInMergeQueue)
	assert.Equal(t, queued.ID, enqueuePull(t, regularPR, true).ID)
	require.NoError(t, processQueue(ctx, repo.ID, branch))
	entries, err := pull_model.GetMergeQueueEntries(ctx, repo.ID, branch)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, queued.ID, entries[0].ID)
	for i, entry := range entries {
		position, err := pull_model.GetMergeQueuePosition(ctx, entry)
		require.NoError(t, err)
		assert.Equal(t, i+1, position)
	}

	events := &mergeQueueEvents{branch: branch}
	notify_service.RegisterNotifier(events)
	defer func() { events.branch = "" }()
	defer failPushesWithConnectionError(t, repo)()
	require.NoError(t, processQueue(ctx, repo.ID, branch))
	_, err = db.GetEngine(ctx).ID(entries[1].ID).Cols("created_unix").Update(&pull_model.MergeQueueEntry{CreatedUnix: timeutil.TimeStampNow().AddDuration(-time.Hour)})
	require.NoError(t, err)
	require.ErrorContains(t, processQueue(ctx, repo.ID, branch), private.InternalServerConnectionError)
	for _, entry := range entries {
		unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{ID: entry.ID, State: pull_model.MergeQueueStateMergeable})
		pr := &issues_model.PullRequest{ID: entry.PullID, MergedCommitID: entry.GroupHeadSHA, MergerID: owner.ID, MergedUnix: timeutil.TimeStampNow()}
		require.NoError(t, pr.UpdateCols(ctx, "merged_commit_id", "merger_id", "merged_unix"))
	}
	require.NoError(t, git.UpdateRef(ctx, repo, git.BranchPrefix+branch, entries[1].GroupHeadSHA))
	require.NoError(t, git_model.DeleteProtectedBranch(ctx, repo, rule.ID))
	require.NoError(t, processQueue(ctx, repo.ID, branch))
	assert.Equal(t, []string{"2 merged", "2 merged merged=true closed=true", "5 merged", "5 merged merged=true closed=true"}, events.log)
	unittest.AssertNotExistsBean(t, &pull_model.MergeQueueEntry{BaseBranch: branch})
	for _, entry := range entries {
		assert.False(t, git.IsReferenceExist(ctx, repo, entry.GroupRef))
	}
}

func TestProcessQueueDropsEntryChangingProtectedFiles(t *testing.T) {
	ctx := t.Context()
	const branch = "queue-protected-files"
	rule, _, pr := prepareQueuePulls(t, branch, 1)
	repo := rule.Repo
	require.NoError(t, git.CreateBranch(ctx, repo, branch, repo.DefaultBranch))
	defer func() { require.NoError(t, git.RemoveRef(ctx, repo, git.BranchPrefix+branch)) }()
	enqueuePull(t, pr, false)
	require.NoError(t, processQueue(ctx, repo.ID, branch))

	defer failPushesWithConnectionError(t, repo)()
	rule.ProtectedFilePatterns = "file-wow"
	_, err := db.GetEngine(ctx).ID(rule.ID).Cols("config").Update(rule)
	require.NoError(t, err)
	require.NoError(t, processQueue(ctx, repo.ID, branch))
	assertRemoved(t, pr, issues_model.MergeQueueRemovedNotMergeable)
}

func TestProcessQueueMissingHeads(t *testing.T) {
	ctx := t.Context()
	rule, missingRepo, missingRef := prepareQueuePulls(t, "master", 1)
	repo := rule.Repo
	enqueuePull(t, missingRepo, false)
	entry := enqueuePull(t, missingRef, false)
	t.Run("queued head rejects rename, update and AGit update", func(t *testing.T) {
		doer := repo.MustOwner(ctx)
		_, err := repo_service.RenameBranch(ctx, repo, doer, missingRepo.HeadBranch, "new-head")
		require.ErrorIs(t, err, issues_model.ErrQueuedPullHead)
		require.ErrorIs(t, pull_service.Update(missingRepo, doer, "", false), issues_model.ErrQueuedPullHead)
		missingRepo.Flow, missingRepo.HeadBranch = issues_model.PullRequestFlowAGit, doer.LowerName+"/topic"
		require.NoError(t, missingRepo.UpdateCols(ctx, "flow", "head_branch"))
		gitRepo, err := git.OpenRepository(ctx, repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		results, err := agit.ProcReceive(ctx, repo, gitRepo, access_model.Permission{}, &agit.ProcReceiveOptions{
			OldCommitIDs: []string{""}, NewCommitIDs: []string{missingRepo.MergeBase}, RefFullNames: []git.RefName{"refs/for/master/topic"}, Doer: doer,
		})
		require.NoError(t, err)
		assert.Contains(t, results[0].Err, "merge queue")
	})
	missingRepo.HeadRepoID = unittest.NonexistentID
	require.NoError(t, missingRepo.UpdateCols(ctx, "head_repo_id"))
	require.NoError(t, git.RemoveRef(ctx, repo, missingRef.GetGitHeadRefName()))
	defer func() { require.NoError(t, git.UpdateRef(ctx, repo, missingRef.GetGitHeadRefName(), entry.HeadSHA)) }()
	require.NoError(t, processQueue(ctx, repo.ID, missingRepo.BaseBranch))
	assertRemoved(t, missingRepo, issues_model.MergeQueueRemovedHeadChanged)
	assertRemoved(t, missingRef, issues_model.MergeQueueRemovedHeadChanged)
}
