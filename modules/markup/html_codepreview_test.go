// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"context"
	"html/template"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"

	"github.com/stretchr/testify/assert"
)

func TestRenderCodePreview(t *testing.T) {
	markup.Init(&markup.RenderHelperFuncs{
		RenderRepoFileCodePreview: func(ctx context.Context, options markup.RenderCodePreviewOptions) (template.HTML, error) {
			return "<div>code preview</div>", nil
		},
	})
	test := func(input, expected string) {
		buffer, err := markup.RenderString(markup.NewTestRenderContext().WithMarkupType(markdown.MarkupName), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}
	test("http://localhost:3000/owner/repo/src/commit/0123456789/foo/bar.md#L10-L20", "<p><div>code preview</div></p>")
	test("http://other/owner/repo/src/commit/0123456789/foo/bar.md#L10-L20", `<p><a href="http://other/owner/repo/src/commit/0123456789/foo/bar.md#L10-L20" rel="nofollow">http://other/owner/repo/src/commit/0123456789/foo/bar.md#L10-L20</a></p>`)
}
