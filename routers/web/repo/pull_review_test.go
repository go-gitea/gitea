// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/templates"
	"gitea.dev/routers/common"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderConversation(t *testing.T) {
	unittest.PrepareTestEnv(t)

	pr, _ := issues_model.GetPullRequestByID(t.Context(), 2)
	_ = pr.LoadIssue(t.Context())
	_ = pr.Issue.LoadPoster(t.Context())
	_ = pr.Issue.LoadRepo(t.Context())

	run := func(name string, cb func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder)) {
		t.Run(name, func(t *testing.T) {
			ctx, resp := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
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

	for _, testCase := range []struct {
		name, origin, expected string
		showOutdated           bool
	}{
		{"diff with outdated", "diff", `<div class="content comment-container"`, true},
		{"diff without outdated", "diff", `conversation-not-existing`, false},
		{"timeline with outdated", "timeline", `<div id="code-comments-`, true},
		{"timeline is not affected by ShowOutdatedComments=false", "timeline", `<div id="code-comments-`, false},
	} {
		run(testCase.name, func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
			ctx.Data["ShowOutdatedComments"] = testCase.showOutdated
			renderConversation(ctx, preparedComment, testCase.origin)
			assert.Contains(t, resp.Body.String(), testCase.expected)
		})
	}
	for _, origin := range []string{"diff", "timeline"} {
		run(origin+" non-existing review", func(t *testing.T, ctx *context.Context, resp *httptest.ResponseRecorder) {
			err := db.TruncateBeans(t.Context(), &issues_model.Review{})
			assert.NoError(t, err)
			ctx.Data["ShowOutdatedComments"] = true
			renderConversation(ctx, preparedComment, origin)
			assert.Equal(t, http.StatusOK, resp.Code)
			assert.NotContains(t, resp.Body.String(), common.PageInternalServerErrorMark)
		})
	}
}
