// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown_test

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/svg"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func TestAttention(t *testing.T) {
	defer svg.MockIcon("octicon-info")()
	defer svg.MockIcon("octicon-light-bulb")()
	defer svg.MockIcon("octicon-report")()
	defer svg.MockIcon("octicon-alert")()
	defer svg.MockIcon("octicon-stop")()

	renderAttention := func(attention, icon string) string {
		tmpl := `<blockquote class="attention-header attention-{attention}"><p><svg class="attention-icon attention-{attention} svg {icon}" width="16" height="16"></svg><strong class="attention-{attention}">{Attention}</strong></p>`
		tmpl = strings.ReplaceAll(tmpl, "{attention}", attention)
		tmpl = strings.ReplaceAll(tmpl, "{icon}", icon)
		tmpl = strings.ReplaceAll(tmpl, "{Attention}", cases.Title(language.English).String(attention))
		return tmpl
	}

	test := func(input, expected string) {
		result, err := markdown.RenderString(markup.NewTestRenderContext(), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(result)))
	}

	test(`
> [!NOTE]
> text
`, renderAttention("note", "octicon-info")+"\n<p>text</p>\n</blockquote>")

	test(`> [!note]`, renderAttention("note", "octicon-info")+"\n</blockquote>")
	test(`> [!tip]`, renderAttention("tip", "octicon-light-bulb")+"\n</blockquote>")
	test(`> [!important]`, renderAttention("important", "octicon-report")+"\n</blockquote>")
	test(`> [!warning]`, renderAttention("warning", "octicon-alert")+"\n</blockquote>")
	test(`> [!caution]`, renderAttention("caution", "octicon-stop")+"\n</blockquote>")

	// escaped by mdformat
	test(`> \[!NOTE\]`, renderAttention("note", "octicon-info")+"\n</blockquote>")

	// legacy GitHub style
	test(`> **warning**`, renderAttention("warning", "octicon-alert")+"\n</blockquote>")
}
