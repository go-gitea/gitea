// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

const nl = "\n"

func TestMathRender(t *testing.T) {
	setting.Markdown.MathCodeBlockOptions = setting.MarkdownMathCodeBlockOptions{ParseInlineDollar: true, ParseInlineParentheses: true}
	testcases := []struct {
		testcase string
		expected string
	}{
		{
			"$a$",
			`<p><code class="language-math">a</code></p>` + nl,
		},
		{
			"$ a $",
			`<p><code class="language-math">a</code></p>` + nl,
		},
		{
			"$a$ $b$",
			`<p><code class="language-math">a</code> <code class="language-math">b</code></p>` + nl,
		},
		{
			`\(a\) \(b\)`,
			`<p><code class="language-math">a</code> <code class="language-math">b</code></p>` + nl,
		},
		{
			`$a$.`,
			`<p><code class="language-math">a</code>.</p>` + nl,
		},
		{
			`.$a$`,
			`<p>.$a$</p>` + nl,
		},
		{
			`$a a$b b$`,
			`<p>$a a$b b$</p>` + nl,
		},
		{
			`a a$b b`,
			`<p>a a$b b</p>` + nl,
		},
		{
			`a$b $a a$b b$`,
			`<p>a$b $a a$b b$</p>` + nl,
		},
		{
			"a$x$",
			`<p>a$x$</p>` + nl,
		},
		{
			"$x$a",
			`<p>$x$a</p>` + nl,
		},
		{
			"$a$ ($b$) [$c$] {$d$}",
			`<p><code class="language-math">a</code> (<code class="language-math">b</code>) [$c$] {$d$}</p>` + nl,
		},
		{
			"$$a$$",
			`<p><code class="language-math">a</code></p>` + nl,
		},
		{
			"$$a$$ test",
			`<p><code class="language-math">a</code> test</p>` + nl,
		},
		{
			"test $$a$$",
			`<p>test <code class="language-math">a</code></p>` + nl,
		},
		{
			`foo $x=\$$ bar`,
			`<p>foo <code class="language-math">x=\$</code> bar</p>` + nl,
		},
		{
			`$\text{$b$}$`,
			`<p><code class="language-math">\text{$b$}</code></p>` + nl,
		},
		{
			"a$`b`$c",
			`<p>a<code class="language-math">b</code>c</p>` + nl,
		},
		{
			"a $`b`$ c",
			`<p>a <code class="language-math">b</code> c</p>` + nl,
		},
		{
			"a$``b``$c x$```y```$z",
			`<p>a<code class="language-math">b</code>c x<code class="language-math">y</code>z</p>` + nl,
		},
	}

	for _, test := range testcases {
		t.Run(test.testcase, func(t *testing.T) {
			res, err := RenderString(markup.NewTestRenderContext(), test.testcase)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, string(res))
		})
	}
}

func TestMathRenderBlockIndent(t *testing.T) {
	setting.Markdown.MathCodeBlockOptions = setting.MarkdownMathCodeBlockOptions{ParseBlockDollar: true, ParseBlockSquareBrackets: true}
	testcases := []struct {
		name     string
		testcase string
		expected string
	}{
		{
			"indent-0",
			`
\[
\alpha
\]
`,
			`<pre class="code-block is-loading"><code class="language-math display">
\alpha
</code></pre>
`,
		},
		{
			"indent-1",
			`
 \[
 \alpha
 \]
`,
			`<pre class="code-block is-loading"><code class="language-math display">
\alpha
</code></pre>
`,
		},
		{
			"indent-2-mismatch",
			`
  \[
a
 b
  c
   d
  \]
`,
			`<pre class="code-block is-loading"><code class="language-math display">
a
b
c
 d
</code></pre>
`,
		},
		{
			"indent-2",
			`
  \[
  a
   b
  c
  \]
`,
			`<pre class="code-block is-loading"><code class="language-math display">
a
 b
c
</code></pre>
`,
		},
		{
			"indent-0-oneline",
			`$$ x $$
foo`,
			`<code class="language-math display"> x </code>
<p>foo</p>
`,
		},
		{
			"indent-3-oneline",
			`   $$ x $$<SPACE>
foo`,
			`<code class="language-math display"> x </code>
<p>foo</p>
`,
		},
		{
			"quote-block",
			`
> \[
> a
> \]
> \[
> b
> \]
`,
			`<blockquote>
<pre class="code-block is-loading"><code class="language-math display">
a
</code></pre>
<pre class="code-block is-loading"><code class="language-math display">
b
</code></pre>
</blockquote>
`,
		},
		{
			"list-block",
			`
1. a
   \[
   x
   \]
2. b`,
			`<ol>
<li>a
<pre class="code-block is-loading"><code class="language-math display">
x
</code></pre>
</li>
<li>b</li>
</ol>
`,
		},
		{
			"inline-non-math",
			`\[x]`,
			`<p>[x]</p>` + nl,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			res, err := RenderString(markup.NewTestRenderContext(), strings.ReplaceAll(test.testcase, "<SPACE>", " "))
			assert.NoError(t, err)
			assert.Equal(t, test.expected, string(res), "unexpected result for test case:\n%s", test.testcase)
		})
	}
}

func TestMathRenderOptions(t *testing.T) {
	setting.Markdown.MathCodeBlockOptions = setting.MarkdownMathCodeBlockOptions{}
	defer test.MockVariableValue(&setting.Markdown.MathCodeBlockOptions)
	test := func(t *testing.T, expected, input string) {
		res, err := RenderString(markup.NewTestRenderContext(), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(res)), "input: %s", input)
	}

	// default (non-conflict) inline syntax
	test(t, `<p><code class="language-math">a</code></p>`, "$`a`$")

	// ParseInlineDollar
	test(t, `<p>$a$</p>`, `$a$`)
	setting.Markdown.MathCodeBlockOptions.ParseInlineDollar = true
	test(t, `<p><code class="language-math">a</code></p>`, `$a$`)

	// ParseInlineParentheses
	test(t, `<p>(a)</p>`, `\(a\)`)
	setting.Markdown.MathCodeBlockOptions.ParseInlineParentheses = true
	test(t, `<p><code class="language-math">a</code></p>`, `\(a\)`)

	// ParseBlockDollar
	test(t, `<p>$$
a
$$</p>
`, `
$$
a
$$
`)
	setting.Markdown.MathCodeBlockOptions.ParseBlockDollar = true
	test(t, `<pre class="code-block is-loading"><code class="language-math display">
a
</code></pre>
`, `
$$
a
$$
`)

	// ParseBlockSquareBrackets
	test(t, `<p>[
a
]</p>
`, `
\[
a
\]
`)
	setting.Markdown.MathCodeBlockOptions.ParseBlockSquareBrackets = true
	test(t, `<pre class="code-block is-loading"><code class="language-math display">
a
</code></pre>
`, `
\[
a
\]
`)
}
