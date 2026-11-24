// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPullReviewSuggestions(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := t.Context()
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		apiCtx := NewAPITestContext(t, owner.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)

		const branch = "apply-suggestions"
		const path = "suggestions.txt"
		gitRepo, err := git.OpenRepository(ctx, repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		baseHead, err := gitRepo.GetBranchCommitID(ctx, repo.DefaultBranch)
		require.NoError(t, err)
		require.NoError(t, git.ForceFastImport(ctx, repo, []git.FastImportCommit{{
			Ref: git.RefNameFromBranch(branch).String(), Parent: baseHead,
			Files: []git.FastImportFile{{Path: path, Content: "first\nsecond\nthird\n"}},
		}}))
		require.NoError(t, repo_service.SyncBranchesToDB(ctx, repo, owner.ID, gitRepo, []string{branch}))
		initialHead, err := gitRepo.GetBranchCommitID(ctx, branch)
		require.NoError(t, err)
		pull, err := doAPICreatePullRequest(apiCtx, owner.Name, repo.Name, repo.DefaultBranch, branch)(t)
		require.NoError(t, err)
		pr, err := issues_model.GetPullRequestByIndex(ctx, repo.ID, pull.Index)
		require.NoError(t, err)

		createComment := func(doer *user_model.User, line int64, content string, pending bool, replyTo int64) *issues_model.Comment {
			t.Helper()
			comment, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, pr.Issue, pending, initialHead, issues_model.CreateCommentOptions{LineNum: line, Content: content, TreePath: path, ReviewID: replyTo})
			require.NoError(t, err)
			return comment
		}
		first := createComment(reviewer, 3, "```suggestion\nthree\n```", false, 0)
		root := createComment(owner, 2, "Please update this line.", false, 0)
		second := createComment(reviewer, 2, "```suggestion\ntwo\n```", false, root.ReviewID)
		third := createComment(owner, 1, "```suggestion\none\n```", true, 0)

		apiURL := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/suggestions/apply", owner.Name, repo.Name, pull.Index)
		assertAPIError := func(expectedStatus int, opts api.ApplyPullReviewSuggestionsOptions, message string) {
			t.Helper()
			response := MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, apiURL, opts).AddTokenAuth(apiCtx.Token), expectedStatus)
			assert.Contains(t, response.Body.String(), message)
		}

		assertAPIError(http.StatusUnprocessableEntity, api.ApplyPullReviewSuggestionsOptions{CommentIDs: []int64{third.ID}}, "pending reviews")
		require.NoError(t, issues_model.MarkConversation(ctx, root, owner, true))
		assertAPIError(http.StatusUnprocessableEntity, api.ApplyPullReviewSuggestionsOptions{CommentIDs: []int64{second.ID}}, "resolved conversations")
		root = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: root.ID})
		require.NoError(t, issues_model.MarkConversation(ctx, root, owner, false))
		_, _, err = pull_service.SubmitReview(ctx, owner, gitRepo, pr.Issue, issues_model.ReviewTypeComment, "", initialHead, nil)
		require.NoError(t, err)

		require.NoError(t, pull_service.LoadCodeSuggestionStates(ctx, reviewer, pr, []*issues_model.Comment{first}))
		assert.Nil(t, first.CodeSuggestion)

		webURL := fmt.Sprintf("/%s/%s/pulls/%d", owner.Name, repo.Name, pull.Index)
		page := apiCtx.Session.MakeRequest(t, NewRequest(t, http.MethodGet, webURL), http.StatusOK)
		controls := NewHTMLParser(t, page.Body).Find(fmt.Sprintf(`.code-suggestion-actions > [data-comment-id="%d"]`, first.ID))
		assert.Contains(t, controls.Find("button.show-modal").Text(), "Apply suggestion")
		assert.Contains(t, controls.Parent().Prev().Find(".code-suggestion").Text(), "Suggested change")
		response := apiCtx.Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, webURL+"/suggestions/apply", map[string]string{
			"origin":         "diff",
			"comment_ids":    fmt.Sprintf("%d:%d", first.ID, first.ContentVersion),
			"head_commit_id": initialHead,
		}), http.StatusOK)
		assert.Equal(t, webURL+"/files", DecodeJSON(t, response, map[string]string{})["redirect"])

		checkCommit := func(parent, content, title, body string) string {
			t.Helper()
			commit, err := gitRepo.GetBranchCommit(ctx, branch)
			require.NoError(t, err)
			parentID, err := commit.ParentID(0)
			require.NoError(t, err)
			assert.Equal(t, parent, parentID.String())
			assert.Equal(t, owner.GitName(), commit.Author.Name)
			assert.Equal(t, owner.GetEmail(), commit.Author.Email)
			assert.Equal(t, title, commit.MessageTitle())
			assert.Equal(t, body, commit.MessageBody())
			actual, err := commit.GetFileContent(ctx, gitRepo, path, 1024)
			require.NoError(t, err)
			assert.Equal(t, content, actual)
			return commit.ID.String()
		}

		trailer := git.CoAuthoredByTrailer + ": " + reviewer.NewGitSig().String()
		webHead := checkCommit(initialHead, "first\nsecond\nthree\n", "Apply suggestion from @"+reviewer.Name, trailer)
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: first.ID, ResolveDoerID: owner.ID})

		assertAPIError(http.StatusConflict, api.ApplyPullReviewSuggestionsOptions{CommentIDs: []int64{second.ID}, HeadCommitID: initialHead}, "branch changed")
		response = MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, apiURL, api.ApplyPullReviewSuggestionsOptions{
			CommentIDs: []int64{second.ID, third.ID, second.ID},
			Message:    "Apply the remaining edits\n\nFollow up on the review.",
		}).AddTokenAuth(apiCtx.Token), http.StatusCreated)
		result := DecodeJSON(t, response, api.ApplyPullReviewSuggestionsResponse{})
		assert.Equal(t, result.SHA, checkCommit(webHead, "one\ntwo\nthree\n", "Apply the remaining edits", "Follow up on the review.\n\n"+trailer))
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: root.ID, ResolveDoerID: owner.ID})
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: third.ID, ResolveDoerID: owner.ID})
	})
}
