// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown_test

import (
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
)

func BenchmarkSpecializedMarkdown(b *testing.B) {
	// 240856	      4719 ns/op
	for b.Loop() {
		markdown.SpecializedMarkdown(&markup.RenderContext{})
	}
}

func BenchmarkMarkdownRender(b *testing.B) {
	// 23202	     50840 ns/op
	for b.Loop() {
		_, _ = markdown.RenderString(markup.NewTestRenderContext(), "https://example.com\n- a\n- b\n")
	}
}
