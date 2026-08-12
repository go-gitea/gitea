// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func TestLoadMarkup(t *testing.T) {
	defer test.MockVariableValue(&Markdown)()

	testConfigLoad(t, []any{loadMarkupFrom}, []configTestCase{
		{
			name: "defaults",
			want: []configCheck{
				field("MATH_CODE_BLOCK_DETECTION", &Markdown.MathCodeBlockOptions, MarkdownMathCodeBlockOptions{ParseInlineDollar: true, ParseBlockDollar: true}),
				field("RENDER_OPTIONS_COMMENT", &Markdown.RenderOptionsComment, MarkdownRenderOptions{NewLineHardBreak: true, ShortIssuePattern: true}),
				field("RENDER_OPTIONS_WIKI", &Markdown.RenderOptionsWiki, MarkdownRenderOptions{ShortIssuePattern: true}),
				field("RENDER_OPTIONS_REPO_FILE", &Markdown.RenderOptionsRepoFile, MarkdownRenderOptions{}),
			},
		},
		{
			name: "math detection none",
			ini:  "[markdown]\nMATH_CODE_BLOCK_DETECTION = none",
			want: []configCheck{field("MATH_CODE_BLOCK_DETECTION", &Markdown.MathCodeBlockOptions, MarkdownMathCodeBlockOptions{})},
		},
		{
			name: "math detection all delimiters",
			ini:  "[markdown]\nMATH_CODE_BLOCK_DETECTION = inline-dollar, inline-parentheses, block-dollar, block-square-brackets",
			want: []configCheck{field("MATH_CODE_BLOCK_DETECTION", &Markdown.MathCodeBlockOptions, MarkdownMathCodeBlockOptions{ParseInlineDollar: true, ParseInlineParentheses: true, ParseBlockDollar: true, ParseBlockSquareBrackets: true})},
		},
		{
			name: "comment render options none",
			ini:  "[markdown]\nRENDER_OPTIONS_COMMENT = none",
			want: []configCheck{field("RENDER_OPTIONS_COMMENT", &Markdown.RenderOptionsComment, MarkdownRenderOptions{})},
		},
		{
			name: "repo file render options",
			ini:  "[markdown]\nRENDER_OPTIONS_REPO_FILE = short-issue-pattern, new-line-hard-break",
			want: []configCheck{field("RENDER_OPTIONS_REPO_FILE", &Markdown.RenderOptionsRepoFile, MarkdownRenderOptions{NewLineHardBreak: true, ShortIssuePattern: true})},
		},
	})
}
