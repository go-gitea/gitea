// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"

	"github.com/stretchr/testify/assert"
)

func TestRepoComment(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("AutoLink", func(t *testing.T) {
		rctx := NewRenderContextRepoComment(context.Background(), repo1).WithMarkupType(markdown.MarkupName)
		rendered, err := markup.RenderString(rctx, `
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
		rctx := NewRenderContextRepoComment(context.Background(), repo1).WithMarkupType(markdown.MarkupName)

		// It is Gitea's old behavior, the relative path is resolved to the repo path
		// It is different from GitHub, GitHub resolves relative links to current page's path
		rendered, err := markup.RenderString(rctx, `
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

	t.Run("WithCurrentRefPath", func(t *testing.T) {
		rctx := NewRenderContextRepoComment(context.Background(), repo1, RepoCommentOptions{CurrentRefPath: "/commit/1234"}).
			WithMarkupType(markdown.MarkupName)

		// the ref path is only used to render commit message: a commit message is rendered at the commit page with its commit ID path
		rendered, err := markup.RenderString(rctx, `
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
}
