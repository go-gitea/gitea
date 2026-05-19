// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLDiff(t *testing.T) {
	t.Run("unchanged content", func(t *testing.T) {
		out := HTMLDiff("<p>hello</p>", "<p>hello</p>")
		assert.Equal(t, `<p>hello</p>`, string(out))
	})

	t.Run("both empty", func(t *testing.T) {
		out := HTMLDiff("", "")
		assert.Empty(t, string(out))
	})

	t.Run("word insertion", func(t *testing.T) {
		// old bundles to `<p>hello</p>`, new cannot bundle (space in body) so the two sides
		// share no tokens; the diff degrades to a full replacement, which is still valid HTML.
		out := HTMLDiff("<p>hello</p>", "<p>hello world</p>")
		assert.Equal(t, `<del class="removed-code"><p>hello</p></del><p><span class="added-code">hello world</span></p>`, string(out))
	})

	t.Run("word deletion", func(t *testing.T) {
		out := HTMLDiff("<p>hello world</p>", "<p>hello</p>")
		assert.Equal(t, `<p><span class="removed-code">hello world</span></p><ins class="added-code"><p>hello</p></ins>`, string(out))
	})

	t.Run("word replacement", func(t *testing.T) {
		out := HTMLDiff("<p>hello</p>", "<p>world</p>")
		assert.Equal(t, `<del class="removed-code"><p>hello</p></del><ins class="added-code"><p>world</p></ins>`, string(out))
	})

	t.Run("insert only", func(t *testing.T) {
		out := HTMLDiff("", "<p>hello</p>")
		assert.Equal(t, `<ins class="added-code"><p>hello</p></ins>`, string(out))
	})

	t.Run("delete only", func(t *testing.T) {
		out := HTMLDiff("<p>hello</p>", "")
		assert.Equal(t, `<del class="removed-code"><p>hello</p></del>`, string(out))
	})

	t.Run("heading id change", func(t *testing.T) {
		// Auto-generated heading ids change between revisions. Each side becomes its own bundled
		// token so the output is a full replacement rather than hiding the attribute change.
		out := HTMLDiff(`<h2 id="bar">bar</h2>`, `<h2 id="baz">baz</h2>`)
		assert.Equal(t, `<del class="removed-code"><h2 id="bar">bar</h2></del><ins class="added-code"><h2 id="baz">baz</h2></ins>`, string(out))
	})

	t.Run("href change is visible", func(t *testing.T) {
		// Pure attribute changes must not be silently dropped: both sides are emitted.
		out := HTMLDiff(`<a href="/old">content</a>`, `<a href="/new">content</a>`)
		assert.Equal(t, `<del class="removed-code"><a href="/old">content</a></del><ins class="added-code"><a href="/new">content</a></ins>`, string(out))
	})

	t.Run("nested inline tags", func(t *testing.T) {
		out := HTMLDiff(`<p>this is <strong>bold</strong> text</p>`, `<p>this is <strong>bolder</strong> text</p>`)
		assert.Equal(t, `<p>this is <span class="removed-code"><strong>bold</strong></span><span class="added-code"><strong>bolder</strong></span> text</p>`, string(out))
	})

	t.Run("void element paired with a closer is not bundled", func(t *testing.T) {
		// `<img></p>` must not be bundled as a single `<<img></p>>` unit — <img> is a void element
		// and does not pair with <p>. Otherwise the diff output would contain crossed/duplicated tags.
		// The block splitter serializes void elements in the XHTML self-closing form.
		out := HTMLDiff(`<p><img src="/a.png" alt="a"></p>`, `<p><img src="/b.png" alt="b"></p>`)
		assert.Equal(t, `<p><span class="removed-code"><img src="/a.png" alt="a"/></span><span class="added-code"><img src="/b.png" alt="b"/></span></p>`, string(out))
	})

	t.Run("replace paragraph with image", func(t *testing.T) {
		out := HTMLDiff(`<p>foo</p>`, `<img src="bar">`)
		assert.Equal(t, `<del class="removed-code"><p>foo</p></del><ins class="added-code"><img src="bar"/></ins>`, string(out))
	})

	// The cases below use the actual HTML shapes produced by the Markdown renderer for
	// fenced code blocks, math, and mermaid diagrams, to confirm that the rich diff
	// produces word-level inline highlighting inside them. See modules/markup/markdown.

	t.Run("code block body change", func(t *testing.T) {
		oldH := `<div class="code-block-container code-overflow-scroll"><pre class="code-block"><code class="chroma language-js display">let x = 1;</code></pre></div>`
		newH := `<div class="code-block-container code-overflow-scroll"><pre class="code-block"><code class="chroma language-js display">let x = 2;</code></pre></div>`
		want := `<div class="code-block-container code-overflow-scroll"><pre class="code-block"><code class="chroma language-js display">let x = <span class="removed-code">1</span><span class="added-code">2</span>;</code></pre></div>`
		assert.Equal(t, want, string(HTMLDiff(template.HTML(oldH), template.HTML(newH))))
	})

	t.Run("code block word insertion", func(t *testing.T) {
		oldH := `<div class="code-block-container code-overflow-scroll"><pre class="code-block"><code class="chroma language-js display">let x = 1;</code></pre></div>`
		newH := `<div class="code-block-container code-overflow-scroll"><pre class="code-block"><code class="chroma language-js display">let x = 1; // note</code></pre></div>`
		want := `<div class="code-block-container code-overflow-scroll"><pre class="code-block"><code class="chroma language-js display">let x = 1;<span class="added-code"> // note</span></code></pre></div>`
		assert.Equal(t, want, string(HTMLDiff(template.HTML(oldH), template.HTML(newH))))
	})

	t.Run("inline math body change", func(t *testing.T) {
		// Inline math `$a+b$` renders as <code class="language-math">a+b</code>.
		out := HTMLDiff(
			`<p>when <code class="language-math">a+b</code> holds</p>`,
			`<p>when <code class="language-math">a+c</code> holds</p>`,
		)
		assert.Equal(t, `<p>when <code class="language-math">a+<span class="removed-code">b</span><span class="added-code">c</span></code> holds</p>`, string(out))
	})

	t.Run("block math body change", func(t *testing.T) {
		// Block math `$$...$$` renders into an is-loading pre that is MathJax'd client-side,
		// so the diff only sees the raw LaTeX source. That is still useful: a word-level diff
		// of the source shows up in place, and the client renderer re-runs on the final HTML.
		oldH := `<pre class="code-block is-loading"><code class="language-math display">a + b</code></pre>`
		newH := `<pre class="code-block is-loading"><code class="language-math display">a + c</code></pre>`
		want := `<pre class="code-block is-loading"><code class="language-math display">a + <span class="removed-code">b</span><span class="added-code">c</span></code></pre>`
		assert.Equal(t, want, string(HTMLDiff(template.HTML(oldH), template.HTML(newH))))
	})

	// The cases below exercise the block-level outer pass: structural changes
	// that the single-pass word-level diff would render as a wall of red/green.

	t.Run("unchanged blocks pass through", func(t *testing.T) {
		// Only the second paragraph is edited; the heading and third paragraph must
		// be emitted verbatim with no diff wrappers.
		out := HTMLDiff(
			`<h1>Title</h1><p>intro text</p><p>body content</p>`,
			`<h1>Title</h1><p>intro text here</p><p>body content</p>`,
		)
		assert.Equal(t, `<h1>Title</h1><p>intro text<span class="added-code"> here</span></p><p>body content</p>`, string(out))
	})

	t.Run("reordered paragraphs", func(t *testing.T) {
		// The two paragraphs are swapped; the block-level outer pass should match
		// the unchanged "second paragraph" block in place and emit the other as a
		// delete+insert pair rather than marking both as changed.
		out := HTMLDiff(
			`<p>first paragraph</p><p>second paragraph</p>`,
			`<p>second paragraph</p><p>first paragraph</p>`,
		)
		assert.Equal(t, `<del class="removed-code"><p>first paragraph</p></del><p>second paragraph</p><ins class="added-code"><p>first paragraph</p></ins>`, string(out))
	})

	t.Run("paragraph replaced by list is a whole-block change", func(t *testing.T) {
		// A <p>/<ul> pair has no sensible word-level diff — pairing them would
		// produce broken nesting. The root-tag guard must fall back to whole-block
		// delete+insert in this case.
		out := HTMLDiff(`<p>item one and item two</p>`, `<ul><li>item one</li><li>item two</li></ul>`)
		assert.Equal(t, `<del class="removed-code"><p>item one and item two</p></del><ins class="added-code"><ul><li>item one</li><li>item two</li></ul></ins>`, string(out))
	})

	t.Run("mermaid body change", func(t *testing.T) {
		// Mermaid ```mermaid blocks render to an is-loading code block whose body is the raw
		// diagram source; the browser replaces it with the rendered SVG at load time. The diff
		// therefore shows inline edits in the source, not a visual SVG diff.
		oldH := `<div class="code-block-container code-overflow-scroll"><pre class="code-block is-loading"><code class="chroma language-mermaid display">graph TD; A--&gt;B</code></pre></div>`
		newH := `<div class="code-block-container code-overflow-scroll"><pre class="code-block is-loading"><code class="chroma language-mermaid display">graph TD; A--&gt;C</code></pre></div>`
		want := `<div class="code-block-container code-overflow-scroll"><pre class="code-block is-loading"><code class="chroma language-mermaid display">graph TD; A--&gt;<span class="removed-code">B</span><span class="added-code">C</span></code></pre></div>`
		assert.Equal(t, want, string(HTMLDiff(template.HTML(oldH), template.HTML(newH))))
	})
}
