package misc

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"gitea.com/macaron/inject"
	"gitea.com/macaron/macaron"
	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"
const Repo = "gogits/gogs"
const AppSubURL = AppURL + Repo + "/"

func createContext(req *http.Request) (*macaron.Context, *httptest.ResponseRecorder) {
	resp := httptest.NewRecorder()
	c := &macaron.Context{
		Injector: inject.New(),
		Req:      macaron.Request{Request: req},
		Resp:     macaron.NewResponseWriter(req.Method, resp),
		Render:   &macaron.DummyRender{ResponseWriter: resp},
		Data:     make(map[string]interface{}),
	}
	c.Map(c)
	c.Map(req)
	return c, resp
}

func wrap(ctx *macaron.Context) *context.APIContext {
	return &context.APIContext{
		Context: &context.Context{
			Context: ctx,
		},
	}
}

func TestAPI_RenderGFM(t *testing.T) {
	setting.AppURL = AppURL

	options := api.MarkdownOption{
		Mode:    "gfm",
		Text:    "",
		Context: Repo,
		Wiki:    true,
	}
	requrl, _ := url.Parse(util.URLJoin(AppURL, "api", "v1", "markdown"))
	req := &http.Request{
		Method: "POST",
		URL:    requrl,
	}
	m, resp := createContext(req)
	ctx := wrap(m)

	testCases := []string{
		// dear imgui wiki markdown extract: special wiki syntax
		`Wiki! Enjoy :)
- [[Links, Language bindings, Engine bindings|Links]]
- [[Tips]]
- Bezier widget (by @r-lyeh) https://github.com/ocornut/imgui/issues/786`,
		// rendered
		`<p>Wiki! Enjoy :)</p>
<ul>
<li><a href="` + AppSubURL + `wiki/Links" rel="nofollow">Links, Language bindings, Engine bindings</a></li>
<li><a href="` + AppSubURL + `wiki/Tips" rel="nofollow">Tips</a></li>
<li>Bezier widget (by <a href="` + AppURL + `r-lyeh" rel="nofollow">@r-lyeh</a>) <a href="https://github.com/ocornut/imgui/issues/786" rel="nofollow">https://github.com/ocornut/imgui/issues/786</a></li>
</ul>
`,
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
<p><a href="` + AppSubURL + `wiki/Configuration" rel="nofollow">Configuration</a>
<a href="` + AppSubURL + `wiki/raw/images/icon-bug.png" rel="nofollow"><img src="` + AppSubURL + `wiki/raw/images/icon-bug.png" title="icon-bug.png" alt="images/icon-bug.png"/></a></p>
`,
		// Guard wiki sidebar: special syntax
		`[[Guardfile-DSL / Configuring-Guard|Guardfile-DSL---Configuring-Guard]]`,
		// rendered
		`<p><a href="` + AppSubURL + `wiki/Guardfile-DSL---Configuring-Guard" rel="nofollow">Guardfile-DSL / Configuring-Guard</a></p>
`,
		// special syntax
		`[[Name|Link]]`,
		// rendered
		`<p><a href="` + AppSubURL + `wiki/Link" rel="nofollow">Name</a></p>
`,
		// empty
		``,
		// rendered
		``,
	}

	for i := 0; i < len(testCases); i += 2 {
		options.Text = testCases[i]
		Markdown(ctx, options)
		assert.Equal(t, testCases[i+1], resp.Body.String())
		resp.Body.Reset()
	}
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
		Context: Repo,
	}
	requrl, _ := url.Parse(util.URLJoin(AppURL, "api", "v1", "markdown"))
	req := &http.Request{
		Method: "POST",
		URL:    requrl,
	}
	m, resp := createContext(req)
	ctx := wrap(m)

	for i := 0; i < len(simpleCases); i += 2 {
		options.Text = simpleCases[i]
		Markdown(ctx, options)
		assert.Equal(t, simpleCases[i+1], resp.Body.String())
		resp.Body.Reset()
	}
}

func TestAPI_RenderRaw(t *testing.T) {
	setting.AppURL = AppURL

	requrl, _ := url.Parse(util.URLJoin(AppURL, "api", "v1", "markdown"))
	req := &http.Request{
		Method: "POST",
		URL:    requrl,
	}
	m, resp := createContext(req)
	ctx := wrap(m)

	for i := 0; i < len(simpleCases); i += 2 {
		ctx.Req.Request.Body = ioutil.NopCloser(strings.NewReader(simpleCases[i]))
		MarkdownRaw(ctx)
		assert.Equal(t, simpleCases[i+1], resp.Body.String())
		resp.Body.Reset()
	}
}
