// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestRenderHelperCodePreview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx, _ := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.HTMLRenderer()})
	htm, err := renderRepoFileCodePreview(ctx, markup.RenderCodePreviewOptions{
		FullURL:   "http://full",
		OwnerName: "user2",
		RepoName:  "repo1",
		CommitID:  "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		FilePath:  "/README.md",
		LineStart: 1,
		LineStop:  2,
	})
	assert.NoError(t, err)
	assert.Equal(t, `<div class="code-preview-container file-content">
	<div class="code-preview-header">
		<a href="http://full" class="muted" rel="nofollow">/README.md</a>
		repo.code_preview_line_from_to:1,2,<a href="/user2/repo1/src/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" rel="nofollow">65f1bf27bc</a>
	</div>
	<table class="file-view">
		<tbody><tr>
				<td class="lines-num"><span data-line-number="1"></span></td>
				<td class="lines-code chroma"><div class="code-inner"><span class="gh"># repo1</div></td>
			</tr><tr>
				<td class="lines-num"><span data-line-number="2"></span></td>
				<td class="lines-code chroma"><div class="code-inner"></span><span class="gh"></span></div></td>
			</tr></tbody>
	</table>
</div>
`, string(htm))

	ctx, _ = contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.HTMLRenderer()})
	htm, err = renderRepoFileCodePreview(ctx, markup.RenderCodePreviewOptions{
		FullURL:   "http://full",
		OwnerName: "user2",
		RepoName:  "repo1",
		CommitID:  "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		FilePath:  "/README.md",
		LineStart: 1,
	})
	assert.NoError(t, err)
	assert.Equal(t, `<div class="code-preview-container file-content">
	<div class="code-preview-header">
		<a href="http://full" class="muted" rel="nofollow">/README.md</a>
		repo.code_preview_line_in:1,<a href="/user2/repo1/src/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" rel="nofollow">65f1bf27bc</a>
	</div>
	<table class="file-view">
		<tbody><tr>
				<td class="lines-num"><span data-line-number="1"></span></td>
				<td class="lines-code chroma"><div class="code-inner"><span class="gh"># repo1</div></td>
			</tr></tbody>
	</table>
</div>
`, string(htm))

	ctx, _ = contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.HTMLRenderer()})
	_, err = renderRepoFileCodePreview(ctx, markup.RenderCodePreviewOptions{
		FullURL:   "http://full",
		OwnerName: "user15",
		RepoName:  "big_test_private_1",
		CommitID:  "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		FilePath:  "/README.md",
		LineStart: 1,
		LineStop:  10,
	})
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
}
