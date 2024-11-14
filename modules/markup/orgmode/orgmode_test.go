// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"

func TestRender_StandardLinks(t *testing.T) {
	setting.AppURL = AppURL

	test := func(input, expected string, isWiki bool) {
		buffer, err := RenderString(&markup.RenderContext{
			Ctx: git.DefaultContext,
			Links: markup.Links{
				Base:       "/relative-path",
				BranchPath: "branch/main",
			},
			ContentMode: util.Iif(isWiki, markup.RenderContentAsWiki, markup.RenderContentAsDefault),
		}, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test("[[https://google.com/]]",
		`<p><a href="https://google.com/">https://google.com/</a></p>`, false)
	test("[[WikiPage][The WikiPage Desc]]",
		`<p><a href="/relative-path/wiki/WikiPage">The WikiPage Desc</a></p>`, true)
	test("[[ImageLink.svg][The Image Desc]]",
		`<p><a href="/relative-path/media/branch/main/ImageLink.svg">The Image Desc</a></p>`, false)
}

func TestRender_InternalLinks(t *testing.T) {
	setting.AppURL = AppURL

	test := func(input, expected string) {
		buffer, err := RenderString(&markup.RenderContext{
			Ctx: git.DefaultContext,
			Links: markup.Links{
				Base:       "/relative-path",
				BranchPath: "branch/main",
			},
		}, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test("[[file:test.org][Test]]",
		`<p><a href="/relative-path/src/branch/main/test.org">Test</a></p>`)
	test("[[./test.org][Test]]",
		`<p><a href="/relative-path/src/branch/main/test.org">Test</a></p>`)
	test("[[test.org][Test]]",
		`<p><a href="/relative-path/src/branch/main/test.org">Test</a></p>`)
	test("[[path/to/test.org][Test]]",
		`<p><a href="/relative-path/src/branch/main/path/to/test.org">Test</a></p>`)
}

func TestRender_Media(t *testing.T) {
	setting.AppURL = AppURL

	test := func(input, expected string) {
		buffer, err := RenderString(&markup.RenderContext{
			Ctx: git.DefaultContext,
			Links: markup.Links{
				Base: "./relative-path",
			},
		}, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test("[[file:../../.images/src/02/train.jpg]]",
		`<p><img src=".images/src/02/train.jpg" alt=".images/src/02/train.jpg" /></p>`)
	test("[[file:train.jpg]]",
		`<p><img src="relative-path/train.jpg" alt="relative-path/train.jpg" /></p>`)

	// With description.
	test("[[https://example.com][https://example.com/example.svg]]",
		`<p><a href="https://example.com"><img src="https://example.com/example.svg" alt="https://example.com/example.svg" /></a></p>`)
	test("[[https://example.com][pre https://example.com/example.svg post]]",
		`<p><a href="https://example.com">pre <img src="https://example.com/example.svg" alt="https://example.com/example.svg" /> post</a></p>`)
	test("[[https://example.com][https://example.com/example.mp4]]",
		`<p><a href="https://example.com"><video src="https://example.com/example.mp4">https://example.com/example.mp4</video></a></p>`)
	test("[[https://example.com][pre https://example.com/example.mp4 post]]",
		`<p><a href="https://example.com">pre <video src="https://example.com/example.mp4">https://example.com/example.mp4</video> post</a></p>`)

	// Without description.
	test("[[https://example.com/example.svg]]",
		`<p><img src="https://example.com/example.svg" alt="https://example.com/example.svg" /></p>`)
	test("[[https://example.com/example.mp4]]",
		`<p><video src="https://example.com/example.mp4">https://example.com/example.mp4</video></p>`)

	// test [[LINK][DESCRIPTION]] syntax with "file:" prefix
	test(`[[https://example.com/][file:https://example.com/foo%20bar.svg]]`,
		`<p><a href="https://example.com/"><img src="https://example.com/foo%20bar.svg" alt="https://example.com/foo%20bar.svg" /></a></p>`)
	test(`[[file:https://example.com/foo%20bar.svg][Goto Image]]`,
		`<p><a href="https://example.com/foo%20bar.svg">Goto Image</a></p>`)
	test(`[[file:https://example.com/link][https://example.com/image.jpg]]`,
		`<p><a href="https://example.com/link"><img src="https://example.com/image.jpg" alt="https://example.com/image.jpg" /></a></p>`)
	test(`[[file:https://example.com/link][file:https://example.com/image.jpg]]`,
		`<p><a href="https://example.com/link"><img src="https://example.com/image.jpg" alt="https://example.com/image.jpg" /></a></p>`)
}

func TestRender_Source(t *testing.T) {
	setting.AppURL = AppURL

	test := func(input, expected string) {
		buffer, err := RenderString(&markup.RenderContext{
			Ctx: git.DefaultContext,
		}, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test(`#+begin_src go
// HelloWorld prints "Hello World"
func HelloWorld() {
	fmt.Println("Hello World")
}
#+end_src
`, `<div class="src src-go">
<pre><code class="chroma language-go"><span class="c1">// HelloWorld prints &#34;Hello World&#34;
</span><span class="c1"></span><span class="kd">func</span> <span class="nf">HelloWorld</span><span class="p">()</span> <span class="p">{</span>
	<span class="nx">fmt</span><span class="p">.</span><span class="nf">Println</span><span class="p">(</span><span class="s">&#34;Hello World&#34;</span><span class="p">)</span>
<span class="p">}</span></code></pre>
</div>`)
}
