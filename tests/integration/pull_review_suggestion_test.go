// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
	pull_service "gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPullReviewSuggestions(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := t.Context()
		pr, err := issues_model.GetPullRequestByIndex(ctx, 1, 3)
		require.NoError(t, err)
		require.NoError(t, pr.Issue.LoadRepo(ctx))
		repo := pr.Issue.Repo
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		apiCtx := NewAPITestContext(t, owner.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)

		const treePath = "README.md"
		gitRepo, err := git.OpenRepository(ctx, repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		initialHead, err := gitRepo.GetBranchCommitID(ctx, pr.HeadBranch)
		require.NoError(t, err)

		comment := func(doer *user_model.User, line int64, content string, pending bool, reviewID int64) *issues_model.Comment {
			t.Helper()
			item, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, pr.Issue, pending, initialHead,
				issues_model.CreateCommentOptions{LineNum: line, Content: content, TreePath: treePath, ReviewID: reviewID})
			require.NoError(t, err)
			return item
		}
		first := comment(reviewer, 1, "~~~suggestion\n# Repository\n~~~", false, 0)
		root := comment(owner, 3, "Please update this line.", false, 0)
		second := comment(reviewer, 3, "```suggestion\nA new description\n```", false, root.ReviewID)
		third := comment(owner, 4, "```suggestion\nAdditional notes:\n```", true, 0)
		fourth := comment(reviewer, 5, "```suggestion\nFirst branch change\n```", false, 0)
		fifth := comment(reviewer, 6, "```suggestion\nSecond branch change\n```", false, 0)

		webURL := fmt.Sprintf("%s/pulls/%d", repo.Link(), pr.Index)
		apiApply := func(status int, options api.ApplyPullReviewSuggestionsOptions) *httptest.ResponseRecorder {
			t.Helper()
			return MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, "/api/v1/repos"+webURL+"/suggestions/apply", options).
				AddTokenAuth(apiCtx.Token), status)
		}
		assert.Contains(t, apiApply(http.StatusUnprocessableEntity, api.ApplyPullReviewSuggestionsOptions{CommentIDs: []int64{third.ID}}).Body.String(), "pending reviews")
		require.NoError(t, issues_model.MarkConversation(ctx, root, owner, true))
		assert.Contains(t, apiApply(http.StatusUnprocessableEntity, api.ApplyPullReviewSuggestionsOptions{CommentIDs: []int64{second.ID}}).Body.String(), "resolved conversations")
		require.NoError(t, issues_model.MarkConversation(ctx, unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: root.ID}), owner, false))
		_, _, err = pull_service.SubmitReview(ctx, owner, gitRepo, pr.Issue, issues_model.ReviewTypeComment, "", initialHead, nil)
		require.NoError(t, err)

		require.NoError(t, pull_service.LoadCodeSuggestionStates(ctx, reviewer, pr, []*issues_model.Comment{first}))
		assert.Nil(t, first.CodeSuggestion)

		response := apiCtx.Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, webURL+"/suggestions/apply", map[string]string{
			"origin":         "diff",
			"comment_ids":    fmt.Sprintf("%d:%d,%d:%d", first.ID, first.ContentVersion, fourth.ID, fourth.ContentVersion),
			"head_commit_id": initialHead,
		}), http.StatusOK)
		assert.Equal(t, webURL+"/files", DecodeJSON(t, response, map[string]string{})["redirect"])

		checkCommit := func(parent, content, title, body string) string {
			t.Helper()
			commit, err := gitRepo.GetBranchCommit(ctx, pr.HeadBranch)
			require.NoError(t, err)
			parentID, err := commit.ParentID(0)
			require.NoError(t, err)
			assert.Equal(t, parent, parentID.String())
			assert.Equal(t, owner.GitName(), commit.Author.Name)
			assert.Equal(t, owner.GetEmail(), commit.Author.Email)
			assert.Equal(t, title, commit.MessageTitle())
			assert.Equal(t, body, commit.MessageBody())
			actual, err := commit.GetFileContent(ctx, gitRepo, treePath, 1024)
			require.NoError(t, err)
			assert.Equal(t, content, actual)
			return commit.ID.String()
		}

		trailer := git.CoAuthoredByTrailer + ": " + reviewer.NewGitSig().String()
		webHead := checkCommit(initialHead, "# Repository\n\nDescription for repo1\n\nFirst branch change\nand a second one\n", "Apply suggestions from code review", trailer)
		assert.Contains(t, apiApply(http.StatusConflict, api.ApplyPullReviewSuggestionsOptions{CommentIDs: []int64{second.ID}, HeadCommitID: initialHead}).Body.String(), "branch changed")
		batch := DecodeJSON(t, apiApply(http.StatusCreated, api.ApplyPullReviewSuggestionsOptions{
			CommentIDs: []int64{second.ID, third.ID, second.ID},
			Message:    "Apply the review edits\n\nFollow up.",
		}), api.ApplyPullReviewSuggestionsResponse{})
		assert.Equal(t, batch.SHA, checkCommit(webHead, "# Repository\n\nA new description\nAdditional notes:\nFirst branch change\nand a second one\n", "Apply the review edits", "Follow up.\n\n"+trailer))
		single := DecodeJSON(t, apiApply(http.StatusCreated, api.ApplyPullReviewSuggestionsOptions{CommentIDs: []int64{fifth.ID}}), api.ApplyPullReviewSuggestionsResponse{})
		assert.Equal(t, single.SHA, checkCommit(batch.SHA, "# Repository\n\nA new description\nAdditional notes:\nFirst branch change\nSecond branch change\n", "Apply suggestion from @"+reviewer.Name, trailer))
		for _, resolved := range []*issues_model.Comment{first, root, third, fourth, fifth} {
			unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: resolved.ID, ResolveDoerID: owner.ID})
		}
	})
}
