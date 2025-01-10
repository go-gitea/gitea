// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderConversation(t *testing.T) {
	unittest.PrepareTestEnv(t)

	pr, _ := issues_model.GetPullRequestByID(db.DefaultContext, 2)
	_ = pr.LoadIssue(db.DefaultContext)
	_ = pr.Issue.LoadPoster(db.DefaultContext)
	_ = pr.Issue.LoadRepo(db.DefaultContext)

	run := func(name string, cb func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder)) {
		t.Run(name, func(t *testing.T) {
			ctx, resp := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.HTMLRenderer()})
			contexttest.LoadUser(t, ctx, pr.Issue.PosterID)
			contexttest.LoadRepo(t, ctx, pr.BaseRepoID)
			contexttest.LoadGitRepo(t, ctx)
			defer ctx.Repo.GitRepo.Close()
			cb(t, ctx, resp)
		})
	}

	var preparedComment *issues_model.Comment
	run("prepare", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
		comment, err := pull.CreateCodeComment(ctx, pr.Issue.Poster, ctx.Repo.GitRepo, pr.Issue, 1, "content", "", false, 0, pr.HeadCommitID, nil)
		require.NoError(t, err)

		comment.Invalidated = true
		err = issues_model.UpdateCommentInvalidate(ctx, comment)
		require.NoError(t, err)

		preparedComment = comment
	})
	require.NotNil(t, preparedComment)

	run("diff with outdated", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
		ctx.Data["ShowOutdatedComments"] = true
		renderConversation(ctx, preparedComment, "diff")
		assert.Contains(t, resp.Body.String(), `<div class="content comment-container"`)
	})
	run("diff without outdated", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
		ctx.Data["ShowOutdatedComments"] = false
		renderConversation(ctx, preparedComment, "diff")
		assert.Contains(t, resp.Body.String(), `conversation-not-existing`)
	})
	run("timeline with outdated", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
		ctx.Data["ShowOutdatedComments"] = true
		renderConversation(ctx, preparedComment, "timeline")
		assert.Contains(t, resp.Body.String(), `<div id="code-comments-`)
	})
	run("timeline is not affected by ShowOutdatedComments=false", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
		ctx.Data["ShowOutdatedComments"] = false
		renderConversation(ctx, preparedComment, "timeline")
		assert.Contains(t, resp.Body.String(), `<div id="code-comments-`)
	})
	run("diff non-existing review", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
		err := db.TruncateBeans(db.DefaultContext, &issues_model.Review{})
		assert.NoError(t, err)
		ctx.Data["ShowOutdatedComments"] = true
		renderConversation(ctx, preparedComment, "diff")
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.NotContains(t, resp.Body.String(), `status-page-500`)
	})
	run("timeline non-existing review", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
		err := db.TruncateBeans(db.DefaultContext, &issues_model.Review{})
		assert.NoError(t, err)
		ctx.Data["ShowOutdatedComments"] = true
		renderConversation(ctx, preparedComment, "timeline")
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.NotContains(t, resp.Body.String(), `status-page-500`)
	})
}
