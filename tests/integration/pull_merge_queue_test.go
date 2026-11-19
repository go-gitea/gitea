// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	repo_module "gitea.dev/modules/repository"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/forms"
	"gitea.dev/services/mergequeue"
	repo_service "gitea.dev/services/repository"
	commitstatus_service "gitea.dev/services/repository/commitstatus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullMergeQueue(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		setStatus := func(sha string, state commitstatus.CommitStatusState) {
			require.NoError(t, commitstatus_service.CreateCommitStatus(t.Context(), repo, user2, sha, &git_model.CommitStatus{State: state, Context: "ci"}))
		}
		createPullTo := func(base, branch string) *issues_model.PullRequest {
			testCreateFile(t, session, "user2", "repo1", base, branch, branch+".txt", branch)
			testPullCreate(t, session, "user2", "repo1", true, base, branch, branch)
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadBranch: branch})
			headSHA, err := git.GetBranchCommitID(t.Context(), repo, branch)
			require.NoError(t, err)
			setStatus(headSHA, commitstatus.CommitStatusSuccess)
			return pr
		}
		createPull := func(branch string) *issues_model.PullRequest { return createPullTo("master", branch) }
		waitForEntry := func(pr *issues_model.PullRequest, cond func(*pull_model.MergeQueueEntry) bool) *pull_model.MergeQueueEntry {
			var entry *pull_model.MergeQueueEntry
			require.Eventually(t, func() bool {
				entry, _ = pull_model.GetMergeQueueEntryByPullID(t.Context(), pr.ID)
				return entry != nil && cond(entry)
			}, 5*time.Second, 50*time.Millisecond)
			return entry
		}
		waitForDequeue := func(pr *issues_model.PullRequest) {
			require.Eventually(t, func() bool {
				e, _ := pull_model.GetMergeQueueEntryByPullID(t.Context(), pr.ID)
				return e == nil
			}, 5*time.Second, 50*time.Millisecond)
		}
		enqueue := func(pr *issues_model.PullRequest) {
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/user2/repo1/pulls/%d/merge-queue", pr.Index), &api.AddToMergeQueueOption{}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		protect := func(branch string) {
			req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
				"rule_name":             branch,
				"enable_push":           "true",
				"enable_status_check":   "true",
				"status_check_contexts": "ci",
				"require_merge_queue":   "true",

				"merge_queue_merge_method":                      "merge",
				"merge_queue_max_entries_to_build":              "5",
				"merge_queue_min_entries_to_merge":              "1",
				"merge_queue_max_entries_to_merge":              "5",
				"merge_queue_min_entries_to_merge_wait_minutes": "0",
				"merge_queue_grouping_strategy":                 "ALLGREEN",
				"merge_queue_check_response_timeout_minutes":    "60",
			})
			session.MakeRequest(t, req, http.StatusOK)
		}
		protect("master")

		pr1, pr2, pr3 := createPull("queue-1"), createPull("queue-2"), createPull("queue-3")

		// like GitHub, CI can build on pushes of the queue branches
		var pushedRefs sync.Map
		provider := newMockWebhookProvider(func(r *http.Request) {
			var payload api.PushPayload
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			pushedRefs.Store(payload.Ref, payload.After)
		}, http.StatusOK)
		defer provider.Close()
		testAPICreateWebhookForRepo(t, session, "user2", "repo1", provider.URL(), "push", "gh-readonly-queue/**")

		enqueue(pr1)
		enqueue(pr2)
		enqueue(pr3)

		// each queue commit is built on the one before it
		built := func(e *pull_model.MergeQueueEntry) bool { return e.IsBuilt() }
		entry1, entry2, entry3 := waitForEntry(pr1, built), waitForEntry(pr2, built), waitForEntry(pr3, built)
		assert.Equal(t, entry1.GroupHeadSHA, entry2.BaseSHA)
		assert.Equal(t, entry2.GroupHeadSHA, entry3.BaseSHA)
		groupSHA, err := git.GetFullCommitID(t.Context(), repo, entry1.GroupRef)
		require.NoError(t, err)
		assert.Equal(t, entry1.GroupHeadSHA, groupSHA)
		require.Eventually(t, func() bool {
			after, ok := pushedRefs.Load(entry1.GroupRef)
			return ok && after == entry1.GroupHeadSHA
		}, 5*time.Second, 50*time.Millisecond)

		// queue branches are neither listed nor writable by users
		queueBranch := git.RefName(entry1.GroupRef).BranchName()
		_, err = repo_module.SyncRepoBranches(t.Context(), repo.ID, user2.ID)
		require.NoError(t, err)
		unittest.AssertNotExistsBean(t, &git_model.Branch{RepoID: repo.ID, Name: queueBranch})
		req := NewRequest(t, "DELETE", "/api/v1/repos/user2/repo1/branches/"+queueBranch).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)

		resp := session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/pulls/%d/merge_box", pr1.Index)), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "/user2/repo1/queue/master")
		resp = session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/queue/master"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), fmt.Sprintf("#%d", pr3.Index))
		assert.Contains(t, resp.Body.String(), "Waiting for ci")

		req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/merge-queue/master").AddTokenAuth(token)
		var listed []*api.MergeQueueEntry
		DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &listed)
		require.Len(t, listed, 3)
		assert.Equal(t, pr1.Index, listed[0].PullRequestIndex)
		assert.Equal(t, "awaiting_checks", listed[0].State)

		// a failing entry leaves the queue and the entry after it is rebuilt without it
		setStatus(entry2.GroupHeadSHA, commitstatus.CommitStatusFailure)
		waitForDequeue(pr2)
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr2.IssueID, Type: issues_model.CommentTypePRRemovedFromMergeQueue, Content: string(issues_model.MergeQueueRemovedChecksFailed)})
		entry3 = waitForEntry(pr3, func(e *pull_model.MergeQueueEntry) bool { return e.IsBuilt() && e.BaseSHA == entry1.GroupHeadSHA })

		// passing queue commits fast-forward the base branch
		setStatus(entry1.GroupHeadSHA, commitstatus.CommitStatusSuccess)
		setStatus(entry3.GroupHeadSHA, commitstatus.CommitStatusSuccess)
		require.Eventually(t, func() bool {
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr3.ID})
			return pr.HasMerged
		}, 5*time.Second, 50*time.Millisecond)
		pr1 = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr1.ID})
		pr3 = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr3.ID})
		assert.True(t, pr1.HasMerged)
		assert.Equal(t, entry1.GroupHeadSHA, pr1.MergedCommitID)
		assert.Equal(t, entry3.GroupHeadSHA, pr3.MergedCommitID)
		masterSHA, err := git.GetBranchCommitID(t.Context(), repo, "master")
		require.NoError(t, err)
		assert.Equal(t, entry3.GroupHeadSHA, masterSHA)
		require.Eventually(t, func() bool {
			has, _ := pull_model.HasMergeQueueEntries(t.Context(), repo.ID, "master")
			return !has
		}, 5*time.Second, 50*time.Millisecond)
		assert.False(t, git.IsReferenceExist(t.Context(), repo, entry1.GroupRef))

		// pull requests can't skip the queue
		pr4 := createPull("queue-4")
		// like a direct merge, a stale head isn't queued
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/user2/repo1/pulls/%d/merge", pr4.Index), &forms.MergePullRequestForm{Do: "merge", HeadCommitID: entry1.HeadSHA}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/user2/repo1/pulls/%d/merge", pr4.Index), map[string]string{"do": "merge"})
		session.MakeRequest(t, req, http.StatusOK)
		entry4 := waitForEntry(pr4, built)

		// a pull request that became a draft while queued isn't merged
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/user2/repo1/pulls/%d", pr4.Index), &api.EditPullRequestOption{Title: "WIP: queue-4"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		setStatus(entry4.GroupHeadSHA, commitstatus.CommitStatusSuccess)
		waitForDequeue(pr4)
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr4.IssueID, Type: issues_model.CommentTypePRRemovedFromMergeQueue, Content: string(issues_model.MergeQueueRemovedNotMergeable)})
		pr4 = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr4.ID})
		assert.False(t, pr4.HasMerged)

		// the queue follows a renamed branch and keeps its queue commits
		testAPICreateBranch(t, session, "user2", "repo1", "master", "release", http.StatusCreated)
		protect("release")
		pr5, pr6 := createPullTo("release", "queue-5"), createPullTo("release", "queue-6")
		enqueue(pr5)
		enqueue(pr6)
		entry5 := waitForEntry(pr5, built)
		waitForEntry(pr6, built)
		_, err = repo_service.RenameBranch(t.Context(), repo, user2, "release", "release2")
		require.NoError(t, err)
		setStatus(entry5.GroupHeadSHA, commitstatus.CommitStatusSuccess)
		require.Eventually(t, func() bool {
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr5.ID})
			return pr.HasMerged && pr.MergedCommitID == entry5.GroupHeadSHA
		}, 5*time.Second, 50*time.Millisecond)

		// a deleted branch empties its queue
		waitForEntry(pr6, built)
		require.NoError(t, git.DeleteBranch(t.Context(), repo, "release2", true))
		require.NoError(t, mergequeue.CheckAllQueues(t.Context()))
		waitForDequeue(pr6)
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pr6.IssueID, Type: issues_model.CommentTypePRRemovedFromMergeQueue, Content: string(issues_model.MergeQueueRemovedBaseDeleted)})
	})
}
