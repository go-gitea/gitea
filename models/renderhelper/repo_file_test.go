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

func TestRepoFile(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("AutoLink", func(t *testing.T) {
		rctx := NewRenderContextRepoFile(context.Background(), repo1).WithMarkupType(markdown.MarkupName)
		rendered, err := markup.RenderString(rctx, `
65f1bf27bc3bf70f64657658635e66094edbcb4d
#1
@user2
`)
		assert.NoError(t, err)
		assert.Equal(t,
			`<p><a href="/user2/repo1/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d" rel="nofollow"><code>65f1bf27bc</code></a>
#1
<a href="/user2" rel="nofollow">@user2</a></p>
`, rendered)
	})

	t.Run("AbsoluteAndRelative", func(t *testing.T) {
		rctx := NewRenderContextRepoFile(context.Background(), repo1, RepoFileOptions{
			CurrentRefPath: "branch/main",
		}).WithMarkupType(markdown.MarkupName)
		rendered, err := markup.RenderString(rctx, `
[/test](/test)
[./test](./test)
![/image](/image)
![./image](./image)
`)
		assert.NoError(t, err)
		assert.Equal(t,
			`<p><a href="/user2/repo1/src/branch/main/test" rel="nofollow">/test</a>
<a href="/user2/repo1/src/branch/main/test" rel="nofollow">./test</a>
<a href="/user2/repo1/media/branch/main/image" target="_blank" rel="nofollow noopener"><img src="/user2/repo1/media/branch/main/image" alt="/image"/></a>
<a href="/user2/repo1/media/branch/main/image" target="_blank" rel="nofollow noopener"><img src="/user2/repo1/media/branch/main/image" alt="./image"/></a></p>
`, rendered)
	})

	t.Run("WithCurrentRefPath", func(t *testing.T) {
		rctx := NewRenderContextRepoFile(context.Background(), repo1, RepoFileOptions{CurrentRefPath: "/commit/1234"}).
			WithMarkupType(markdown.MarkupName)
		rendered, err := markup.RenderString(rctx, `
[/test](/test)
![/image](/image)
`)
		assert.NoError(t, err)
		assert.Equal(t, `<p><a href="/user2/repo1/src/commit/1234/test" rel="nofollow">/test</a>
<a href="/user2/repo1/media/commit/1234/image" target="_blank" rel="nofollow noopener"><img src="/user2/repo1/media/commit/1234/image" alt="/image"/></a></p>
`, rendered)
	})
}
