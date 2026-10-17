// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgmode_test

import (
	"os"
	"strings"
	"testing"

	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/orgmode"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	setting.AppURL = "http://localhost:3000/"
	setting.IsInTesting = true
	os.Exit(m.Run())
}

func testRender(t *testing.T, input, expected string) {
	buffer, err := orgmode.RenderString(markup.NewTestRenderContext(), input)
	assert.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
}

func TestRender_StandardLinks(t *testing.T) {
	testRender(t, "[[https://google.com/]]",
		`<p><a href="https://google.com/">https://google.com/</a></p>`)
	testRender(t, "[[ImageLink.svg][The Image Desc]]",
		`<p><a href="ImageLink.svg">The Image Desc</a></p>`)
}

func TestRender_InternalLinks(t *testing.T) {
	testRender(t, "[[file:test.org][Test]]",
		`<p><a href="test.org">Test</a></p>`)
	testRender(t, "[[./test.org][Test]]",
		`<p><a href="./test.org">Test</a></p>`)
	testRender(t, "[[test.org][Test]]",
		`<p><a href="test.org">Test</a></p>`)
	testRender(t, "[[path/to/test.org][Test]]",
		`<p><a href="path/to/test.org">Test</a></p>`)
}

func TestRender_Media(t *testing.T) {
	testRender(t, "[[file:../../.images/src/02/train.jpg]]",
		`<p><img src="../../.images/src/02/train.jpg" alt="../../.images/src/02/train.jpg"></p>`)
	testRender(t, "[[file:train.jpg]]",
		`<p><img src="train.jpg" alt="train.jpg"></p>`)

	// With description.
	testRender(t, "[[https://example.com][https://example.com/example.svg]]",
		`<p><a href="https://example.com"><img src="https://example.com/example.svg" alt="https://example.com/example.svg"></a></p>`)
	testRender(t, "[[https://example.com][pre https://example.com/example.svg post]]",
		`<p><a href="https://example.com">pre <img src="https://example.com/example.svg" alt="https://example.com/example.svg"> post</a></p>`)
	testRender(t, "[[https://example.com][https://example.com/example.mp4]]",
		`<p><a href="https://example.com"><video src="https://example.com/example.mp4">https://example.com/example.mp4</video></a></p>`)
	testRender(t, "[[https://example.com][pre https://example.com/example.mp4 post]]",
		`<p><a href="https://example.com">pre <video src="https://example.com/example.mp4">https://example.com/example.mp4</video> post</a></p>`)

	// Without description.
	testRender(t, "[[https://example.com/example.svg]]",
		`<p><img src="https://example.com/example.svg" alt="https://example.com/example.svg"></p>`)
	testRender(t, "[[https://example.com/example.mp4]]",
		`<p><video src="https://example.com/example.mp4">https://example.com/example.mp4</video></p>`)

	// test [[LINK][DESCRIPTION]] syntax with "file:" prefix
	testRender(t, `[[https://example.com/][file:https://example.com/foo%20bar.svg]]`,
		`<p><a href="https://example.com/"><img src="https://example.com/foo%20bar.svg" alt="https://example.com/foo%20bar.svg"></a></p>`)
	testRender(t, `[[file:https://example.com/foo%20bar.svg][Goto Image]]`,
		`<p><a href="https://example.com/foo%20bar.svg">Goto Image</a></p>`)
	testRender(t, `[[file:https://example.com/link][https://example.com/image.jpg]]`,
		`<p><a href="https://example.com/link"><img src="https://example.com/image.jpg" alt="https://example.com/image.jpg"></a></p>`)
	testRender(t, `[[file:https://example.com/link][file:https://example.com/image.jpg]]`,
		`<p><a href="https://example.com/link"><img src="https://example.com/image.jpg" alt="https://example.com/image.jpg"></a></p>`)
}

func TestRender_Source(t *testing.T) {
	testRender(t, `#+begin_src c
int a;
#+end_src
`, `<div class="src src-c">
<pre class="code-block"><code class="chroma language-c" data-code-language="c"><span class="kt">int</span> <span class="n">a</span><span class="p">;</span></code></pre>
</div>`)
}

func TestRender_IncludeLink(t *testing.T) {
	testRender(t, `#+INCLUDE: "./other.org" src text`, `<div class="src src-text">
<pre class="code-block"><code class="chroma language-text" data-code-language="text">#+INCLUDE: [[other.org]]</code></pre>
</div>`)
}
