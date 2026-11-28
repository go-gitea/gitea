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
	const add, del, changed = `class="rich-diff-added"`, `class="rich-diff-removed"`, `class="rich-diff-changed"`
	const blockAdded, blockRemoved, blockChanged = `<div class="rich-diff-block rich-diff-block-added">`, `<div class="rich-diff-block rich-diff-block-removed">`, `<div class="rich-diff-block rich-diff-block-changed">`
	cases := []struct {
		name, old, new, expected string
	}{
		{
			name:     "words and version number",
			old:      `<p>The quick brown fox needs Go 1.24 or newer.</p>`,
			new:      `<p>The quick red fox needs Go 1.25 or newer.</p>`,
			expected: blockChanged + `<p>The quick <del ` + del + `>brown</del><ins ` + add + `>red</ins> fox needs Go <del ` + del + `>1.24</del><ins ` + add + `>1.25</ins> or newer.</p></div>`,
		},
		{
			name:     "reflowed text is unchanged",
			old:      "<p>one two\nthree</p>",
			new:      "<p>one\ntwo three</p>",
			expected: "<p>one\ntwo three</p>",
		},
		{
			name:     "formatting and link target change",
			old:      `<p>Some bold text, see <a href="/old">the docs</a>.</p>`,
			new:      `<p>Some <strong>bold</strong> text, see <a href="/new">the docs</a>.</p>`,
			expected: blockChanged + `<p>Some <strong><span ` + changed + `>bold</span></strong> text, see <a href="/new"><span ` + changed + `>the docs</span></a>.</p></div>`,
		},
		{
			name:     "equal siblings are not merged",
			old:      `<p><em>A</em><em>B</em><em>C</em> x</p>`,
			new:      `<p><em>A</em><em>C</em><em>D</em> x</p>`,
			expected: blockChanged + `<p><em>A</em><em><del ` + del + `>B</del></em><em>C</em><em><ins ` + add + `>D</ins></em> x</p></div>`,
		},
		{
			name:     "inline code and emoji are replaced whole",
			old:      `<p>Call <code>foo(1)</code> <span class="emoji">🇺🇸</span></p>`,
			new:      `<p>Call <code>bar(1)</code> <span class="emoji">🇺🇦</span></p>`,
			expected: blockChanged + `<p>Call <del ` + del + `><code>foo(1)</code> <span class="emoji">🇺🇸</span></del><ins ` + add + `><code>bar(1)</code> <span class="emoji">🇺🇦</span></ins></p></div>`,
		},
		{
			name:     "checkbox and image alt change in place",
			old:      `<ul><li><input type="checkbox"/>task <img src="/media/commit/old-sha/x.png" alt="x"></li></ul>`,
			new:      `<ul><li><input type="checkbox" checked=""/>task <img src="/media/commit/new-sha/x.png" alt="y"></li></ul>`,
			expected: blockChanged + `<ul><li><span ` + changed + `><input type="checkbox" checked=""/></span>task <span ` + changed + `><img src="/media/commit/new-sha/x.png" alt="y"/></span></li></ul></div>`,
		},
		{
			name:     "no words in common",
			old:      `<p>alpha beta.</p>`,
			new:      `<p>gamma delta.</p>`,
			expected: blockRemoved + `<p>alpha beta.</p></div>` + blockAdded + `<p>gamma delta.</p></div>`,
		},
		{
			name:     "inserted paragraph before an edited one",
			old:      `<p>Install npm.</p><p>Build it.</p>`,
			new:      `<p>Requires Node.</p><p>Install pnpm.</p><p>Build it.</p>`,
			expected: blockAdded + `<p>Requires Node.</p></div>` + blockChanged + `<p>Install <del ` + del + `>npm</del><ins ` + add + `>pnpm</ins>.</p></div><p>Build it.</p>`,
		},
		{
			name:     "heading level changed",
			old:      `<h2 id="install">Install</h2>`,
			new:      `<h3 id="install">Install</h3>`,
			expected: blockChanged + `<h3 id="install" ` + changed + `>Install</h3></div>`,
		},
		{
			name:     "block tag changed and removed copy has no id",
			old:      `<h2 id="intro">Intro <a id="anchor" href="#x">x</a></h2>`,
			new:      `<p>Intro <a id="anchor" href="#x">x</a></p>`,
			expected: blockRemoved + `<h2>Intro <a href="#x">x</a></h2></div>` + blockAdded + `<p>Intro <a href="#x" id="anchor">x</a></p></div>`,
		},
		{
			name:     "reordered paragraphs",
			old:      `<p>one</p><p>two</p><p>three</p>`,
			new:      `<p>three</p><p>one</p><p>two</p>`,
			expected: blockAdded + `<p>three</p></div><p>one</p><p>two</p>` + blockRemoved + `<p>three</p></div>`,
		},
		{
			name:     "list item added and loose",
			old:      "<ul>\n<li>one</li>\n<li>two</li>\n</ul>",
			new:      "<ul>\n<li>\n<p>one</p>\n</li>\n<li>\n<p>two</p>\n<p>more</p>\n</li>\n<li>three</li>\n</ul>",
			expected: blockChanged + `<ul><li><p>one</p></li><li><p>two</p><p ` + add + `>more</p></li><li ` + add + `>three</li></ul></div>`,
		},
		{
			name:     "list start changed",
			old:      `<ol start="1"><li>a</li></ol>`,
			new:      `<ol start="5"><li>a</li></ol>`,
			expected: blockChanged + `<ol start="5" ` + changed + `><li>a</li></ol></div>`,
		},
		{
			name:     "table cells by position",
			old:      "<table><tbody>\n<tr>\n<td align=\"left\">1</td>\n<td>beta</td>\n</tr>\n</tbody></table>",
			new:      "<table><tbody>\n<tr>\n<td align=\"center\">1</td>\n<td>stable</td>\n</tr>\n</tbody></table>",
			expected: blockChanged + `<table><tbody><tr><td align="center" ` + changed + `>1</td><td><del ` + del + `>beta</del><ins ` + add + `>stable</ins></td></tr></tbody></table></div>`,
		},
		{
			name: "table columns changed",
			old:  `<table><tbody><tr><td>1</td></tr></tbody></table><table><tbody><tr><td>a</td></tr><tr><td>b</td><td>c</td></tr></tbody></table>`,
			new:  `<table><tbody><tr><td>1</td><td>2</td></tr></tbody></table><table><tbody><tr><td>a</td></tr><tr><td>c</td></tr></tbody></table>`,
			expected: blockRemoved + `<table><tbody><tr><td>1</td></tr></tbody></table></div>` + blockAdded + `<table><tbody><tr><td>1</td><td>2</td></tr></tbody></table></div>` +
				blockChanged + `<table><tbody><tr><td>a</td></tr><tr ` + del + `><td>b</td><td>c</td></tr><tr ` + add + `><td>c</td></tr></tbody></table></div>`,
		},
		{
			name:     "code block lines",
			old:      "<pre><code><span>a</span>\n<span>b</span>\n</code></pre>",
			new:      "<pre><code><span>a</span>\n<span>c</span>\n</code></pre>",
			expected: blockChanged + "<pre><code><span>a</span>\n<span><del " + del + ">b</del></span><del " + del + ">\n</del><span><ins " + add + ">c</ins></span><ins " + add + ">\n</ins></code></pre></div>",
		},
		{
			name:     "code block highlighting change only",
			old:      "<pre><code class=\"language-js\"><span class=\"kd\">const</span> a\n</code></pre>",
			new:      "<pre><code class=\"language-ts\"><span class=\"kr\">const</span> a\n</code></pre>",
			expected: "<pre><code class=\"language-ts\"><span class=\"kr\">const</span> a\n</code></pre>",
		},
		{
			name:     "mermaid block is atomic",
			old:      `<pre><code class="language-mermaid">A--&gt;B</code></pre>`,
			new:      `<pre><code class="language-mermaid">A--&gt;C</code></pre>`,
			expected: blockRemoved + `<pre><code class="language-mermaid">A--&gt;B</code></pre></div>` + blockAdded + `<pre><code class="language-mermaid">A--&gt;C</code></pre></div>`,
		},
	}
	normalizeLink := strings.NewReplacer("old-sha", "", "new-sha", "").Replace
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			out, err := Diff(testCase.old, testCase.new, normalizeLink)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, string(out))
		})
	}
}
