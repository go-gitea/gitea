// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"

	"github.com/stretchr/testify/assert"
)

func TestSimpleDocument(t *testing.T) {
	unittest.PrepareTestEnv(t)
	rctx := NewRenderContextSimpleDocument(t.Context(), "/base").WithMarkupType(markdown.MarkupName)
	rendered, err := markup.RenderString(rctx, `
65f1bf27bc3bf70f64657658635e66094edbcb4d
#1
@user2

[/test](/test)
[./test](./test)
![/image](/image)
![./image](./image)
`)
	assert.NoError(t, err)
	assert.Equal(t,
		`<p>65f1bf27bc3bf70f64657658635e66094edbcb4d
#1
<a href="/base/user2" rel="nofollow">@user2</a></p>
<p><a href="/base/test" rel="nofollow">/test</a>
<a href="/base/test" rel="nofollow">./test</a>
<a href="/base/image" target="_blank" rel="nofollow noopener"><img src="/base/image" alt="/image"/></a>
<a href="/base/image" target="_blank" rel="nofollow noopener"><img src="/base/image" alt="./image"/></a></p>
`, rendered)
}
