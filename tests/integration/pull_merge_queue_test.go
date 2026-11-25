// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/private"
	"gitea.dev/modules/queue"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/forms"
	"gitea.dev/services/mergequeue"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mergeQueueAPI(t *testing.T, method, path string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	return MakeRequest(t, NewRequestWithJSON(t, method, "/api/v1/repos/user2/repo1"+path, body).AddBasicAuth("user2"), status)
}

func protectMergeQueue(t *testing.T) {
	t.Helper()
	mergeQueueAPI(t, "POST", "/branch_protections", api.CreateBranchProtectionOption{
		RuleName: "master", RequireMergeQueue: true, BlockAdminMergeOverride: true,
		EnableStatusCheck: true, StatusCheckContexts: []string{"ci"},
		MergeQueue: &api.BranchProtectionMergeQueueOption{MaxEntriesToBuild: new(0)},
	}, http.StatusCreated)
}

func setMergeQueueStatus(t *testing.T, sha string, state commitstatus.CommitStatusState) {
	t.Helper()
	mergeQueueAPI(t, "POST", "/statuses/"+sha, api.CreateStatusOption{State: state, Context: "ci"}, http.StatusCreated)
}

func flushMergeQueue(t *testing.T) {
	t.Helper()
	for _, worker := range queue.GetManager().ManagedQueues() {
		if worker.GetName() == "merge_queue" {
			require.NoError(t, worker.FlushWithContext(t.Context(), 5*time.Second))
			return
		}
	}
	t.Fatal("merge queue worker not found")
}

func seedMergeQueueGroup(t *testing.T, repo *repo_model.Repository, entry *pull_model.MergeQueueEntry, index int64, baseSHA, groupSHA string) {
	t.Helper()
	entry.BaseSHA, entry.GroupHeadSHA = baseSHA, groupSHA
	entry.GroupRef = pull_model.MergeQueueGroupRef(entry.BaseBranch, index, baseSHA)
	entry.State, entry.ChecksStartedUnix = pull_model.MergeQueueStateAwaitingChecks, timeutil.TimeStampNow()
	require.NoError(t, git.UpdateRef(t.Context(), repo, entry.GroupRef, groupSHA))
}

func insertMergeQueueGroup(t *testing.T, repo *repo_model.Repository, pr *issues_model.PullRequest, baseSHA string) *pull_model.MergeQueueEntry {
	t.Helper()
	headSHA, err := git.GetFullCommitID(t.Context(), repo, pr.GetGitHeadRefName())
	require.NoError(t, err)
	entry := &pull_model.MergeQueueEntry{RepoID: repo.ID, PullID: pr.ID, EnqueuerID: repo.OwnerID, BaseBranch: pr.BaseBranch, HeadSHA: headSHA}
	seedMergeQueueGroup(t, repo, entry, pr.Index, baseSHA, headSHA)
	require.NoError(t, db.Insert(t.Context(), entry))
	return entry
}

func prepareMergeQueueRepo(t *testing.T) *repo_model.Repository {
	t.Helper()
	require.NoError(t, unittest.LoadFixtures())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, unittest.SyncDirs(filepath.Join(setting.GetGiteaTestSourceRoot(), "tests/gitea-repositories-meta", repo.GitRepoLocation()), gitrepo.RepoLocalPath(repo)))
	return repo
}

func TestPullMergeQueue(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	ctx := t.Context()
	repo := prepareMergeQueueRepo(t)
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: 3})
	require.NoError(t, git.ManagedConfigSet(ctx, repo, "core.hooksPath", t.TempDir()))
	defer func() {
		require.NoError(t, gitcmd.NewCommand("config", "--unset", "core.hooksPath").WithRepo(repo).RunWithStderr(ctx))
	}()
	protectMergeQueue(t)
	headSHA, err := git.GetFullCommitID(ctx, repo, pr.GetGitHeadRefName())
	require.NoError(t, err)
	baseSHA, err := git.GetBranchCommitID(ctx, repo, pr.BaseBranch)
	require.NoError(t, err)
	setMergeQueueStatus(t, headSHA, commitstatus.CommitStatusSuccess)

	const pullURL = "/pulls/3"
	mergeQueueAPI(t, "POST", pullURL+"/merge", forms.MergePullRequestForm{Do: "merge", HeadCommitID: pr.MergeBase}, http.StatusConflict)
	added := DecodeJSON(t, mergeQueueAPI(t, "POST", pullURL+"/merge-queue", api.AddToMergeQueueOption{Jump: true}, http.StatusCreated), &api.MergeQueueEntry{})
	assert.True(t, added.Jump)
	mergeQueueAPI(t, "DELETE", pullURL+"/merge-queue", nil, http.StatusNoContent)
	mergeQueueAPI(t, "POST", pullURL+"/merge", forms.MergePullRequestForm{Do: "merge", HeadCommitID: headSHA}, http.StatusCreated)
	flushMergeQueue(t)

	entry := unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{PullID: pr.ID})
	seedMergeQueueGroup(t, repo, entry, pr.Index, baseSHA, headSHA)
	require.NoError(t, pull_model.UpdateMergeQueueEntry(ctx, entry))
	setMergeQueueStatus(t, entry.GroupHeadSHA, commitstatus.CommitStatusSuccess)
	flushMergeQueue(t)

	merged, err := git.GetBranchCommitID(ctx, repo, pr.BaseBranch)
	require.NoError(t, err)
	assert.Equal(t, entry.GroupHeadSHA, merged)
	unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID, HasMerged: true})
	unittest.AssertNotExistsBean(t, &pull_model.MergeQueueEntry{PullID: pr.ID})
}

func TestPullMergeQueueInvalidationAndBranchProtection(t *testing.T) {
	defer test.MockVariableValue(&setting.Repository.PullRequest.MergeQueueBranchPrefix, "custom-queue/")()
	defer tests.PrintCurrentTest(t)()
	ctx := t.Context()
	repo := prepareMergeQueueRepo(t)
	baseSHA, err := git.GetBranchCommitID(ctx, repo, "master")
	require.NoError(t, err)
	protectMergeQueue(t)

	predecessor := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: 5})
	predecessor.BaseBranch = "master"
	require.NoError(t, predecessor.UpdateCols(ctx, "base_branch"))
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: 3})
	first := insertMergeQueueGroup(t, repo, predecessor, baseSHA)
	next := insertMergeQueueGroup(t, repo, pr, first.GroupHeadSHA)

	_, err = repo_module.SyncRepoBranches(ctx, repo.ID, repo.OwnerID)
	require.NoError(t, err)
	branch := git.RefName(next.GroupRef).BranchName()
	unittest.AssertNotExistsBean(t, &git_model.Branch{RepoID: repo.ID, Name: branch})
	mergeQueueAPI(t, "PATCH", "/branches/feature/1", api.RenameBranchRepoOption{Name: branch}, http.StatusForbidden)
	mergeQueueAPI(t, "DELETE", "/branches/"+branch, nil, http.StatusForbidden)
	preReceive := func(ref git.RefName, oldSHA, newSHA string, pullID int64, status int) *httptest.ResponseRecorder {
		t.Helper()
		return MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/internal/hook/pre-receive/user2/repo1", private.HookOptions{
			UserID: repo.OwnerID, PullRequestID: pullID, RefFullNames: []git.RefName{ref}, OldCommitIDs: []string{oldSHA}, NewCommitIDs: []string{newSHA},
		}).SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken), status)
	}
	assert.Contains(t, preReceive(git.RefName(next.GroupRef), next.GroupHeadSHA, git.ObjectFormatFromName(repo.ObjectFormatName).EmptyObjectID().String(), 0, http.StatusForbidden).Body.String(), "reserved for the merge queue")
	assert.Contains(t, preReceive(git.RefNameFromBranch(pr.HeadBranch), next.HeadSHA, baseSHA, 0, http.StatusForbidden).Body.String(), "dequeue the associated pull request")
	mergeQueueAPI(t, "PATCH", "/pulls/3", api.EditPullRequestOption{Base: "feature/1"}, http.StatusUnprocessableEntity)

	first.State = pull_model.MergeQueueStateMergeable
	require.NoError(t, pull_model.UpdateMergeQueueEntry(ctx, first))
	preReceive(git.RefNameFromBranch(first.BaseBranch), baseSHA, first.GroupHeadSHA, predecessor.ID, http.StatusOK)
	setMergeQueueStatus(t, first.GroupHeadSHA, commitstatus.CommitStatusFailure)
	flushMergeQueue(t)
	unittest.AssertNotExistsBean(t, &pull_model.MergeQueueEntry{ID: first.ID})
	assert.False(t, git.IsReferenceExist(ctx, repo, next.GroupRef))
	unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{ID: next.ID, State: pull_model.MergeQueueStateQueued})

	seedMergeQueueGroup(t, repo, next, pr.Index, baseSHA, next.HeadSHA)
	require.NoError(t, pull_model.UpdateMergeQueueEntry(ctx, next))
	mergeQueueAPI(t, "PATCH", "/branches/master", api.RenameBranchRepoOption{Name: "renamed"}, http.StatusNoContent)
	flushMergeQueue(t)
	entries := DecodeJSON(t, mergeQueueAPI(t, "GET", "/merge-queue/renamed", nil, http.StatusOK), []api.MergeQueueEntry{})
	require.Len(t, entries, 1)
	assert.Equal(t, "queued", entries[0].State)
	assert.Empty(t, entries[0].MergeGroupHeadRef)
	assert.False(t, git.IsReferenceExist(ctx, repo, next.GroupRef))

	require.NoError(t, git.DeleteBranch(ctx, repo, "renamed", true))
	require.NoError(t, mergequeue.CheckAllQueues(ctx))
	flushMergeQueue(t)
	unittest.AssertNotExistsBean(t, &pull_model.MergeQueueEntry{ID: next.ID})
}
