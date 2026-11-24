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
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderConversation(t *testing.T) {
	unittest.PrepareTestEnv(t)

	pr, err := issues_model.GetPullRequestByID(t.Context(), 2)
	require.NoError(t, err)
	require.NoError(t, pr.LoadIssue(t.Context()))
	newContext := func(t *testing.T) (*context.Context, *httptest.ResponseRecorder) {
		t.Helper()
		ctx, resp := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
		contexttest.LoadUser(t, ctx, pr.Issue.PosterID)
		contexttest.LoadRepo(t, ctx, pr.BaseRepoID)
		contexttest.LoadGitRepo(t, ctx)
		t.Cleanup(func() { ctx.Repo.GitRepo.Close() })
		return ctx, resp
	}

	ctx, _ := newContext(t)
	comment, err := pull.CreateCodeComment(ctx, ctx.Doer, ctx.Repo.GitRepo, pr.Issue, false, pr.HeadCommitID, issues_model.CreateCommentOptions{LineNum: 1, Content: "content"})
	require.NoError(t, err)
	comment.Invalidated = true
	require.NoError(t, issues_model.UpdateCommentInvalidate(ctx, comment))

	check := func(t *testing.T, origin string, showOutdated bool, want string) {
		t.Helper()
		ctx, resp := newContext(t)
		ctx.Data["ShowOutdatedComments"] = showOutdated
		renderConversation(ctx, comment, origin)
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), want)
	}
	for _, test := range []struct {
		name, origin, want string
		showOutdated       bool
	}{
		{"DiffShowsOutdated", "diff", `<div class="content comment-container"`, true},
		{"DiffHidesOutdated", "diff", "conversation-not-existing", false},
		{"TimelineAlwaysShowsOutdated", "timeline", `<div id="code-comments-`, false},
	} {
		t.Run(test.name, func(t *testing.T) { check(t, test.origin, test.showOutdated, test.want) })
	}

	_, err = db.DeleteByID[issues_model.Review](t.Context(), comment.ReviewID)
	require.NoError(t, err)
	for _, test := range []struct{ origin, want string }{
		{"diff", `<div class="content comment-container"`}, {"timeline", `<div id="code-comments-`},
	} {
		t.Run(test.origin+"/MissingReview", func(t *testing.T) { check(t, test.origin, true, test.want) })
	}
}
