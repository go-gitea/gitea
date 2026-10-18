// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePullReviewUnknownCommitID(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, resp := contexttest.MockAPIContext(t, "user2/repo1/pulls/2/reviews")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()
	ctx.SetPathParam("index", "2")

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	headCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	require.NoError(t, err)

	web.SetForm(ctx, &api.CreatePullReviewOptions{
		Event:    api.ReviewStateComment,
		CommitID: strings.Repeat("a", 40),
		Comments: []api.CreatePullReviewComment{{Path: "README.md", Body: "inline", NewLineNum: 1}},
	})
	CreatePullReview(ctx)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var review api.PullReview
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &review))
	assert.Equal(t, headCommitID, review.CommitID)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ReviewID: review.ID, Type: issues_model.CommentTypeCode, Content: "inline"})
}
