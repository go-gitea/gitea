// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
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
	"gitea.dev/modules/queue"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/forms"
	"gitea.dev/services/mergequeue"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mergeQueueAPI(t *testing.T, method, path string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	return MakeRequest(t, NewRequestWithJSON(t, method, "/api/v1/repos/user2/repo1"+path, body).AddBasicAuth("user2"), status)
}

func setMergeQueueStatus(t *testing.T, sha string, state commitstatus.CommitStatusState) {
	t.Helper()
	mergeQueueAPI(t, "POST", "/statuses/"+sha, api.CreateStatusOption{State: state, Context: "ci"}, http.StatusCreated)
}

func watchQueueWebhook[T any](t *testing.T, session *TestSession, event string, accept func(*T) bool, branchFilter ...string) func(*testing.T) *T {
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
	testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), event, branchFilter...)
	return func(t *testing.T) *T {
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

func TestPullMergeQueue(t *testing.T) {
	defer test.MockVariableValue(&setting.Repository.PullRequest.MergeQueueBranchPrefix, "custom-queue/")()
	onGiteaRun(t, func(t *testing.T, _giteaURL *url.URL) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		user := repo.MustOwner(t.Context())
		gitRepo, err := git.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		session := loginUser(t, user.Name)
		nextRemoval := watchQueueWebhook(t, session, "pull_request", func(payload *api.PullRequestPayload) bool { return payload.Action == api.HookIssueDequeued })
		assertRemoved := func(t *testing.T, index int64, reason string) {
			t.Helper()
			event := nextRemoval(t)
			assert.Equal(t, index, event.Index)
			assert.Equal(t, reason, event.Reason)
		}
		predecessor := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: 5})
		headSHA, err := gitRepo.GetBranchCommitID(t.Context(), predecessor.HeadBranch)
		require.NoError(t, err)
		prepareGroup := func(t *testing.T, branch string) pull_model.MergeQueueEntry {
			t.Helper()
			ctx := t.Context()
			predecessor.BaseBranch = branch
			require.NoError(t, predecessor.UpdateCols(ctx, "base_branch"))
			mergeQueueAPI(t, "POST", "/branch_protections", api.CreateBranchProtectionOption{
				RuleName: branch, RequireMergeQueue: true, EnableStatusCheck: true, StatusCheckContexts: []string{"ci"},
				MergeQueue: &api.BranchProtectionMergeQueueOption{MergeMethod: new("squash")},
			}, http.StatusCreated)
			baseSHA, err := gitRepo.GetBranchCommitID(ctx, branch)
			require.NoError(t, err)
			return pull_model.MergeQueueEntry{
				RepoID: repo.ID, PullID: predecessor.ID, EnqueuerID: user.ID, BaseBranch: branch, HeadSHA: headSHA,
				BaseSHA: baseSHA, GroupHeadSHA: headSHA, GroupRef: pull_model.MergeQueueGroupRef(branch, predecessor.Index, baseSHA),
				State: pull_model.MergeQueueStateAwaitingChecks, ChecksStartedUnix: timeutil.TimeStampNow(),
			}
		}
		insertGroup := func(t *testing.T, entry pull_model.MergeQueueEntry) {
			t.Helper()
			require.NoError(t, git.UpdateRef(t.Context(), repo, entry.GroupRef, entry.GroupHeadSHA))
			require.NoError(t, db.Insert(t.Context(), &entry))
		}

		t.Run("RebuildAndMerge", func(t *testing.T) {
			ctx := t.Context()
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: 3})
			nextPush := watchQueueWebhook[api.PushPayload](t, session, "push", nil, "custom-queue/*/pr-3-*")
			first := prepareGroup(t, pr.BaseBranch)
			insertGroup(t, first)
			headSHA, err := gitRepo.GetBranchCommitID(ctx, pr.HeadBranch)
			require.NoError(t, err)
			setMergeQueueStatus(t, headSHA, commitstatus.CommitStatusSuccess)
			const pullURL = "/pulls/3"
			mergeQueueAPI(t, "POST", pullURL+"/merge", forms.MergePullRequestForm{Do: "merge", HeadCommitID: pr.MergeBase}, http.StatusConflict)
			mergeQueueAPI(t, "POST", pullURL+"/merge-queue", api.AddToMergeQueueOption{}, http.StatusCreated)
			built := nextPush(t)
			const refPrefix = "refs/heads/custom-queue/master/pr-3-"
			assert.Equal(t, refPrefix+first.GroupHeadSHA, built.Ref)
			setMergeQueueStatus(t, first.GroupHeadSHA, commitstatus.CommitStatusFailure)
			assertRemoved(t, predecessor.Index, "CI_FAILURE")
			rebuilt := nextPush(t)
			assert.Equal(t, refPrefix+first.BaseSHA, rebuilt.Ref)
			assert.False(t, git.IsReferenceExist(ctx, repo, built.Ref))
			require.NoError(t, queue.GetManager().FlushAll(ctx, 5*time.Second))

			mergeQueueAPI(t, "PATCH", "", api.EditRepoOption{SquashMergeCommitMessage: new(string(repo_model.SquashMergeMessageCommitMessages))}, http.StatusOK)
			mergeQueueAPI(t, "PATCH", "/branches/master", api.RenameBranchRepoOption{Name: "renamed"}, http.StatusNoContent)
			require.NoError(t, mergequeue.CheckAllQueues(ctx))
			renamed := nextPush(t)
			assert.Contains(t, renamed.Ref, "custom-queue/renamed/")
			assert.False(t, git.IsReferenceExist(ctx, repo, rebuilt.Ref))
			setMergeQueueStatus(t, renamed.After, commitstatus.CommitStatusSuccess)
			assertRemoved(t, pr.Index, "MERGE")
			merged, err := gitRepo.GetBranchCommit(ctx, "renamed")
			require.NoError(t, err)
			assert.Equal(t, renamed.After, merged.ID.String())
			assert.Contains(t, merged.CommitMessage.MessageRaw, "* a change\n\n* make pull5 outdated")
			assert.Contains(t, merged.MessageTrailer()["co-authored-by"], "6543 <6543@obermui.de>")
			testAPICreateBranch(t, session, user.Name, repo.Name, "renamed", git.RefName(renamed.Ref).BranchName(), http.StatusCreated)
		})

		t.Run("ProtectedBranchesAndRemoval", func(t *testing.T) {
			ctx := t.Context()
			setMergeQueueStatus(t, headSHA, commitstatus.CommitStatusPending)
			entry := prepareGroup(t, "develop")
			insertGroup(t, entry)
			branch := git.RefName(entry.GroupRef).BranchName()
			_, _, err := repo_module.SyncRepoBranchesWithRepo(ctx, repo, gitRepo, user.ID)
			require.NoError(t, err)
			unittest.AssertNotExistsBean(t, &git_model.Branch{RepoID: repo.ID, Name: branch})
			assert.Contains(t, mergeQueueAPI(t, "PATCH", "/branches/develop", api.RenameBranchRepoOption{Name: branch}, http.StatusForbidden).Body.String(), "branch protection rules")
			testAPIDeleteBranch(t, branch, http.StatusForbidden)
			push := func(refspec string) error {
				return git.PushManaged(ctx, repo, repo, git.PushOptions{Branch: refspec, Env: repo_module.PushingEnvironment(user, repo)})
			}
			require.ErrorContains(t, push(":"+entry.GroupRef), "reserved for the merge queue")
			require.ErrorContains(t, push("+develop:"+git.BranchPrefix+predecessor.HeadBranch), "To modify this branch, dequeue the associated pull request.")
			assert.Contains(t, mergeQueueAPI(t, "PATCH", "/pulls/5", api.EditPullRequestOption{Base: "feature/1"}, http.StatusUnprocessableEntity).Body.String(), "Cannot change the base branch")

			require.NoError(t, predecessor.LoadIssue(ctx))
			assert.Contains(t, session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/queue/develop"), http.StatusOK).Body.String(), predecessor.Issue.Title)
			session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/queue/feature/1"), http.StatusNotFound)
			session.MakeRequest(t, NewRequest(t, "POST", "/user2/repo1/pulls/5/merge_queue/jump"), http.StatusOK)
			assert.NotZero(t, unittest.AssertExistsAndLoadBean(t, &pull_model.MergeQueueEntry{PullID: entry.PullID}).JumpUnix)
			session.MakeRequest(t, NewRequest(t, "POST", "/user2/repo1/queue/clear/develop"), http.StatusOK)
			assertRemoved(t, predecessor.Index, "QUEUE_CLEARED")

			insertGroup(t, entry)
			mergeQueueAPI(t, "PATCH", "/pulls/5", api.EditPullRequestOption{Title: "WIP: queued change"}, http.StatusCreated)
			setMergeQueueStatus(t, entry.GroupHeadSHA, commitstatus.CommitStatusSuccess)
			assertRemoved(t, predecessor.Index, "BRANCH_PROTECTIONS")

			require.NoError(t, git.DeleteBranch(ctx, repo, "develop", true))
			insertGroup(t, entry)
			require.NoError(t, mergequeue.CheckAllQueues(ctx))
			assertRemoved(t, predecessor.Index, "QUEUE_CLEARED")
			testAPICreateBranch(t, session, user.Name, repo.Name, "", "develop", http.StatusForbidden)
		})
	})
}
