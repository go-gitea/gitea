// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/markup/markdown"

	"github.com/stretchr/testify/assert"
)

func TestRepoComment(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("AutoLink", func(t *testing.T) {
		rctx := NewRenderContextRepoComment(t.Context(), repo1).WithMarkupType(markdown.MarkupName)
		rendered, err := testRenderString(rctx, `
65f1bf27bc3bf70f64657658635e66094edbcb4d
#1
@user2
`)
		assert.NoError(t, err)
		assert.Equal(t,
			`<p><a href="/user2/repo1/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" rel="nofollow"><code>65f1bf27bc</code></a><br/>
<a href="/user2/repo1/issues/1" class="ref-issue" rel="nofollow">#1</a><br/>
<a href="/user2" rel="nofollow">@user2</a></p>
`, rendered)
	})

	t.Run("AbsoluteAndRelative", func(t *testing.T) {
		rctx := NewRenderContextRepoComment(t.Context(), repo1).WithMarkupType(markdown.MarkupName)

		// It is Gitea's old behavior, the relative path is resolved to the repo path
		// It is different from GitHub, GitHub resolves relative links to current page's path
		rendered, err := testRenderString(rctx, `
[/test](/test)
[./test](./test)
![/image](/image)
![./image](./image)
`)
		assert.NoError(t, err)
		assert.Equal(t,
			`<p><a href="/user2/repo1/test" rel="nofollow">/test</a><br/>
<a href="/user2/repo1/test" rel="nofollow">./test</a><br/>
<a href="/user2/repo1/image" target="_blank" rel="nofollow noopener"><img src="/user2/repo1/image" alt="/image"/></a><br/>
<a href="/user2/repo1/image" target="_blank" rel="nofollow noopener"><img src="/user2/repo1/image" alt="./image"/></a></p>
`, rendered)
	})

	t.Run("WithCurrentRefSubURL", func(t *testing.T) {
		rctx := NewRenderContextRepoComment(t.Context(), repo1, RepoCommentOptions{CurrentRefSubURL: "/commit/1234"}).
			WithMarkupType(markdown.MarkupName)

		// the ref path is only used to render commit message: a commit message is rendered at the commit page with its commit ID path
		rendered, err := testRenderString(rctx, `
[/test](/test)
[./test](./test)
![/image](/image)
![./image](./image)
`)
		assert.NoError(t, err)
		assert.Equal(t, `<p><a href="/user2/repo1/test" rel="nofollow">/test</a><br/>
<a href="/user2/repo1/commit/1234/test" rel="nofollow">./test</a><br/>
<a href="/user2/repo1/image" target="_blank" rel="nofollow noopener"><img src="/user2/repo1/image" alt="/image"/></a><br/>
<a href="/user2/repo1/commit/1234/image" target="_blank" rel="nofollow noopener"><img src="/user2/repo1/commit/1234/image" alt="./image"/></a></p>
`, rendered)
	})

	t.Run("HeadingAnchor", func(t *testing.T) {
		// markdown headings in comments must get an id so anchor links like "[x](#section-name)" can jump to them.
		// The per-comment context ID keeps the heading id and its link unique across comments on the same page,
		// so a link resolves to the heading in its own comment instead of the first matching one.
		input := "## Section Name\n\n[jump](#section-name)\n"

		rctx1 := NewRenderContextRepoComment(t.Context(), repo1, RepoCommentOptions{FootnoteContextID: "11"}).
			WithMarkupType(markdown.MarkupName)
		rendered1, err := testRenderString(rctx1, input)
		assert.NoError(t, err)
		assert.Equal(t,
			`<h2 id="user-content-section-name-11">Section Name</h2>
<p><a href="#user-content-section-name-11" rel="nofollow">jump</a></p>
`, rendered1)

		rctx2 := NewRenderContextRepoComment(t.Context(), repo1, RepoCommentOptions{FootnoteContextID: "22"}).
			WithMarkupType(markdown.MarkupName)
		rendered2, err := testRenderString(rctx2, input)
		assert.NoError(t, err)
		assert.Equal(t,
			`<h2 id="user-content-section-name-22">Section Name</h2>
<p><a href="#user-content-section-name-22" rel="nofollow">jump</a></p>
`, rendered2)
	})

	t.Run("NoRepo", func(t *testing.T) {
		rctx := NewRenderContextRepoComment(t.Context(), nil).WithMarkupType(markdown.MarkupName)
		rendered, err := testRenderString(rctx, "any")
		assert.NoError(t, err)
		assert.Equal(t, "<p>any</p>\n", rendered)
	})
}
