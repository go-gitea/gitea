// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestView(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("RenderFileSVGIsInImgTag", testRenderFileSVGIsInImgTag)
	t.Run("CommitListActions", testCommitListActions)
	t.Run("SecurityHeadersDefaults", testSecurityHeadersDefaults)
	t.Run("SiteManifest", testSiteManifest)
}

func testRenderFileSVGIsInImgTag(t *testing.T) {
	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo2/src/branch/master/line.svg")
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	src, exists := doc.doc.Find(".file-view img").Attr("src")
	assert.True(t, exists, "The SVG image should be in an <img> tag so that scripts in the SVG are not run")
	assert.Equal(t, "/user2/repo2/raw/branch/master/line.svg", src)
}

func testCommitListActions(t *testing.T) {
	session := loginUser(t, "user2")

	t.Run("WikiRevisionList", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo1/wiki/Home?action=_revision")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, htmlDoc, ".commit-list .copy-commit-id", true)
		AssertHTMLElement(t, htmlDoc, `.commit-list .view-single-diff`, false)
		AssertHTMLElement(t, htmlDoc, `.commit-list .view-commit-path`, false)
	})

	t.Run("RepoCommitList", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		AssertHTMLElement(t, htmlDoc, `.commit-list .copy-commit-id`, true)
		AssertHTMLElement(t, htmlDoc, `.commit-list .view-single-diff`, false)
		AssertHTMLElement(t, htmlDoc, `.commit-list .view-commit-path`, true)
	})

	t.Run("RepoFileHistory", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master/README.md")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		AssertHTMLElement(t, htmlDoc, `.commit-list .copy-commit-id`, true)
		AssertHTMLElement(t, htmlDoc, `.commit-list .view-single-diff`, true)
		AssertHTMLElement(t, htmlDoc, `.commit-list .view-commit-path`, true)
	})
}

func testSecurityHeadersDefaults(t *testing.T) {
	assertSecurityHeaders := func(t *testing.T, uri string) {
		req := NewRequest(t, "GET", uri)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "nosniff", resp.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "SAMEORIGIN", resp.Header().Get("X-Frame-Options"))
	}
	assertSecurityHeaders(t, "/")
	assertSecurityHeaders(t, "/api/v1/version")
	assertSecurityHeaders(t, "/assets/img/favicon.png")
}

func testSiteManifest(t *testing.T) {
	req := NewRequest(t, "GET", "/")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<link rel="manifest" href="/assets/site-manifest.json">`)

	req = NewRequest(t, "GET", "/assets/site-manifest.json")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "application/manifest+json", resp.Header().Get("Content-Type"))

	assetBase := strings.TrimSuffix(setting.AppURL, "/")
	expectedJSON := fmt.Sprintf(`{
		"name": %q,
		"short_name": %q,
		"start_url": %q,
		"icons": [
			{"src": %q, "type": "image/png",     "sizes": "512x512"},
			{"src": %q, "type": "image/svg+xml", "sizes": "512x512"}
		]
	}`,
		setting.AppName,
		setting.AppName,
		setting.AppURL,
		assetBase+"/assets/img/logo.png",
		assetBase+"/assets/img/logo.svg",
	)
	assert.JSONEq(t, expectedJSON, resp.Body.String())
}
