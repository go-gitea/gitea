// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	go_context "context"
	"io"
	"net/http"
	"path"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"

func testRenderMarkup(t *testing.T, mode string, wiki bool, filePath, text, expectedBody string, expectedCode int) {
	setting.AppURL = AppURL
	context := "/gogits/gogs"
	if !wiki {
		context += path.Join("/src/branch/main", path.Dir(filePath))
	}
	options := api.MarkupOption{
		Mode:     mode,
		Text:     text,
		Context:  context,
		Wiki:     wiki,
		FilePath: filePath,
	}
	ctx, resp := contexttest.MockAPIContext(t, "POST /api/v1/markup")
	web.SetForm(ctx, &options)
	Markup(ctx)
	actual := strings.ReplaceAll(resp.Body.String(), ` data-markdown-generated-content=""`, "")
	assert.Equal(t, expectedBody, actual)
	assert.Equal(t, expectedCode, resp.Code)
	resp.Body.Reset()
}

func testRenderMarkdown(t *testing.T, mode string, wiki bool, text, responseBody string, responseCode int) {
	setting.AppURL = AppURL
	context := "/gogits/gogs"
	if !wiki {
		context += "/src/branch/main"
	}
	options := api.MarkdownOption{
		Mode:    mode,
		Text:    text,
		Context: context,
		Wiki:    wiki,
	}
	ctx, resp := contexttest.MockAPIContext(t, "POST /api/v1/markdown")
	web.SetForm(ctx, &options)
	Markdown(ctx)
	actual := strings.ReplaceAll(resp.Body.String(), ` data-markdown-generated-content=""`, "")
	assert.Equal(t, responseBody, actual)
	assert.Equal(t, responseCode, resp.Code)
	resp.Body.Reset()
}

func TestAPI_RenderGFM(t *testing.T) {
	markup.Init(&markup.ProcessorHelper{
		IsUsernameMentionable: func(ctx go_context.Context, username string) bool {
			return username == "r-lyeh"
		},
	})

	testCasesWiki := []string{
		// dear imgui wiki markdown extract: special wiki syntax
		`Wiki! Enjoy :)
- [[Links, Language bindings, Engine bindings|Links]]
- [[Tips]]
- Bezier widget (by @r-lyeh) https://github.com/ocornut/imgui/issues/786`,
		// rendered
		`<p>Wiki! Enjoy :)</p>
<ul>
<li><a href="http://localhost:3000/gogits/gogs/wiki/Links" rel="nofollow">Links, Language bindings, Engine bindings</a></li>
<li><a href="http://localhost:3000/gogits/gogs/wiki/Tips" rel="nofollow">Tips</a></li>
<li>Bezier widget (by <a href="http://localhost:3000/r-lyeh" rel="nofollow">@r-lyeh</a>) <a href="https://github.com/ocornut/imgui/issues/786" rel="nofollow">https://github.com/ocornut/imgui/issues/786</a></li>
</ul>
`,
		// Guard wiki sidebar: special syntax
		`[[Guardfile-DSL / Configuring-Guard|Guardfile-DSL---Configuring-Guard]]`,
		// rendered
		`<p><a href="http://localhost:3000/gogits/gogs/wiki/Guardfile-DSL---Configuring-Guard" rel="nofollow">Guardfile-DSL / Configuring-Guard</a></p>
`,
		// special syntax
		`[[Name|Link]]`,
		// rendered
		`<p><a href="http://localhost:3000/gogits/gogs/wiki/Link" rel="nofollow">Name</a></p>
`,
		// empty
		``,
		// rendered
		``,
	}

	testCasesWikiDocument := []string{
		// wine-staging wiki home extract: special wiki syntax, images
		`## What is Wine Staging?
**Wine Staging** on website [wine-staging.com](http://wine-staging.com).

## Quick Links
Here are some links to the most important topics. You can find the full list of pages at the sidebar.

[[Configuration]]
[[images/icon-bug.png]]
`,
		// rendered
		`<h2 id="user-content-what-is-wine-staging">What is Wine Staging?</h2>
<p><strong>Wine Staging</strong> on website <a href="http://wine-staging.com" rel="nofollow">wine-staging.com</a>.</p>
<h2 id="user-content-quick-links">Quick Links</h2>
<p>Here are some links to the most important topics. You can find the full list of pages at the sidebar.</p>
<p><a href="http://localhost:3000/gogits/gogs/wiki/Configuration" rel="nofollow">Configuration</a>
<a href="http://localhost:3000/gogits/gogs/wiki/raw/images/icon-bug.png" rel="nofollow"><img src="http://localhost:3000/gogits/gogs/wiki/raw/images/icon-bug.png" title="icon-bug.png" alt="images/icon-bug.png"/></a></p>
`,
	}

	for i := 0; i < len(testCasesWiki); i += 2 {
		text := testCasesWiki[i]
		response := testCasesWiki[i+1]
		testRenderMarkdown(t, "gfm", true, text, response, http.StatusOK)
		testRenderMarkup(t, "gfm", true, "", text, response, http.StatusOK)
		testRenderMarkdown(t, "comment", true, text, response, http.StatusOK)
		testRenderMarkup(t, "comment", true, "", text, response, http.StatusOK)
		testRenderMarkup(t, "file", true, "path/test.md", text, response, http.StatusOK)
	}

	for i := 0; i < len(testCasesWikiDocument); i += 2 {
		text := testCasesWikiDocument[i]
		response := testCasesWikiDocument[i+1]
		testRenderMarkdown(t, "gfm", true, text, response, http.StatusOK)
		testRenderMarkup(t, "gfm", true, "", text, response, http.StatusOK)
		testRenderMarkup(t, "file", true, "path/test.md", text, response, http.StatusOK)
	}

	input := "[Link](test.md)\n![Image](image.png)"
	testRenderMarkdown(t, "gfm", false, input, `<p><a href="http://localhost:3000/gogits/gogs/src/branch/main/test.md" rel="nofollow">Link</a>
<a href="http://localhost:3000/gogits/gogs/media/branch/main/image.png" target="_blank" rel="nofollow noopener"><img src="http://localhost:3000/gogits/gogs/media/branch/main/image.png" alt="Image"/></a></p>
`, http.StatusOK)

	testRenderMarkdown(t, "gfm", false, input, `<p><a href="http://localhost:3000/gogits/gogs/src/branch/main/test.md" rel="nofollow">Link</a>
<a href="http://localhost:3000/gogits/gogs/media/branch/main/image.png" target="_blank" rel="nofollow noopener"><img src="http://localhost:3000/gogits/gogs/media/branch/main/image.png" alt="Image"/></a></p>
`, http.StatusOK)

	testRenderMarkup(t, "gfm", false, "", input, `<p><a href="http://localhost:3000/gogits/gogs/src/branch/main/test.md" rel="nofollow">Link</a>
<a href="http://localhost:3000/gogits/gogs/media/branch/main/image.png" target="_blank" rel="nofollow noopener"><img src="http://localhost:3000/gogits/gogs/media/branch/main/image.png" alt="Image"/></a></p>
`, http.StatusOK)

	testRenderMarkup(t, "file", false, "path/new-file.md", input, `<p><a href="http://localhost:3000/gogits/gogs/src/branch/main/path/test.md" rel="nofollow">Link</a>
<a href="http://localhost:3000/gogits/gogs/media/branch/main/path/image.png" target="_blank" rel="nofollow noopener"><img src="http://localhost:3000/gogits/gogs/media/branch/main/path/image.png" alt="Image"/></a></p>
`, http.StatusOK)

	testRenderMarkup(t, "file", true, "path/test.unknown", "## Test", "Unsupported render extension: .unknown\n", http.StatusUnprocessableEntity)
	testRenderMarkup(t, "unknown", true, "", "## Test", "Unknown mode: unknown\n", http.StatusUnprocessableEntity)
}

var simpleCases = []string{
	// Guard wiki sidebar: special syntax
	`[[Guardfile-DSL / Configuring-Guard|Guardfile-DSL---Configuring-Guard]]`,
	// rendered
	`<p>[[Guardfile-DSL / Configuring-Guard|Guardfile-DSL---Configuring-Guard]]</p>
`,
	// special syntax
	`[[Name|Link]]`,
	// rendered
	`<p>[[Name|Link]]</p>
`,
	// empty
	``,
	// rendered
	``,
}

func TestAPI_RenderSimple(t *testing.T) {
	setting.AppURL = AppURL
	options := api.MarkdownOption{
		Mode:    "markdown",
		Text:    "",
		Context: "/gogits/gogs",
	}
	ctx, resp := contexttest.MockAPIContext(t, "POST /api/v1/markdown")
	for i := 0; i < len(simpleCases); i += 2 {
		options.Text = simpleCases[i]
		web.SetForm(ctx, &options)
		Markdown(ctx)
		assert.Equal(t, simpleCases[i+1], resp.Body.String())
		resp.Body.Reset()
	}
}

func TestAPI_RenderRaw(t *testing.T) {
	setting.AppURL = AppURL
	ctx, resp := contexttest.MockAPIContext(t, "POST /api/v1/markdown")
	for i := 0; i < len(simpleCases); i += 2 {
		ctx.Req.Body = io.NopCloser(strings.NewReader(simpleCases[i]))
		MarkdownRaw(ctx)
		assert.Equal(t, simpleCases[i+1], resp.Body.String())
		resp.Body.Reset()
	}
}
