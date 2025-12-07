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
	"code.gitea.io/gitea/modules/charset"
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
		t.Skip("only SQLite3 test config supports external markup renderer")
		return
	}

	const binaryContentPrefix = "any prefix text."
	const binaryContent = binaryContentPrefix + "\xfe\xfe\xfe\x00\xff\xff"
	detectedEncoding, _ := charset.DetectEncoding([]byte(binaryContent))
	assert.NotEqual(t, binaryContent, strings.ToValidUTF8(binaryContent, "?"))
	assert.Equal(t, "ISO-8859-2", detectedEncoding) // even if the binary content can be detected as text encoding, it shouldn't affect the raw rendering

	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		_, err := createFileInBranch(user2, repo1, createFileInBranchOptions{}, map[string]string{
			"test.html":         `<div><any attr="val"><script></script></div>`,
			"html.no-sanitizer": `<script>foo("raw")</script>`,
			"bin.no-sanitizer":  binaryContent,
		})
		require.NoError(t, err)

		t.Run("RenderNoSanitizer", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/html.no-sanitizer")
			resp := MakeRequest(t, req, http.StatusOK)
			div := NewHTMLParser(t, resp.Body).Find("div.file-view")
			data, err := div.Html()
			assert.NoError(t, err)
			assert.Equal(t, `<script>foo("raw")</script>`, strings.TrimSpace(data))

			req = NewRequest(t, "GET", "/user2/repo1/src/branch/master/bin.no-sanitizer")
			resp = MakeRequest(t, req, http.StatusOK)
			div = NewHTMLParser(t, resp.Body).Find("div.file-view")
			data, err = div.Html()
			assert.NoError(t, err)
			assert.Equal(t, strings.ReplaceAll(binaryContent, "\x00", ""), strings.TrimSpace(data)) // HTML template engine removes the null bytes
		})
	})

	t.Run("RenderContentDirectly", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/test.html")
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))

		doc := NewHTMLParser(t, resp.Body)
		div := doc.Find("div.file-view")
		data, err := div.Html()
		assert.NoError(t, err)
		// the content is fully sanitized
		assert.Equal(t, `<div>&lt;script&gt;&lt;/script&gt;</div>`, strings.TrimSpace(data))
	})

	// above tested in-page rendering (no iframe), then we test iframe mode below
	r := markup.GetRendererByFileName("any-file.html").(*external.Renderer)
	defer test.MockVariableValue(&r.RenderContentMode, setting.RenderContentModeIframe)()
	assert.True(t, r.NeedPostProcess())
	r = markup.GetRendererByFileName("any-file.no-sanitizer").(*external.Renderer)
	defer test.MockVariableValue(&r.RenderContentMode, setting.RenderContentModeIframe)()
	assert.False(t, r.NeedPostProcess())

	t.Run("RenderContentInIFrame", func(t *testing.T) {
		t.Run("DefaultSandbox", func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/test.html")

			t.Run("ParentPage", func(t *testing.T) {
				respParent := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, "text/html; charset=utf-8", respParent.Header().Get("Content-Type"))

				iframe := NewHTMLParser(t, respParent.Body).Find("iframe.external-render-iframe")
				assert.Empty(t, iframe.AttrOr("src", "")) // src should be empty, "data-src" is used instead

				// default sandbox on parent page
				assert.Equal(t, "allow-scripts allow-popups", iframe.AttrOr("sandbox", ""))
				assert.Equal(t, "/user2/repo1/render/branch/master/test.html", iframe.AttrOr("data-src", ""))
			})
			t.Run("SubPage", func(t *testing.T) {
				req = NewRequest(t, "GET", "/user2/repo1/render/branch/master/test.html")
				respSub := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, "text/html; charset=utf-8", respSub.Header().Get("Content-Type"))

				// default sandbox in sub page response
				assert.Equal(t, "frame-src 'self'; sandbox allow-scripts allow-popups", respSub.Header().Get("Content-Security-Policy"))
				// FIXME: actually here is a bug (legacy design problem), the "PostProcess" will escape "<script>" tag, but it indeed is the sanitizer's job
				assert.Equal(t, `<script src="/assets/js/external-render-iframe.js"></script><link rel="stylesheet" href="/assets/css/external-render-iframe.css"><div><any attr="val">&lt;script&gt;&lt;/script&gt;</any></div>`, respSub.Body.String())
			})
		})

		t.Run("NoSanitizerNoSandbox", func(t *testing.T) {
			t.Run("BinaryContent", func(t *testing.T) {
				req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/bin.no-sanitizer")
				respParent := MakeRequest(t, req, http.StatusOK)
				iframe := NewHTMLParser(t, respParent.Body).Find("iframe.external-render-iframe")
				assert.Equal(t, "/user2/repo1/render/branch/master/bin.no-sanitizer", iframe.AttrOr("data-src", ""))

				req = NewRequest(t, "GET", "/user2/repo1/render/branch/master/bin.no-sanitizer")
				respSub := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, binaryContent, respSub.Body.String()) // raw content should keep the raw bytes (including invalid UTF-8 bytes), and no "external-render-iframe" helpers

				// no sandbox (disabled by RENDER_CONTENT_SANDBOX)
				assert.Empty(t, iframe.AttrOr("sandbox", ""))
				assert.Equal(t, "frame-src 'self'", respSub.Header().Get("Content-Security-Policy"))
			})

			t.Run("HTMLContentWithExternalRenderIframeHelper", func(t *testing.T) {
				req := NewRequest(t, "GET", "/user2/repo1/render/branch/master/html.no-sanitizer")
				respSub := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, `<script src="/assets/js/external-render-iframe.js"></script><link rel="stylesheet" href="/assets/css/external-render-iframe.css"><script>foo("raw")</script>`, respSub.Body.String())
				assert.Equal(t, "frame-src 'self'", respSub.Header().Get("Content-Security-Policy"))
			})
		})
	})
}
