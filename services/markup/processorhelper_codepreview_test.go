// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestProcessorHelperCodePreview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx, _ := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.HTMLRenderer()})
	htm, err := renderRepoFileCodePreview(ctx, markup.RenderCodePreviewOptions{
		FullURL:   "http://full",
		OwnerName: "user2",
		RepoName:  "repo1",
		CommitID:  "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		FilePath:  "/README.md",
		LineStart: 1,
		LineStop:  10,
	})
	assert.NoError(t, err)
	assert.Equal(t, `<div class="code-preview-container">
	<div class="code-preview-header">
		<a href="http://full" class="muted" rel="nofollow">/README.md</a>
		Lines 1 to 10 in
		<a href="/user2/repo1/src/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" rel="nofollow">65f1bf27bc</a>
	</div>
	<table>
		<tbody><tr>
				<td class="lines-num"><span data-line-number="1"></span></td>
				<td class="lines-code chroma"><span class="gh"># repo1</td>
			</tr><tr>
				<td class="lines-num"><span data-line-number="2"></span></td>
				<td class="lines-code chroma"></span><span class="gh"></span></td>
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
	assert.ErrorContains(t, err, "no permission")
}
