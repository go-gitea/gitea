// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIfNeedApproval(t *testing.T) {
	alwaysWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
		return true, nil
	}
	neverWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
		return false, nil
	}
	hasMerged := func(_ context.Context, _, _ int64) (bool, error) { return true, nil }
	noMerged := func(_ context.Context, _, _ int64) (bool, error) { return false, nil }
	errPerm := errors.New("perm error")
	errMerge := errors.New("merge error")

	forkRun := &actions_model.ActionRun{IsForkPullRequest: true, TriggerEvent: actions_module.GithubEventPullRequest}
	nonForkRun := &actions_model.ActionRun{IsForkPullRequest: false, TriggerEvent: actions_module.GithubEventPullRequest}
	prTargetRun := &actions_model.ActionRun{IsForkPullRequest: true, TriggerEvent: actions_module.GithubEventPullRequestTarget}

	repo := &repo_model.Repository{ID: 1}
	normalUser := &user_model.User{ID: 10}
	restrictedUser := &user_model.User{ID: 11, IsRestricted: true}

	for _, tt := range []struct {
		name      string
		run       *actions_model.ActionRun
		user      *user_model.User
		canWrite  func(context.Context, *repo_model.Repository, *user_model.User) (bool, error)
		hasMerged func(context.Context, int64, int64) (bool, error)
		check     assert.BoolAssertionFunc
	}{
		{"not a fork PR never needs approval", nonForkRun, normalUser, alwaysWrite, hasMerged, assert.False},
		{"pull_request_target never needs approval even when fork", prTargetRun, normalUser, alwaysWrite, hasMerged, assert.False},
		{"restricted user always needs approval", forkRun, restrictedUser, alwaysWrite, hasMerged, assert.True},
		{"fork PR with write permission does not need approval", forkRun, normalUser, alwaysWrite, noMerged, assert.False},
		{"fork PR with merged PR but no write permission does not need approval", forkRun, normalUser, neverWrite, hasMerged, assert.False},
		{"fork PR with no write and no merged PR needs approval", forkRun, normalUser, neverWrite, noMerged, assert.True},
	} {
		t.Run(tt.name, func(t *testing.T) {
			need, err := ifNeedApprovalWith(t.Context(), tt.run, repo, tt.user, tt.canWrite, tt.hasMerged)
			require.NoError(t, err)
			tt.check(t, need)
		})
	}

	t.Run("canWriteActions error is propagated", func(t *testing.T) {
		failWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
			return false, errPerm
		}
		_, err := ifNeedApprovalWith(t.Context(), forkRun, repo, normalUser, failWrite, noMerged)
		require.ErrorIs(t, err, errPerm)
	})

	t.Run("hasMergedPR error is propagated", func(t *testing.T) {
		failMerge := func(_ context.Context, _, _ int64) (bool, error) { return false, errMerge }
		_, err := ifNeedApprovalWith(t.Context(), forkRun, repo, normalUser, neverWrite, failMerge)
		require.ErrorIs(t, err, errMerge)
	})

	t.Run("restricted user skips permission check entirely", func(t *testing.T) {
		// The perm and merge functions must not be called for a restricted user.
		called := false
		trackWrite := func(_ context.Context, _ *repo_model.Repository, _ *user_model.User) (bool, error) {
			called = true
			return true, nil
		}
		need, err := ifNeedApprovalWith(t.Context(), forkRun, repo, restrictedUser, trackWrite, noMerged)
		require.NoError(t, err)
		assert.True(t, need)
		assert.False(t, called, "permission check must not run for restricted user")
	})
}

func TestGetApprovalUsersAddsForkPullRequestAuthorUnlessDefaultBranchWorkflow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	approvalUsers, err := getApprovalUsers(t.Context(), &notifyInput{Doer: doer, PullRequest: pr, Event: webhook_module.HookEventPullRequest}, true)
	require.NoError(t, err)
	require.Len(t, approvalUsers, 2)
	assert.Equal(t, []int64{doer.ID, pr.Issue.PosterID}, []int64{approvalUsers[0].ID, approvalUsers[1].ID})

	approvalUsers, err = getApprovalUsers(t.Context(), &notifyInput{Doer: doer, PullRequest: pr, Event: webhook_module.HookEventIssueComment}, true)
	require.NoError(t, err)
	assert.Equal(t, []*user_model.User{doer}, approvalUsers)
}

func TestFilteredWorkflowCommitStatusForForkPullRequest(t *testing.T) {
	input := new(notifyInput).WithPullRequest(&issues_model.PullRequest{
		Flow:       issues_model.PullRequestFlowGithub,
		BaseRepoID: 1,
		HeadRepoID: 2,
		Index:      1,
	})

	assert.True(t, isForkPullRequestInput(input))
	assert.Equal(t, "refs/pull/1/head", input.Ref.String())
	assert.False(t, shouldCreateSkippedCommitStatusForFilteredWorkflow(input, &actions_module.DetectedWorkflow{
		TriggerEvent: &jobparser.Event{Name: actions_module.GithubEventPullRequest},
	}))
	assert.True(t, shouldCreateSkippedCommitStatusForFilteredWorkflow(input, &actions_module.DetectedWorkflow{
		TriggerEvent: &jobparser.Event{Name: actions_module.GithubEventPullRequestTarget},
	}))

	assert.True(t, shouldCreateSkippedCommitStatusForFilteredWorkflow(newNotifyInput(&repo_model.Repository{ID: 1}, &user_model.User{ID: 2}, actions_module.GithubEventPullRequest), &actions_module.DetectedWorkflow{
		TriggerEvent: &jobparser.Event{Name: actions_module.GithubEventPullRequest},
	}))
}

func TestDetectAndHandleSchedulesDropsRemovedWorkflows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, actions_model.ReplaceScheduleTasksByRepo(t.Context(), repo.ID, []*actions_model.ActionSchedule{{RepoID: repo.ID, WorkflowID: "removed.yml"}}))

	require.NoError(t, DetectAndHandleSchedules(t.Context(), repo))
	unittest.AssertNotExistsBean(t, &actions_model.ActionSchedule{RepoID: repo.ID})
}

func TestNotifyPullRequestWithDeletedBaseBranch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.Issue.LoadRepo(t.Context()))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: pr.Issue.Repo.OwnerID})
	pr.BaseBranch = "deleted"
	input := newNotifyInput(pr.Issue.Repo, doer, webhook_module.HookEventPullRequest).
		WithPayload(&api.PullRequestPayload{Action: api.HookIssueDequeued}).
		WithPullRequest(pr)
	require.NoError(t, notify(t.Context(), input))
}

func TestMergeGroupDestroyedAfterRefDeletion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	require.NoError(t, db.Insert(t.Context(), &git_model.ProtectedBranch{
		RepoID: repo.ID, RuleName: repo.DefaultBranch,
		ProtectedBranchConfig: git_model.ProtectedBranchConfig{EnableStatusCheck: true, StatusCheckContexts: []string{"*"}},
	}))

	const groupRef = "refs/heads/gh-readonly-queue/master/pr-1"
	const jobs = "\njobs: {cleanup: {runs-on: linux, steps: [{run: echo done}]}}"
	require.NoError(t, git.ForceFastImport(t.Context(), repo, []git.FastImportCommit{{
		Ref: groupRef,
		Files: []git.FastImportFile{
			{Path: ".gitea/workflows/cleanup.yml", Content: "on: {merge_group: {types: [destroyed]}}" + jobs},
			{Path: ".gitea/workflows/default.yml", Content: "on: merge_group" + jobs},
			{Path: ".gitea/workflows/filtered.yml", Content: "on: {merge_group: {types: [destroyed], branches: [other]}}" + jobs},
			{Path: ".gitea/workflows/invalid.yml", Content: "on: {merge_group: {types: [destroyed]}}\njobs: {cleanup: {uses: ./.gitea/workflows/missing.yml}}"},
		},
	}}))
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	headSHA, err := gitRepo.GetRefCommitID(t.Context(), groupRef)
	require.NoError(t, err)

	notifier := new(actionsNotifier)
	entry := &pull_model.MergeQueueEntry{GroupRef: groupRef, GroupHeadSHA: headSHA, BaseBranch: repo.DefaultBranch}
	notifier.MergeGroupChanged(t.Context(), doer, repo, entry, api.HookMergeGroupChecksRequested, "")
	unittest.AssertExistsAndLoadBean(t, &git_model.CommitStatus{RepoID: repo.ID, SHA: headSHA, Context: "default.yml / cleanup (merge_group)"})
	require.NoError(t, git.RemoveRef(t.Context(), repo, groupRef))
	notifier.MergeGroupChanged(t.Context(), doer, repo, entry, api.HookMergeGroupDestroyed, "dequeued")

	run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, WorkflowID: "cleanup.yml", Event: webhook_module.HookEventMergeGroup})
	assert.Equal(t, groupRef, run.Ref)
	assert.Equal(t, headSHA, run.CommitSHA)
	assert.Equal(t, headSHA, run.WorkflowCommitSHA)
	var payload api.MergeGroupPayload
	require.NoError(t, json.Unmarshal([]byte(run.EventPayload), &payload))
	assert.Equal(t, api.HookMergeGroupDestroyed, payload.Action)
	assert.Equal(t, "dequeued", payload.Reason)
	require.NotNil(t, payload.MergeGroup)
	assert.Equal(t, groupRef, payload.MergeGroup.HeadRef)
	assert.Equal(t, headSHA, payload.MergeGroup.HeadSHA)
	unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, Name: "cleanup"})
	unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, WorkflowID: "invalid.yml", Ref: groupRef, Status: actions_model.StatusFailure})
	unittest.AssertCount(t, &actions_model.ActionRun{RepoID: repo.ID, Ref: groupRef}, 3)
	unittest.AssertCount(t, &git_model.CommitStatus{RepoID: repo.ID, SHA: headSHA}, 1)
}
