// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown_test

import (
	"context"
	"html/template"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

const (
	AppURL            = "http://localhost:3000/"
	testRepoOwnerName = "user13"
	testRepoName      = "repo11"
	FullURL           = AppURL + testRepoOwnerName + "/" + testRepoName + "/"
)

// these values should match the const above
var localMetas = map[string]string{
	"user": testRepoOwnerName,
	"repo": testRepoName,
}

func TestRender_StandardLinks(t *testing.T) {
	test := func(input, expected string) {
		buffer, err := markdown.RenderString(markup.NewTestRenderContext(), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	googleRendered := `<p><a href="https://google.com/" rel="nofollow">https://google.com/</a></p>`
	test("<https://google.com/>", googleRendered)
	test("[Link](Link)", `<p><a href="/Link" rel="nofollow">Link</a></p>`)
}

func TestRender_Images(t *testing.T) {
	setting.AppURL = AppURL

	test := func(input, expected string) {
		buffer, err := markdown.RenderString(markup.NewTestRenderContext(FullURL), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	url := "../../.images/src/02/train.jpg"
	title := "Train"
	href := "https://gitea.io"
	result := util.URLJoin(FullURL, url)
	// hint: With Markdown v2.5.2, there is a new syntax: [link](URL){:target="_blank"} , but we do not support it now

	test(
		"!["+title+"]("+url+")",
		`<p><a href="`+result+`" target="_blank" rel="nofollow noopener"><img src="`+result+`" alt="`+title+`"/></a></p>`)

	test(
		"[["+title+"|"+url+"]]",
		`<p><a href="`+result+`" rel="nofollow"><img src="`+result+`" title="`+title+`" alt="`+title+`"/></a></p>`)
	test(
		"[!["+title+"]("+url+")]("+href+")",
		`<p><a href="`+href+`" rel="nofollow"><img src="`+result+`" alt="`+title+`"/></a></p>`)

	test(
		"!["+title+"]("+url+")",
		`<p><a href="`+result+`" target="_blank" rel="nofollow noopener"><img src="`+result+`" alt="`+title+`"/></a></p>`)

	test(
		"[["+title+"|"+url+"]]",
		`<p><a href="`+result+`" rel="nofollow"><img src="`+result+`" title="`+title+`" alt="`+title+`"/></a></p>`)
	test(
		"[!["+title+"]("+url+")]("+href+")",
		`<p><a href="`+href+`" rel="nofollow"><img src="`+result+`" alt="`+title+`"/></a></p>`)
}

func TestTotal_RenderString(t *testing.T) {
	defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()

	// Test cases without ambiguous links (It is not right to copy a whole file here, instead it should clearly test what is being tested)
	sameCases := []string{
		// dear imgui wiki markdown extract: special wiki syntax
		`Wiki! Enjoy :)
- [[Links, Language bindings, Engine bindings|Links]]
- [[Tips]]

See commit 65f1bf27bc

Ideas and codes

- Bezier widget (by @r-lyeh) ` + AppURL + `ocornut/imgui/issues/786
- Bezier widget (by @r-lyeh) ` + FullURL + `issues/786
- Node graph editors https://github.com/ocornut/imgui/issues/306
- [[Memory Editor|memory_editor_example]]
- [[Plot var helper|plot_var_example]]`,
		// wine-staging wiki home extract: tables, special wiki syntax, images
		`## What is Wine Staging?
**Wine Staging** on website [wine-staging.com](http://wine-staging.com).

## Quick Links
Here are some links to the most important topics. You can find the full list of pages at the sidebar.

| [[images/icon-install.png]]    | [[Installation]]                                         |
|--------------------------------|----------------------------------------------------------|
| [[images/icon-usage.png]]      | [[Usage]]                                                |
`,
		// libgdx wiki page: inline images with special syntax
		`[Excelsior JET](http://www.excelsiorjet.com/) allows you to create native executables for Windows, Linux and Mac OS X.

1. [Package your libGDX application](https://github.com/libgdx/libgdx/wiki/Gradle-on-the-Commandline#packaging-for-the-desktop)
[[images/1.png]]
2. Perform a test run by hitting the Run! button.
[[images/2.png]]

## More tests {#custom-id}

(from https://www.markdownguide.org/extended-syntax/)

### Checkboxes

- [ ] unchecked
- [x] checked
- [ ] still unchecked

### Definition list

First Term
: This is the definition of the first term.

Second Term
: This is one definition of the second term.
: This is another definition of the second term.

### Footnotes

Here is a simple footnote,[^1] and here is a longer one.[^bignote]

[^1]: This is the first footnote.

[^bignote]: Here is one with multiple paragraphs and code.

    Indent paragraphs to include them in the footnote.

    ` + "`{ my code }`" + `

    Add as many paragraphs as you like.
`,
		`
- [ ] <!-- rebase-check --> If you want to rebase/retry this PR, click this checkbox.

---

This PR has been generated by [Renovate Bot](https://github.com/renovatebot/renovate).

<!-- test-comment -->`,
	}

	baseURL := ""
	testAnswers := []string{
		`<p>Wiki! Enjoy :)</p>
<ul>
<li><a href="` + baseURL + `/Links" rel="nofollow">Links, Language bindings, Engine bindings</a></li>
<li><a href="` + baseURL + `/Tips" rel="nofollow">Tips</a></li>
</ul>
<p>See commit <a href="/` + testRepoOwnerName + `/` + testRepoName + `/commit/65f1bf27bc" rel="nofollow"><code>65f1bf27bc</code></a></p>
<p>Ideas and codes</p>
<ul>
<li>Bezier widget (by <a href="/r-lyeh" rel="nofollow">@r-lyeh</a>) <a href="http://localhost:3000/ocornut/imgui/issues/786" class="ref-issue" rel="nofollow">ocornut/imgui#786</a></li>
<li>Bezier widget (by <a href="/r-lyeh" rel="nofollow">@r-lyeh</a>) <a href="` + FullURL + `issues/786" class="ref-issue" rel="nofollow">#786</a></li>
<li>Node graph editors <a href="https://github.com/ocornut/imgui/issues/306" rel="nofollow">https://github.com/ocornut/imgui/issues/306</a></li>
<li><a href="` + baseURL + `/memory_editor_example" rel="nofollow">Memory Editor</a></li>
<li><a href="` + baseURL + `/plot_var_example" rel="nofollow">Plot var helper</a></li>
</ul>
`,
		`<h2 id="user-content-what-is-wine-staging">What is Wine Staging?</h2>
<p><strong>Wine Staging</strong> on website <a href="http://wine-staging.com" rel="nofollow">wine-staging.com</a>.</p>
<h2 id="user-content-quick-links">Quick Links</h2>
<p>Here are some links to the most important topics. You can find the full list of pages at the sidebar.</p>
<table>
<thead>
<tr>
<th><a href="` + baseURL + `/images/icon-install.png" rel="nofollow"><img src="` + baseURL + `/images/icon-install.png" title="icon-install.png" alt="images/icon-install.png"/></a></th>
<th><a href="` + baseURL + `/Installation" rel="nofollow">Installation</a></th>
</tr>
</thead>
<tbody>
<tr>
<td><a href="` + baseURL + `/images/icon-usage.png" rel="nofollow"><img src="` + baseURL + `/images/icon-usage.png" title="icon-usage.png" alt="images/icon-usage.png"/></a></td>
<td><a href="` + baseURL + `/Usage" rel="nofollow">Usage</a></td>
</tr>
</tbody>
</table>
`,
		`<p><a href="http://www.excelsiorjet.com/" rel="nofollow">Excelsior JET</a> allows you to create native executables for Windows, Linux and Mac OS X.</p>
<ol>
<li><a href="https://github.com/libgdx/libgdx/wiki/Gradle-on-the-Commandline#packaging-for-the-desktop" rel="nofollow">Package your libGDX application</a>
<a href="` + baseURL + `/images/1.png" rel="nofollow"><img src="` + baseURL + `/images/1.png" title="1.png" alt="images/1.png"/></a></li>
<li>Perform a test run by hitting the Run! button.
<a href="` + baseURL + `/images/2.png" rel="nofollow"><img src="` + baseURL + `/images/2.png" title="2.png" alt="images/2.png"/></a></li>
</ol>
<h2 id="user-content-custom-id">More tests</h2>
<p>(from <a href="https://www.markdownguide.org/extended-syntax/" rel="nofollow">https://www.markdownguide.org/extended-syntax/</a>)</p>
<h3 id="user-content-checkboxes">Checkboxes</h3>
<ul>
<li class="task-list-item"><input type="checkbox" disabled="" data-source-position="434"/>unchecked</li>
<li class="task-list-item"><input type="checkbox" disabled="" data-source-position="450" checked=""/>checked</li>
<li class="task-list-item"><input type="checkbox" disabled="" data-source-position="464"/>still unchecked</li>
</ul>
<h3 id="user-content-definition-list">Definition list</h3>
<dl>
<dt>First Term</dt>
<dd>This is the definition of the first term.</dd>
<dt>Second Term</dt>
<dd>This is one definition of the second term.</dd>
<dd>This is another definition of the second term.</dd>
</dl>
<h3 id="user-content-footnotes">Footnotes</h3>
<p>Here is a simple footnote,<sup id="fnref:user-content-1"><a href="#fn:user-content-1" rel="nofollow">1</a></sup> and here is a longer one.<sup id="fnref:user-content-bignote"><a href="#fn:user-content-bignote" rel="nofollow">2</a></sup></p>
<div>
<hr/>
<ol>
<li id="fn:user-content-1">
<p>This is the first footnote. <a href="#fnref:user-content-1" rel="nofollow">↩︎</a></p>
</li>
<li id="fn:user-content-bignote">
<p>Here is one with multiple paragraphs and code.</p>
<p>Indent paragraphs to include them in the footnote.</p>
<p><code>{ my code }</code></p>
<p>Add as many paragraphs as you like. <a href="#fnref:user-content-bignote" rel="nofollow">↩︎</a></p>
</li>
</ol>
</div>
`,
		`<ul>
<li class="task-list-item"><input type="checkbox" disabled="" data-source-position="3"/> If you want to rebase/retry this PR, click this checkbox.</li>
</ul>
<hr/>
<p>This PR has been generated by <a href="https://github.com/renovatebot/renovate" rel="nofollow">Renovate Bot</a>.</p>
`,
	}

	markup.Init(&markup.RenderHelperFuncs{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			return username == "r-lyeh"
		},
	})
	for i := 0; i < len(sameCases); i++ {
		line, err := markdown.RenderString(markup.NewTestRenderContext(localMetas), sameCases[i])
		assert.NoError(t, err)
		assert.Equal(t, testAnswers[i], string(line))
	}
}

func TestRender_RenderParagraphs(t *testing.T) {
	test := func(t *testing.T, str string, cnt int) {
		res, err := markdown.RenderRawString(markup.NewTestRenderContext(), str)
		assert.NoError(t, err)
		assert.Equal(t, cnt, strings.Count(res, "<p"), "Rendered result for unix should have %d paragraph(s) but has %d:\n%s\n", cnt, strings.Count(res, "<p"), res)

		mac := strings.ReplaceAll(str, "\n", "\r")
		res, err = markdown.RenderRawString(markup.NewTestRenderContext(), mac)
		assert.NoError(t, err)
		assert.Equal(t, cnt, strings.Count(res, "<p"), "Rendered result for mac should have %d paragraph(s) but has %d:\n%s\n", cnt, strings.Count(res, "<p"), res)

		dos := strings.ReplaceAll(str, "\n", "\r\n")
		res, err = markdown.RenderRawString(markup.NewTestRenderContext(), dos)
		assert.NoError(t, err)
		assert.Equal(t, cnt, strings.Count(res, "<p"), "Rendered result for windows should have %d paragraph(s) but has %d:\n%s\n", cnt, strings.Count(res, "<p"), res)
	}

	test(t, "\nOne\nTwo\nThree", 1)
	test(t, "\n\nOne\nTwo\nThree", 1)
	test(t, "\n\nOne\nTwo\nThree\n\n\n", 1)
	test(t, "A\n\nB\nC\n", 2)
	test(t, "A\n\n\nB\nC\n", 2)
}

func TestMarkdownRenderRaw(t *testing.T) {
	testcases := [][]byte{
		{ // clusterfuzz_testcase_minimized_fuzz_markdown_render_raw_6267570554535936
			0x2a, 0x20, 0x2d, 0x0a, 0x09, 0x20, 0x60, 0x5b, 0x0a, 0x09, 0x20, 0x60,
			0x5b,
		},
		{ // clusterfuzz_testcase_minimized_fuzz_markdown_render_raw_6278827345051648
			0x2d, 0x20, 0x2d, 0x0d, 0x09, 0x60, 0x0d, 0x09, 0x60,
		},
		{ // clusterfuzz_testcase_minimized_fuzz_markdown_render_raw_6016973788020736[] = {
			0x7b, 0x63, 0x6c, 0x61, 0x73, 0x73, 0x3d, 0x35, 0x7d, 0x0a, 0x3d,
		},
	}

	for _, testcase := range testcases {
		log.Info("Test markdown render error with fuzzy data: %x, the following errors can be recovered", testcase)
		_, err := markdown.RenderRawString(markup.NewTestRenderContext(), string(testcase))
		assert.NoError(t, err)
	}
}

func TestRenderSiblingImages_Issue12925(t *testing.T) {
	testcase := `![image1](/image1)
![image2](/image2)
`
	expected := `<p><a href="/image1" target="_blank" rel="nofollow noopener"><img src="/image1" alt="image1"></a>
<a href="/image2" target="_blank" rel="nofollow noopener"><img src="/image2" alt="image2"></a></p>
`
	res, err := markdown.RenderRawString(markup.NewTestRenderContext(), testcase)
	assert.NoError(t, err)
	assert.Equal(t, expected, res)
}

func TestRenderEmojiInLinks_Issue12331(t *testing.T) {
	testcase := `[Link with emoji :moon: in text](https://gitea.io)`
	expected := `<p><a href="https://gitea.io" rel="nofollow">Link with emoji <span class="emoji" aria-label="waxing gibbous moon">🌔</span> in text</a></p>
`
	res, err := markdown.RenderString(markup.NewTestRenderContext(), testcase)
	assert.NoError(t, err)
	assert.Equal(t, template.HTML(expected), res)
}

func TestColorPreview(t *testing.T) {
	const nl = "\n"
	positiveTests := []struct {
		testcase string
		expected string
	}{
		{ // do not render color names
			"The CSS class `red` is there",
			"<p>The CSS class <code>red</code> is there</p>\n",
		},
		{ // hex
			"`#FF0000`",
			`<p><code>#FF0000<span class="color-preview" style="background-color: #FF0000"></span></code></p>` + nl,
		},
		{ // rgb
			"`rgb(16, 32, 64)`",
			`<p><code>rgb(16, 32, 64)<span class="color-preview" style="background-color: rgb(16, 32, 64)"></span></code></p>` + nl,
		},
		{ // short hex
			"This is the color white `#0a0`",
			`<p>This is the color white <code>#0a0<span class="color-preview" style="background-color: #0a0"></span></code></p>` + nl,
		},
		{ // hsl
			"HSL stands for hue, saturation, and lightness. An example: `hsl(0, 100%, 50%)`.",
			`<p>HSL stands for hue, saturation, and lightness. An example: <code>hsl(0, 100%, 50%)<span class="color-preview" style="background-color: hsl(0, 100%, 50%)"></span></code>.</p>` + nl,
		},
		{ // uppercase hsl
			"HSL stands for hue, saturation, and lightness. An example: `HSL(0, 100%, 50%)`.",
			`<p>HSL stands for hue, saturation, and lightness. An example: <code>HSL(0, 100%, 50%)<span class="color-preview" style="background-color: HSL(0, 100%, 50%)"></span></code>.</p>` + nl,
		},
	}

	for _, test := range positiveTests {
		res, err := markdown.RenderString(markup.NewTestRenderContext(), test.testcase)
		assert.NoError(t, err, "Unexpected error in testcase: %q", test.testcase)
		assert.Equal(t, template.HTML(test.expected), res, "Unexpected result in testcase %q", test.testcase)
	}

	negativeTests := []string{
		// not a color code
		"`FF0000`",
		// inside a code block
		"```javascript" + nl + `const red = "#FF0000";` + nl + "```",
		// no backticks
		"rgb(166, 32, 64)",
		// typo
		"`hsI(0, 100%, 50%)`",
		// looks like a color but not really
		"`hsl(40, 60, 80)`",
	}

	for _, test := range negativeTests {
		res, err := markdown.RenderString(markup.NewTestRenderContext(), test)
		assert.NoError(t, err, "Unexpected error in testcase: %q", test)
		assert.NotContains(t, res, `<span class="color-preview" style="background-color: `, "Unexpected result in testcase %q", test)
	}
}

func TestTaskList(t *testing.T) {
	testcases := []struct {
		testcase string
		expected string
	}{
		{
			// data-source-position should take into account YAML frontmatter.
			`---
foo: bar
---
- [ ] task 1`,
			`<details><summary><i class="icon table"></i></summary><table>
<thead>
<tr>
<th>foo</th>
</tr>
</thead>
<tbody>
<tr>
<td>bar</td>
</tr>
</tbody>
</table>
</details><ul>
<li class="task-list-item"><input type="checkbox" disabled="" data-source-position="19"/>task 1</li>
</ul>
`,
		},
	}

	for _, test := range testcases {
		res, err := markdown.RenderString(markup.NewTestRenderContext(), test.testcase)
		assert.NoError(t, err, "Unexpected error in testcase: %q", test.testcase)
		assert.Equal(t, template.HTML(test.expected), res, "Unexpected result in testcase %q", test.testcase)
	}
}

func TestRenderLinks(t *testing.T) {
	input := `  space @mention-user${SPACE}${SPACE}
/just/a/path.bin
https://example.com/file.bin
[local link](file.bin)
[remote link](https://example.com)
[[local link|file.bin]]
[[remote link|https://example.com]]
![local image](image.jpg)
![local image](path/file)
![local image](/path/file)
![remote image](https://example.com/image.jpg)
[[local image|image.jpg]]
[[remote link|https://example.com/image.jpg]]
https://example.com/user/repo/compare/88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb#hash
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb pare
https://example.com/user/repo/commit/88fc37a3c0a4dda553bdcfc80c178a58247f42fb
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb mit
:+1:
mail@domain.com
@mention-user test
#123
  space${SPACE}${SPACE}
`
	input = strings.ReplaceAll(input, "${SPACE}", " ") // replace ${SPACE} with " ", to avoid some editor's auto-trimming
	expected := `<p>space @mention-user<br/>
/just/a/path.bin
<a href="https://example.com/file.bin" rel="nofollow">https://example.com/file.bin</a>
<a href="/file.bin" rel="nofollow">local link</a>
<a href="https://example.com" rel="nofollow">remote link</a>
<a href="/file.bin" rel="nofollow">local link</a>
<a href="https://example.com" rel="nofollow">remote link</a>
<a href="/image.jpg" target="_blank" rel="nofollow noopener"><img src="/image.jpg" alt="local image"/></a>
<a href="/path/file" target="_blank" rel="nofollow noopener"><img src="/path/file" alt="local image"/></a>
<a href="/path/file" target="_blank" rel="nofollow noopener"><img src="/path/file" alt="local image"/></a>
<a href="https://example.com/image.jpg" target="_blank" rel="nofollow noopener"><img src="https://example.com/image.jpg" alt="remote image"/></a>
<a href="/image.jpg" rel="nofollow"><img src="/image.jpg" title="local image" alt="local image"/></a>
<a href="https://example.com/image.jpg" rel="nofollow"><img src="https://example.com/image.jpg" title="remote link" alt="remote link"/></a>
<a href="https://example.com/user/repo/compare/88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb#hash" rel="nofollow"><code>88fc37a3c0...12fc37a3c0 (hash)</code></a>
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb...12fc37a3c0a4dda553bdcfc80c178a58247f42fb pare
<a href="https://example.com/user/repo/commit/88fc37a3c0a4dda553bdcfc80c178a58247f42fb" rel="nofollow"><code>88fc37a3c0</code></a>
com 88fc37a3c0a4dda553bdcfc80c178a58247f42fb mit
<span class="emoji" aria-label="thumbs up">👍</span>
<a href="mailto:mail@domain.com" rel="nofollow">mail@domain.com</a>
@mention-user test
#123
space</p>
`
	defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	result, err := markdown.RenderString(markup.NewTestRenderContext(localMetas), input)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(result))
}
