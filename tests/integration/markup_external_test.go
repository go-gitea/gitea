// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/external"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalMarkupRenderer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	if !setting.Database.Type.IsSQLite3() {
		t.Skip()
		return
	}

	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		_, err := createFile(user2, repo1, "file.no-sanitizer", "master", `any content`)
		require.NoError(t, err)

		t.Run("RenderNoSanitizer", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/file.no-sanitizer")
			resp := MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)
			div := doc.Find("div.file-view")
			data, err := div.Html()
			assert.NoError(t, err)
			assert.Equal(t, `<script>window.alert("hi")</script>`, strings.TrimSpace(data))
		})
	})

	t.Run("RenderContentDirectly", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user30/renderer/src/branch/master/README.html")
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))

		doc := NewHTMLParser(t, resp.Body)
		div := doc.Find("div.file-view")
		data, err := div.Html()
		assert.NoError(t, err)
		assert.Equal(t, "<div>\n\ttest external renderer\n</div>", strings.TrimSpace(data))
	})

	// above tested "no-sanitizer" mode, then we test iframe mode below
	r := markup.GetRendererByFileName("any-file.html").(*external.Renderer)
	defer test.MockVariableValue(&r.RenderContentMode, setting.RenderContentModeIframe)()
	r = markup.GetRendererByFileName("any-file.no-sanitizer").(*external.Renderer)
	defer test.MockVariableValue(&r.RenderContentMode, setting.RenderContentModeIframe)()

	t.Run("RenderContentInIFrame", func(t *testing.T) {
		t.Run("DefaultSandbox", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user30/renderer/src/branch/master/README.html")

			t.Run("ParentPage", func(t *testing.T) {
				respParent := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, "text/html; charset=utf-8", respParent.Header().Get("Content-Type"))

				iframe := NewHTMLParser(t, respParent.Body).Find("iframe.external-render-iframe")
				assert.Empty(t, iframe.AttrOr("src", "")) // src should be empty, "data-src" is used instead

				// default sandbox on parent page
				assert.Equal(t, "allow-scripts allow-popups", iframe.AttrOr("sandbox", ""))
				assert.Equal(t, "/user30/renderer/render/branch/master/README.html", iframe.AttrOr("data-src", ""))
			})
			t.Run("SubPage", func(t *testing.T) {
				req = NewRequest(t, "GET", "/user30/renderer/render/branch/master/README.html")
				respSub := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, "text/html; charset=utf-8", respSub.Header().Get("Content-Type"))

				// default sandbox in sub page response
				assert.Equal(t, "frame-src 'self'; sandbox allow-scripts allow-popups", respSub.Header().Get("Content-Security-Policy"))
				assert.Equal(t, "<script src=\"/assets/js/external-render-iframe.js\"></script><link rel=\"stylesheet\" href=\"/assets/css/external-render-iframe.css\"><div>\n\ttest external renderer\n</div>\n", respSub.Body.String())
			})
		})

		t.Run("NoSanitizerNoSandbox", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/file.no-sanitizer")
			respParent := MakeRequest(t, req, http.StatusOK)
			iframe := NewHTMLParser(t, respParent.Body).Find("iframe.external-render-iframe")
			assert.Equal(t, "/user2/repo1/render/branch/master/file.no-sanitizer", iframe.AttrOr("data-src", ""))

			req = NewRequest(t, "GET", "/user2/repo1/render/branch/master/file.no-sanitizer")
			respSub := MakeRequest(t, req, http.StatusOK)

			// no sandbox (disabled by RENDER_CONTENT_SANDBOX)
			assert.Empty(t, iframe.AttrOr("sandbox", ""))
			assert.Equal(t, "frame-src 'self'", respSub.Header().Get("Content-Security-Policy"))
		})
	})
}
