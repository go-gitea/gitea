// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestView(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("RenderFileSVGIsInImgTag", testRenderFileSVGIsInImgTag)
	t.Run("CommitListActions", testCommitListActions)
	t.Run("SecurityHeadersDefaults", testSecurityHeadersDefaults)
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
