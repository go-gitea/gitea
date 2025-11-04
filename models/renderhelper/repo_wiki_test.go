// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"

	"github.com/stretchr/testify/assert"
)

func TestRepoWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("AutoLink", func(t *testing.T) {
		rctx := NewRenderContextRepoWiki(t.Context(), repo1).WithMarkupType(markdown.MarkupName)
		rendered, err := markup.RenderString(rctx, `
65f1bf27bc3bf70f64657658635e66094edbcb4d
#1
@user2
`)
		assert.NoError(t, err)
		// Repository 1 has subject_id 1, which is "example-subject"
		assert.Equal(t,
			`<p><a href="/article/user2/example-subject/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" rel="nofollow"><code>65f1bf27bc</code></a>
<a href="/article/user2/example-subject/issues/1" class="ref-issue" rel="nofollow">#1</a>
<a href="/user2" rel="nofollow">@user2</a></p>
`, rendered)
	})

	t.Run("AbsoluteAndRelative", func(t *testing.T) {
		rctx := NewRenderContextRepoWiki(t.Context(), repo1).WithMarkupType(markdown.MarkupName)
		rendered, err := markup.RenderString(rctx, `
[/test](/test)
[./test](./test)
![/image](/image)
![./image](./image)
`)
		assert.NoError(t, err)
		// Repository 1 has subject_id 1, which is "example-subject"
		assert.Equal(t,
			`<p><a href="/article/user2/example-subject/wiki/test" rel="nofollow">/test</a>
<a href="/article/user2/example-subject/wiki/test" rel="nofollow">./test</a>
<a href="/article/user2/example-subject/wiki/image" target="_blank" rel="nofollow noopener"><img src="/article/user2/example-subject/wiki/raw/image" alt="/image"/></a>
<a href="/article/user2/example-subject/wiki/image" target="_blank" rel="nofollow noopener"><img src="/article/user2/example-subject/wiki/raw/image" alt="./image"/></a></p>
`, rendered)
	})

	t.Run("PathInTag", func(t *testing.T) {
		rctx := NewRenderContextRepoWiki(t.Context(), repo1).WithMarkupType(markdown.MarkupName)
		rendered, err := markup.RenderString(rctx, `
<img src="LINK">
<video src="LINK">
`)
		assert.NoError(t, err)
		// Repository 1 has subject_id 1, which is "example-subject"
		assert.Equal(t, `<a href="/article/user2/example-subject/wiki/LINK" target="_blank" rel="nofollow noopener"><img src="/article/user2/example-subject/wiki/raw/LINK"/></a>
<video src="/article/user2/example-subject/wiki/raw/LINK">
</video>`, rendered)
	})
}
