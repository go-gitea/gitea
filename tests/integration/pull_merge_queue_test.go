// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	repo_module "gitea.dev/modules/repository"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/forms"
	"gitea.dev/services/mergequeue"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMergeQueue(t *testing.T, mergeMethod string) (*repo_model.Repository, *issues_model.PullRequest) {
	t.Helper()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: 1, Index: 3})
	pr.BaseBranch = "develop"
	require.NoError(t, pr.UpdateCols(t.Context(), "base_branch"))
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/branch_protections", &api.CreateBranchProtectionOption{
		RuleName: pr.BaseBranch, EnableStatusCheck: true, StatusCheckContexts: []string{"ci"},
		RequireMergeQueue: true, MergeQueue: &api.BranchProtectionMergeQueueOption{MergeMethod: &mergeMethod},
	}).AddBasicAuth("user2"), http.StatusCreated)
	return pr.BaseRepo, pr
}

func setMergeQueueStatus(t *testing.T, sha string, state commitstatus.CommitStatusState) {
	t.Helper()
	MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/statuses/"+sha, &api.CreateStatusOption{
		State: state, Context: "ci",
	}).AddBasicAuth("user2"), http.StatusCreated)
}

func watchQueueWebhook[T any](t *testing.T, event string, accept func(*T) bool, branchFilter ...string) func() *T {
	t.Helper()
	events := make(chan *T, 1)
	provider := newMockWebhookProvider(func(r *http.Request) {
		var payload T
		if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) && (accept == nil || accept(&payload)) {
			select {
			case events <- &payload:
			case <-t.Context().Done():
			}
		}
	}, http.StatusOK)
	t.Cleanup(provider.Close)
	testAPICreateWebhookForRepo(t, loginUser(t, "user2"), "user2", "repo1", provider.URL(), event, branchFilter...)
	return func() *T {
		t.Helper()
		select {
		case payload := <-events:
			return payload
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s webhook", event)
			return nil
		}
	}
}

func watchQueueRemovals(t *testing.T) func(*issues_model.PullRequest, string) {
	t.Helper()
	next := watchQueueWebhook(t, "pull_request", func(payload *api.PullRequestPayload) bool { return payload.Action == api.HookIssueDequeued })
	return func(pr *issues_model.PullRequest, reason string) {
		t.Helper()
		payload := next()
		assert.Equal(t, pr.Index, payload.Index)
		assert.Equal(t, reason, payload.Reason)
	}
}

func TestPullMergeQueue(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := t.Context()
		repo, pr := setupMergeQueue(t, "squash")
		user := repo.MustOwner(ctx)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user2/repo1", &api.EditRepoOption{
			SquashMergeCommitMessage: new(string(repo_model.SquashMergeMessageCommitMessages)),
		}).AddBasicAuth(user.Name), http.StatusOK)
		gitRepo, err := git.OpenRepository(ctx, repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		require.NoError(t, repo_service.SyncBranchesToDB(ctx, repo, user.ID, gitRepo, []string{pr.BaseBranch}))
		headSHA, err := gitRepo.GetBranchCommitID(ctx, pr.HeadBranch)
		require.NoError(t, err)
		setMergeQueueStatus(t, headSHA, commitstatus.CommitStatusSuccess)
		pullURL := fmt.Sprintf("/api/v1/repos/user2/repo1/pulls/%d", pr.Index)
		MakeRequest(t, NewRequestWithJSON(t, "POST", pullURL+"/merge", &forms.MergePullRequestForm{Do: "merge", HeadCommitID: pr.MergeBase}).AddBasicAuth(user.Name), http.StatusConflict)

		nextPush := watchQueueWebhook[api.PushPayload](t, "push", nil, "gh-readonly-queue/**")
		assertRemoved := watchQueueRemovals(t)
		MakeRequest(t, NewRequestWithJSON(t, "POST", pullURL+"/merge-queue", &api.AddToMergeQueueOption{}).AddBasicAuth(user.Name), http.StatusCreated)
		push := nextPush()
		assert.Equal(t, 1, push.TotalCommits)
		groupSHA := push.After
		_, err = repo_service.RenameBranch(ctx, repo, user, pr.BaseBranch, "renamed")
		require.NoError(t, err)
		setMergeQueueStatus(t, groupSHA, commitstatus.CommitStatusSuccess)
		assertRemoved(pr, "MERGE")
		merged, err := gitRepo.GetBranchCommit(ctx, "renamed")
		require.NoError(t, err)
		assert.Equal(t, groupSHA, merged.ID.String())
		assert.Contains(t, merged.CommitMessage.MessageRaw, "* a change\n\n* make pull5 outdated")
		assert.Contains(t, merged.MessageTrailer()["co-authored-by"], "6543 <6543@obermui.de>")
	})
}

func insertBuiltQueueEntry(t *testing.T, repo *repo_model.Repository, pr *issues_model.PullRequest) *pull_model.MergeQueueEntry {
	t.Helper()
	ctx := t.Context()
	baseSHA, err := git.GetBranchCommitID(ctx, repo, pr.BaseBranch)
	require.NoError(t, err)
	headSHA, err := git.GetFullCommitID(ctx, repo, pr.GetGitHeadRefName())
	require.NoError(t, err)
	entry := &pull_model.MergeQueueEntry{
		RepoID: repo.ID, PullID: pr.ID, EnqueuerID: repo.OwnerID, BaseBranch: pr.BaseBranch,
		BaseSHA: baseSHA, HeadSHA: headSHA, GroupHeadSHA: headSHA,
		GroupRef: pull_model.MergeQueueGroupRef(pr.BaseBranch, pr.Index, baseSHA),
		State:    pull_model.MergeQueueStateAwaitingChecks, ChecksStartedUnix: timeutil.TimeStampNow(),
	}
	require.NoError(t, git.UpdateRef(ctx, repo, entry.GroupRef, entry.GroupHeadSHA))
	require.NoError(t, db.Insert(ctx, entry))
	return entry
}

func TestPullMergeQueueRebuild(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := t.Context()
		repo, first := setupMergeQueue(t, "merge")
		second := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: 5})
		second.BaseBranch = first.BaseBranch
		require.NoError(t, second.UpdateCols(ctx, "base_branch"))
		seed := insertBuiltQueueEntry(t, repo, first)
		nextPush := watchQueueWebhook[api.PushPayload](t, "push", nil, "gh-readonly-queue/**")
		assertRemoved := watchQueueRemovals(t)
		_, err := mergequeue.Enqueue(ctx, repo.MustOwner(ctx), second, false, "")
		require.NoError(t, err)
		refPrefix := fmt.Sprintf("refs/heads/gh-readonly-queue/%s/pr-%d-", first.BaseBranch, second.Index)
		built := nextPush()
		assert.Equal(t, refPrefix+seed.GroupHeadSHA, built.Ref)
		setMergeQueueStatus(t, seed.GroupHeadSHA, commitstatus.CommitStatusFailure)
		assertRemoved(first, "CI_FAILURE")
		assert.Equal(t, refPrefix+seed.BaseSHA, nextPush().Ref)
		assert.False(t, git.IsReferenceExist(ctx, repo, built.Ref))

		require.NoError(t, git.DeleteBranch(ctx, repo, first.BaseBranch, true))
		require.NoError(t, mergequeue.CheckAllQueues(ctx))
		assertRemoved(second, "QUEUE_CLEARED")
	})
}

func TestPullMergeQueueBranchProtection(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := t.Context()
		repo, pr := setupMergeQueue(t, "rebase")
		user := repo.MustOwner(ctx)
		entry := insertBuiltQueueEntry(t, repo, pr)
		branch := git.RefName(entry.GroupRef).BranchName()

		_, err := repo_module.SyncRepoBranches(ctx, repo.ID, user.ID)
		require.NoError(t, err)
		unittest.AssertNotExistsBean(t, &git_model.Branch{RepoID: repo.ID, Name: branch})
		_, err = repo_service.RenameBranch(ctx, repo, user, repo.DefaultBranch, branch)
		require.ErrorIs(t, err, git_model.ErrBranchIsProtected)
		testAPIDeleteBranch(t, branch, http.StatusForbidden)
		push := func(refspec string) error {
			return git.PushManaged(ctx, repo, repo, git.PushOptions{Branch: refspec, Env: repo_module.PushingEnvironment(user, repo)})
		}
		require.ErrorContains(t, push(":"+entry.GroupRef), "reserved for the merge queue")
		require.ErrorContains(t, push("+pr-to-update:"+git.BranchPrefix+pr.HeadBranch), "To modify this branch, dequeue the associated pull request.")
		pullURL := fmt.Sprintf("/api/v1/repos/user2/repo1/pulls/%d", pr.Index)
		req := NewRequestWithJSON(t, "PATCH", pullURL, &api.EditPullRequestOption{Base: repo.DefaultBranch})
		assert.Contains(t, MakeRequest(t, req.AddBasicAuth(user.Name), http.StatusUnprocessableEntity).Body.String(), "Cannot change the base branch")

		assertRemoved := watchQueueRemovals(t)
		req = NewRequestWithJSON(t, "PATCH", pullURL, &api.EditPullRequestOption{Title: "WIP: queued change"})
		MakeRequest(t, req.AddBasicAuth(user.Name), http.StatusCreated)
		setMergeQueueStatus(t, entry.GroupHeadSHA, commitstatus.CommitStatusSuccess)
		assertRemoved(pr, "BRANCH_PROTECTIONS")
		testAPICreateBranch(t, loginUser(t, user.Name), user.Name, repo.Name, pr.BaseBranch, branch, http.StatusCreated)
	})
}
