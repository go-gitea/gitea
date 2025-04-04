// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadMarkup(t *testing.T) {
	cfg, _ := NewConfigProviderFromData(``)
	loadMarkupFrom(cfg)
	assert.Equal(t, MarkdownMathCodeBlockOptions{ParseInlineDollar: true, ParseBlockDollar: true}, Markdown.MathCodeBlockOptions)
	assert.Equal(t, MarkdownRenderOptions{NewLineHardBreak: true, ShortIssuePattern: true}, Markdown.RenderOptionsComment)
	assert.Equal(t, MarkdownRenderOptions{ShortIssuePattern: true}, Markdown.RenderOptionsWiki)
	assert.Equal(t, MarkdownRenderOptions{}, Markdown.RenderOptionsRepoFile)

	t.Run("Math", func(t *testing.T) {
		cfg, _ = NewConfigProviderFromData(`
[markdown]
MATH_CODE_BLOCK_DETECTION = none
`)
		loadMarkupFrom(cfg)
		assert.Equal(t, MarkdownMathCodeBlockOptions{}, Markdown.MathCodeBlockOptions)

		cfg, _ = NewConfigProviderFromData(`
[markdown]
MATH_CODE_BLOCK_DETECTION = inline-dollar, inline-parentheses, block-dollar, block-square-brackets
`)
		loadMarkupFrom(cfg)
		assert.Equal(t, MarkdownMathCodeBlockOptions{ParseInlineDollar: true, ParseInlineParentheses: true, ParseBlockDollar: true, ParseBlockSquareBrackets: true}, Markdown.MathCodeBlockOptions)
	})

	t.Run("Render", func(t *testing.T) {
		cfg, _ = NewConfigProviderFromData(`
[markdown]
RENDER_OPTIONS_COMMENT = none
`)
		loadMarkupFrom(cfg)
		assert.Equal(t, MarkdownRenderOptions{}, Markdown.RenderOptionsComment)

		cfg, _ = NewConfigProviderFromData(`
[markdown]
RENDER_OPTIONS_REPO_FILE = short-issue-pattern, new-line-hard-break
`)
		loadMarkupFrom(cfg)
		assert.Equal(t, MarkdownRenderOptions{NewLineHardBreak: true, ShortIssuePattern: true}, Markdown.RenderOptionsRepoFile)
	})
}
