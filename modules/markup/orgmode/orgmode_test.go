// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgmode_test

import (
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/orgmode"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	setting.AppURL = "http://localhost:3000/"
	setting.IsInTesting = true
	os.Exit(m.Run())
}

func TestRender_StandardLinks(t *testing.T) {
	test := func(input, expected string) {
		buffer, err := orgmode.RenderString(markup.NewTestRenderContext(), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test("[[https://google.com/]]",
		`<p><a href="https://google.com/">https://google.com/</a></p>`)
	test("[[ImageLink.svg][The Image Desc]]",
		`<p><a href="ImageLink.svg">The Image Desc</a></p>`)
}

func TestRender_InternalLinks(t *testing.T) {
	test := func(input, expected string) {
		buffer, err := orgmode.RenderString(markup.NewTestRenderContext(), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test("[[file:test.org][Test]]",
		`<p><a href="test.org">Test</a></p>`)
	test("[[./test.org][Test]]",
		`<p><a href="./test.org">Test</a></p>`)
	test("[[test.org][Test]]",
		`<p><a href="test.org">Test</a></p>`)
	test("[[path/to/test.org][Test]]",
		`<p><a href="path/to/test.org">Test</a></p>`)
}

func TestRender_Media(t *testing.T) {
	test := func(input, expected string) {
		buffer, err := orgmode.RenderString(markup.NewTestRenderContext(), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test("[[file:../../.images/src/02/train.jpg]]",
		`<p><img src="../../.images/src/02/train.jpg" alt="../../.images/src/02/train.jpg"></p>`)
	test("[[file:train.jpg]]",
		`<p><img src="train.jpg" alt="train.jpg"></p>`)

	// With description.
	test("[[https://example.com][https://example.com/example.svg]]",
		`<p><a href="https://example.com"><img src="https://example.com/example.svg" alt="https://example.com/example.svg"></a></p>`)
	test("[[https://example.com][pre https://example.com/example.svg post]]",
		`<p><a href="https://example.com">pre <img src="https://example.com/example.svg" alt="https://example.com/example.svg"> post</a></p>`)
	test("[[https://example.com][https://example.com/example.mp4]]",
		`<p><a href="https://example.com"><video src="https://example.com/example.mp4">https://example.com/example.mp4</video></a></p>`)
	test("[[https://example.com][pre https://example.com/example.mp4 post]]",
		`<p><a href="https://example.com">pre <video src="https://example.com/example.mp4">https://example.com/example.mp4</video> post</a></p>`)

	// Without description.
	test("[[https://example.com/example.svg]]",
		`<p><img src="https://example.com/example.svg" alt="https://example.com/example.svg"></p>`)
	test("[[https://example.com/example.mp4]]",
		`<p><video src="https://example.com/example.mp4">https://example.com/example.mp4</video></p>`)

	// test [[LINK][DESCRIPTION]] syntax with "file:" prefix
	test(`[[https://example.com/][file:https://example.com/foo%20bar.svg]]`,
		`<p><a href="https://example.com/"><img src="https://example.com/foo%20bar.svg" alt="https://example.com/foo%20bar.svg"></a></p>`)
	test(`[[file:https://example.com/foo%20bar.svg][Goto Image]]`,
		`<p><a href="https://example.com/foo%20bar.svg">Goto Image</a></p>`)
	test(`[[file:https://example.com/link][https://example.com/image.jpg]]`,
		`<p><a href="https://example.com/link"><img src="https://example.com/image.jpg" alt="https://example.com/image.jpg"></a></p>`)
	test(`[[file:https://example.com/link][file:https://example.com/image.jpg]]`,
		`<p><a href="https://example.com/link"><img src="https://example.com/image.jpg" alt="https://example.com/image.jpg"></a></p>`)
}

func TestRender_Source(t *testing.T) {
	test := func(input, expected string) {
		buffer, err := orgmode.RenderString(markup.NewTestRenderContext(), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test(`#+begin_src c
int a;
#+end_src
`, `<div class="src src-c">
<pre><code class="chroma language-c"><span class="kt">int</span> <span class="n">a</span><span class="p">;</span></code></pre>
</div>`)
}

func TestRender_TocHeaderExtraction(t *testing.T) {
	// Test single level headers
	t.Run("SingleLevel", func(t *testing.T) {
		input := `* Header 1
* Header 2
* Header 3
`
		ctx := markup.NewTestRenderContext()
		_, err := orgmode.RenderString(ctx, input)
		assert.NoError(t, err)
		assert.Len(t, ctx.TocHeadingItems, 3)
		assert.Equal(t, "Header 1", ctx.TocHeadingItems[0].InnerText)
		assert.Equal(t, "Header 2", ctx.TocHeadingItems[1].InnerText)
		assert.Equal(t, "Header 3", ctx.TocHeadingItems[2].InnerText)
		for _, item := range ctx.TocHeadingItems {
			assert.Equal(t, 1, item.HeadingLevel)
		}
	})

	// Test nested headers
	t.Run("NestedHeaders", func(t *testing.T) {
		input := `* Level 1
** Level 2
*** Level 3
** Another Level 2
`
		ctx := markup.NewTestRenderContext()
		_, err := orgmode.RenderString(ctx, input)
		assert.NoError(t, err)
		assert.Len(t, ctx.TocHeadingItems, 4)
		assert.Equal(t, 1, ctx.TocHeadingItems[0].HeadingLevel)
		assert.Equal(t, 2, ctx.TocHeadingItems[1].HeadingLevel)
		assert.Equal(t, 3, ctx.TocHeadingItems[2].HeadingLevel)
		assert.Equal(t, 2, ctx.TocHeadingItems[3].HeadingLevel)
	})

	// Test headers with special characters
	t.Run("SpecialCharacters", func(t *testing.T) {
		input := `* Header with <special> & "characters"
* Another header
`
		ctx := markup.NewTestRenderContext()
		_, err := orgmode.RenderString(ctx, input)
		assert.NoError(t, err)
		assert.Len(t, ctx.TocHeadingItems, 2)
		assert.Equal(t, `Header with <special> & "characters"`, ctx.TocHeadingItems[0].InnerText)
	})

	// Test empty document
	t.Run("EmptyDocument", func(t *testing.T) {
		input := `Just some text without headers.`
		ctx := markup.NewTestRenderContext()
		_, err := orgmode.RenderString(ctx, input)
		assert.NoError(t, err)
		assert.Empty(t, ctx.TocHeadingItems)
	})

	// Test that TocShowInSection is set correctly
	t.Run("TocShowInSection", func(t *testing.T) {
		input := `* Header 1`
		ctx := markup.NewTestRenderContext()
		_, err := orgmode.RenderString(ctx, input)
		assert.NoError(t, err)
		assert.Equal(t, markup.TocShowInSidebar, ctx.TocShowInSection)
	})
}
