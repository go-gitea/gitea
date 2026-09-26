// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/services/gitdiff"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIPullReviewViewedFiles(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Git.MaxGitDiffFiles, 1)()
	defer test.MockVariableValue(&setting.Git.MaxGitDiffLines, 1)()

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	otherPR := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pr.BaseRepoID})
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	baseFiles := []git.FastImportFile{
		{Path: "modified.txt", Content: "before\n"},
		{Path: "deleted.txt", Content: "deleted file\n"},
		{Path: "old-name.txt", Content: "renamed file\n"},
		{Path: "unchanged.txt", Content: "unchanged\n"},
	}
	require.NoError(t, git.ForceFastImport(t.Context(), repo.CodeStorageRepo(), []git.FastImportCommit{
		{Ref: "refs/heads/review-merge-base", Files: baseFiles},
		{Ref: "refs/heads/review-base", Files: append(baseFiles, git.FastImportFile{Path: "base-only.txt", Content: "not part of the pull request\n"})},
		{Ref: "refs/heads/review-head", Files: []git.FastImportFile{
			{Path: "modified.txt", Content: "after\n"},
			{Path: "dir/new name.txt", Content: "renamed file\n"},
			{Path: "added.txt", Content: "added file\n"},
			{Path: "unchanged.txt", Content: "unchanged\n"},
		}},
	}))
	mergeBase, err := gitRepo.GetRefCommitID(t.Context(), "refs/heads/review-merge-base")
	require.NoError(t, err)
	// Fast-import creates roots; attach both tips to the same ancestor.
	makeChild := func(ref string) string {
		t.Helper()
		tree, err := gitRepo.GetTree(t.Context(), ref)
		require.NoError(t, err)
		identity := &git.Signature{Name: "Gitea", Email: "gitea@example.com"}
		commitID, err := gitRepo.CommitTree(t.Context(), identity, identity, tree, git.CommitTreeOpts{
			Parents: []string{mergeBase}, Message: ref, NoGPGSign: true,
		})
		require.NoError(t, err)
		require.NoError(t, git.UpdateRef(t.Context(), repo, ref, commitID.String()))
		return commitID.String()
	}
	makeChild("refs/heads/review-base")
	headCommitID := makeChild("refs/heads/review-head")
	for _, pull := range []*issues_model.PullRequest{pr, otherPR} {
		require.NoError(t, git.UpdateRef(t.Context(), repo, pull.GetGitHeadRefName(), headCommitID))
		pull.BaseBranch = "review-base"
		pull.MergeBase = mergeBase
		_, err := db.GetEngine(t.Context()).ID(pull.ID).Cols("base_branch", "merge_base").Update(pull)
		require.NoError(t, err)
	}

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
	otherToken := getUserToken(t, "user5", auth_model.AccessTokenScopeWriteRepository)
	reader := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	permission, err := access_model.GetIndividualUserRepoPermission(t.Context(), repo, reader)
	require.NoError(t, err)
	require.True(t, permission.CanRead(unit.TypePullRequests))
	require.False(t, permission.CanWrite(unit.TypePullRequests))

	update := func(t *testing.T, index int64, operation string, opts api.MarkPullReviewFileOptions, auth string, status int) {
		t.Helper()
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/files/%s", repo.OwnerName, repo.Name, index, operation), opts).AddTokenAuth(auth)
		resp := MakeRequest(t, req, status)
		if status == http.StatusNoContent {
			assert.Empty(t, resp.Body.String())
		}
	}
	assertState := func(t *testing.T, userID, pullID int64, want map[string]pull_model.ViewedState) {
		t.Helper()
		state, exists, err := pull_model.GetReviewState(t.Context(), userID, pullID, headCommitID)
		require.NoError(t, err)
		require.True(t, exists)
		assert.Equal(t, want, state.UpdatedFiles)
		unittest.AssertCount(t, &pull_model.ReviewState{UserID: userID, PullID: pullID}, 1)
	}

	for range 2 {
		update(t, pr.Index, "unviewed", api.MarkPullReviewFileOptions{Path: "modified.txt"}, token, http.StatusNoContent)
	}
	assertState(t, 2, pr.ID, map[string]pull_model.ViewedState{"modified.txt": pull_model.Unviewed})
	want := map[string]pull_model.ViewedState{}
	for _, path := range []string{"modified.txt", "deleted.txt", "dir/new name.txt", "added.txt"} {
		for range 2 {
			update(t, pr.Index, "viewed?limit=1&page=999", api.MarkPullReviewFileOptions{Path: path}, token, http.StatusNoContent)
		}
		want[path] = pull_model.Viewed
	}
	assertState(t, 2, pr.ID, want)
	for range 2 {
		update(t, pr.Index, "unviewed", api.MarkPullReviewFileOptions{Path: "modified.txt", CommitID: headCommitID}, token, http.StatusNoContent)
	}
	want["modified.txt"] = pull_model.Unviewed
	assertState(t, 2, pr.ID, want)

	update(t, pr.Index, "viewed", api.MarkPullReviewFileOptions{Path: "modified.txt", CommitID: headCommitID}, otherToken, http.StatusNoContent)
	update(t, otherPR.Index, "viewed", api.MarkPullReviewFileOptions{Path: "modified.txt"}, token, http.StatusNoContent)
	assertState(t, 5, pr.ID, map[string]pull_model.ViewedState{"modified.txt": pull_model.Viewed})
	assertState(t, 2, otherPR.ID, map[string]pull_model.ViewedState{"modified.txt": pull_model.Viewed})
	assertState(t, 2, pr.ID, want)

	t.Run("InvalidPaths", func(t *testing.T) {
		for _, path := range []string{"", ".", "../modified.txt", "/modified.txt", "./modified.txt", "dir/../modified.txt", "dir//new name.txt", "modified.txt\x00", "missing.txt", "unchanged.txt", "base-only.txt", "dir", "old-name.txt"} {
			t.Run(fmt.Sprintf("%q", path), func(t *testing.T) {
				for _, operation := range []string{"viewed", "unviewed"} {
					update(t, pr.Index, operation, api.MarkPullReviewFileOptions{Path: path}, token, http.StatusUnprocessableEntity)
				}
			})
		}
	})
	t.Run("Errors", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			index    int64
			commitID string
			token    string
			status   int
		}{
			{name: "MissingPR", index: 999999, token: token, status: http.StatusNotFound},
			{name: "IssueNotPR", index: 1, token: token, status: http.StatusNotFound},
			{name: "StaleCommit", index: pr.Index, commitID: mergeBase, token: token, status: http.StatusConflict},
			{name: "AbbreviatedCommit", index: pr.Index, commitID: headCommitID[:12], token: token, status: http.StatusConflict},
			{name: "CommitRef", index: pr.Index, commitID: "refs/heads/review-head", token: token, status: http.StatusConflict},
			{name: "Unauthenticated", index: pr.Index, status: http.StatusUnauthorized},
			{name: "ReadOnlyScope", index: pr.Index, token: readToken, status: http.StatusForbidden},
		} {
			t.Run(tc.name, func(t *testing.T) {
				for _, operation := range []string{"viewed", "unviewed"} {
					update(t, tc.index, operation, api.MarkPullReviewFileOptions{Path: "added.txt", CommitID: tc.commitID}, tc.token, tc.status)
				}
			})
		}
	})
	t.Run("MissingRepo", func(t *testing.T) {
		for _, operation := range []string{"viewed", "unviewed"} {
			req := NewRequestWithJSON(t, http.MethodPost, "/api/v1/repos/user2/missing-review-repo/pulls/3/files/"+operation, api.MarkPullReviewFileOptions{Path: "added.txt"}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		}
	})
	t.Run("Archived", func(t *testing.T) {
		repo.IsArchived = true
		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), repo, "is_archived"))
		for _, operation := range []string{"viewed", "unviewed"} {
			update(t, pr.Index, operation, api.MarkPullReviewFileOptions{Path: "added.txt"}, token, http.StatusNotFound)
		}
	})
	assertState(t, 2, pr.ID, want)
	assertState(t, 5, pr.ID, map[string]pull_model.ViewedState{"modified.txt": pull_model.Viewed})
	assertState(t, 2, otherPR.ID, map[string]pull_model.ViewedState{"modified.txt": pull_model.Viewed})
}

func TestAPIPullReviewViewedFilesNewCommit(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 7})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pr.BaseRepoID})
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	firstCommitID, err := gitRepo.GetRefCommitID(t.Context(), pr.GetGitHeadRefName())
	require.NoError(t, err)
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	url := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/files/", repo.OwnerName, repo.Name, pr.Index)
	for _, path := range []string{"test1.txt", "test2.txt"} {
		req := NewRequestWithJSON(t, http.MethodPost, url+"viewed", api.MarkPullReviewFileOptions{Path: path}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	}
	firstReview, exists, err := pull_model.GetReviewState(t.Context(), 2, pr.ID, firstCommitID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, map[string]pull_model.ViewedState{"test1.txt": pull_model.Viewed, "test2.txt": pull_model.Viewed}, firstReview.UpdatedFiles)

	stdin := fmt.Sprintf(`commit %s
committer Gitea <gitea@example.com> 1772749114 +0000
data 7
change
from %s
M 100644 inline test2.txt
data 7
change
`, pr.GetGitHeadRefName(), firstCommitID)
	require.NoError(t, gitcmd.NewCommand("fast-import").WithRepo(repo).WithStdinBytes([]byte(stdin)).Run(t.Context()))
	headCommitID, err := gitRepo.GetRefCommitID(t.Context(), pr.GetGitHeadRefName())
	require.NoError(t, err)
	require.NotEqual(t, firstCommitID, headCommitID)

	for _, operation := range []string{"viewed", "unviewed"} {
		req := NewRequestWithJSON(t, http.MethodPost, url+operation, api.MarkPullReviewFileOptions{Path: "test3.txt", CommitID: firstCommitID}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)
		assert.Equal(t, firstReview, unittest.AssertExistsAndLoadBean(t, &pull_model.ReviewState{ID: firstReview.ID}))
		unittest.AssertCount(t, &pull_model.ReviewState{UserID: 2, PullID: pr.ID}, 1)
	}

	req := NewRequestWithJSON(t, http.MethodPost, url+"viewed", api.MarkPullReviewFileOptions{Path: "test3.txt"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	review, exists, err := pull_model.GetReviewState(t.Context(), 2, pr.ID, headCommitID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, map[string]pull_model.ViewedState{
		"test1.txt": pull_model.Viewed,
		"test2.txt": pull_model.HasChanged,
		"test3.txt": pull_model.Viewed,
	}, review.UpdatedFiles)

	diff := &gitdiff.Diff{Files: []*gitdiff.DiffFile{{Name: "test1.txt"}, {Name: "test2.txt"}, {Name: "test3.txt"}}}
	_, err = gitdiff.SyncUserSpecificDiff(t.Context(), 2, pr, gitRepo, diff, &gitdiff.DiffOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{AfterCommitID: headCommitID},
	})
	require.NoError(t, err)
	assert.True(t, diff.Files[0].IsViewed)
	assert.False(t, diff.Files[0].HasChangedSinceLastReview)
	assert.False(t, diff.Files[1].IsViewed)
	assert.True(t, diff.Files[1].HasChangedSinceLastReview)
}
