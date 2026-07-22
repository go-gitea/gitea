// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestRenderHelperIssueIconTitle(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx, _ := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
	ctx.Repo.Repository = unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 1})
	htm, err := renderRepoIssueIconTitle(ctx, markup.RenderIssueIconTitleOptions{
		LinkHref:   "/link",
		IssueIndex: 1,
	})
	assert.NoError(t, err)
	assert.Equal(t, `<a href="/link"><span>octicon-issue-opened(16/tw-text-green)</span> issue1 (#1)</a>`, string(htm))

	ctx, _ = contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
	htm, err = renderRepoIssueIconTitle(ctx, markup.RenderIssueIconTitleOptions{
		OwnerName:  "user2",
		RepoName:   "repo1",
		LinkHref:   "/link",
		IssueIndex: 1,
	})
	assert.NoError(t, err)
	assert.Equal(t, `<a href="/link"><span>octicon-issue-opened(16/tw-text-green)</span> issue1 (user2/repo1#1)</a>`, string(htm))

	ctx, _ = contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
	_, err = renderRepoIssueIconTitle(ctx, markup.RenderIssueIconTitleOptions{
		OwnerName:  "user2",
		RepoName:   "repo2",
		LinkHref:   "/link",
		IssueIndex: 2,
	})
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
}
