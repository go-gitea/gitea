// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package htmldiff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiff(t *testing.T) {
	const add, del = `class="rich-diff-added"`, `class="rich-diff-removed"`
	cases := []struct {
		name, old, new, expected string
	}{
		{
			name:     "words",
			old:      `<p>The quick brown fox jumps over the lazy dog.</p>`,
			new:      `<p>The quick red fox leaps over the lazy dog.</p>`,
			expected: `<p>The quick <del ` + del + `>brown</del><ins ` + add + `>red</ins> fox <del ` + del + `>jumps</del><ins ` + add + `>leaps</ins> over the lazy dog.</p>`,
		},
		{
			name:     "formatting",
			old:      `<p>Some <strong>bold</strong> text</p>`,
			new:      `<p>Some <strong>strong</strong> text</p>`,
			expected: `<p>Some <strong><del ` + del + `>bold</del><ins ` + add + `>strong</ins></strong> text</p>`,
		},
		{
			name:     "existing del element is kept",
			old:      `<p>a <del>struck</del> b</p>`,
			new:      `<p>a <del>struck</del> c</p>`,
			expected: `<p>a <del>struck</del> <del ` + del + `>b</del><ins ` + add + `>c</ins></p>`,
		},
		{
			name:     "no words in common",
			old:      `<p>alpha beta</p>`,
			new:      `<p>gamma delta</p>`,
			expected: `<p ` + del + `>alpha beta</p><p ` + add + `>gamma delta</p>`,
		},
		{
			name:     "block tag changed",
			old:      `<p>Getting started</p><p>Body</p>`,
			new:      `<h2 id="x">Getting started</h2><p>Body</p>`,
			expected: `<p ` + del + `>Getting started</p><h2 id="x" ` + add + `>Getting started</h2><p>Body</p>`,
		},
		{
			name:     "link target changed",
			old:      `<p>See <a href="/old">the docs</a> now.</p>`,
			new:      `<p>See <a href="/new">the docs</a> now.</p>`,
			expected: `<p>See <a href="/old"><del ` + del + `>the docs</del></a><a href="/new"><ins ` + add + `>the docs</ins></a> now.</p>`,
		},
		{
			name:     "normalized link is unchanged",
			old:      `<p><img src="/media/commit/old-sha/x.png" alt="x"></p>`,
			new:      `<p><img src="/media/commit/new-sha/x.png" alt="x"></p>`,
			expected: `<p><img src="/media/commit/new-sha/x.png" alt="x"/></p>`,
		},
		{
			name:     "table cell",
			old:      "<table><tbody>\n<tr>\n<td>1</td>\n<td>beta</td>\n</tr>\n</tbody></table>",
			new:      "<table><tbody>\n<tr>\n<td>1</td>\n<td>stable</td>\n</tr>\n</tbody></table>",
			expected: `<table><tbody><tr><td>1</td><td><del ` + del + `>beta</del><ins ` + add + `>stable</ins></td></tr></tbody></table>`,
		},
		{
			name:     "inline math is atomic",
			old:      `<p>Inline <code class="language-math">E=mc^2</code> here.</p>`,
			new:      `<p>Inline <code class="language-math">E=mc^3</code> here.</p>`,
			expected: `<p>Inline <del ` + del + `><code class="language-math">E=mc^2</code></del><ins ` + add + `><code class="language-math">E=mc^3</code></ins> here.</p>`,
		},
		{
			name: "mermaid block is atomic",
			old:  `<div class="code-block-container"><pre class="code-block"><code class="chroma language-mermaid">A--&gt;B</code></pre></div>`,
			new:  `<div class="code-block-container"><pre class="code-block"><code class="chroma language-mermaid">A--&gt;C</code></pre></div>`,
			expected: `<div ` + del + `><div class="code-block-container"><pre class="code-block"><code class="chroma language-mermaid">A--&gt;B</code></pre></div></div>` +
				`<div ` + add + `><div class="code-block-container"><pre class="code-block"><code class="chroma language-mermaid">A--&gt;C</code></pre></div></div>`,
		},
		{
			name:     "code block line",
			old:      "<pre><code>line1\nline2\n</code></pre>",
			new:      "<pre><code>line1\nline2 changed\n</code></pre>",
			expected: "<pre><code>line1\nline2<ins " + add + "> changed</ins>\n</code></pre>",
		},
		{
			name:     "reordered paragraphs",
			old:      `<p>one</p><p>two</p><p>three</p>`,
			new:      `<p>three</p><p>one</p><p>two</p>`,
			expected: `<p ` + add + `>three</p><p>one</p><p>two</p><p ` + del + `>three</p>`,
		},
		{
			name:     "paragraph to list",
			old:      `<p>We support Linux and macOS.</p>`,
			new:      `<p>We support:</p><ul><li>Linux</li><li>macOS</li></ul>`,
			expected: `<p>We support<del ` + del + `> Linux and macOS.</del><ins ` + add + `>:</ins></p><ul ` + add + `><li>Linux</li><li>macOS</li></ul>`,
		},
		{
			name:     "nested list",
			old:      "<ul>\n<li>item<ul><li>x</li></ul></li>\n</ul>",
			new:      "<ul>\n<li>item changed<ul><li>x</li><li>y</li></ul></li>\n</ul>",
			expected: `<ul><li>item<ins ` + add + `> changed</ins><ul><li>x</li><li ` + add + `>y</li></ul></li></ul>`,
		},
		{
			name:     "task checked",
			old:      `<ul><li><input type="checkbox" disabled="" data-source-position="3"/>task</li></ul>`,
			new:      `<ul><li><input type="checkbox" checked="" disabled="" data-source-position="9"/>task</li></ul>`,
			expected: `<ul><li><del ` + del + `><input type="checkbox" disabled="" data-source-position="3"/></del><ins ` + add + `><input type="checkbox" checked="" disabled="" data-source-position="9"/></ins>task</li></ul>`,
		},
		{
			name:     "added document",
			old:      ``,
			new:      `<h1>New</h1><p>file</p>`,
			expected: `<h1 ` + add + `>New</h1><p ` + add + `>file</p>`,
		},
	}
	opts := Options{NormalizeLink: func(link string) string {
		return strings.NewReplacer("old-sha", "", "new-sha", "").Replace(link)
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := Diff(c.old, c.new, opts)
			require.NoError(t, err)
			assert.Equal(t, c.expected, string(out))
		})
	}
}
