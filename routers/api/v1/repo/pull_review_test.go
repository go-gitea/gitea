// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestCreatePullReviewInvalidCommitID(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/pulls/2/reviews")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()
	ctx.SetPathParam("index", "2")

	web.SetForm(ctx, &api.CreatePullReviewOptions{
		Event:    api.ReviewStateComment,
		CommitID: strings.Repeat("a", 40),
		Comments: []api.CreatePullReviewComment{{Path: "README.md", Body: "inline", NewLineNum: 1}},
	})
	CreatePullReview(ctx)

	assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
	unittest.AssertNotExistsBean(t, &issues_model.Review{ReviewerID: 2, IssueID: 2, Type: issues_model.ReviewTypePending})
}
