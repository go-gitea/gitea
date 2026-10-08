// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestRenderHelperCodePreview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx, _ := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
	htm, err := renderRepoFileCodePreview(ctx, markup.RenderCodePreviewOptions{
		FullURL:   "http://full",
		OwnerName: "user2",
		RepoName:  "repo1",
		CommitID:  "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		FilePath:  "README.md",
		LineStart: 1,
		LineStop:  2,
	})
	assert.NoError(t, err)
	assert.Equal(t, `<div class="code-preview-container file-content">
	<div class="code-preview-header">
		<a href="http://full" class="tw-font-semibold" rel="nofollow">repo1/README.md</a>
		repo.code_preview_line_from_to:1,2,<a href="/user2/repo1/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" class="muted tw-font-mono tw-text-text" rel="nofollow">65f1bf27bc</a>
	</div>
	<table class="file-view">
		<tbody><tr>
				<td class="lines-num"><span data-line-number="1"></span></td>
				<td class="lines-code chroma"><div class="code-inner"><span class="gh"># repo1
</span></div></td>
			</tr><tr>
				<td class="lines-num"><span data-line-number="2"></span></td>
				<td class="lines-code chroma"><div class="code-inner">
</div></td>
			</tr></tbody>
	</table>
</div>
`, string(htm))

	ctx, _ = contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
	htm, err = renderRepoFileCodePreview(ctx, markup.RenderCodePreviewOptions{
		FullURL:   "http://full",
		OwnerName: "user2",
		RepoName:  "repo1",
		CommitID:  "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		FilePath:  "README.md",
		LineStart: 1,
	})
	assert.NoError(t, err)
	assert.Equal(t, `<div class="code-preview-container file-content">
	<div class="code-preview-header">
		<a href="http://full" class="tw-font-semibold" rel="nofollow">repo1/README.md</a>
		repo.code_preview_line_in:1,<a href="/user2/repo1/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" class="muted tw-font-mono tw-text-text" rel="nofollow">65f1bf27bc</a>
	</div>
	<table class="file-view">
		<tbody><tr>
				<td class="lines-num"><span data-line-number="1"></span></td>
				<td class="lines-code chroma"><div class="code-inner"><span class="gh"># repo1
</span></div></td>
			</tr></tbody>
	</table>
</div>
`, string(htm))

	ctx, _ = contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.PageRenderer()})
	_, err = renderRepoFileCodePreview(ctx, markup.RenderCodePreviewOptions{
		FullURL:   "http://full",
		OwnerName: "user15",
		RepoName:  "big_test_private_1",
		CommitID:  "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		FilePath:  "README.md",
		LineStart: 1,
		LineStop:  10,
	})
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
}
