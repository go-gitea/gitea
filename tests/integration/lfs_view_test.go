// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// check that files stored in LFS render properly in the web UI
func TestLFSRender(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// check that a markup file is flagged with "Stored in Git LFS" and shows its text
	t.Run("Markup", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/lfs/src/branch/main/CONTRIBUTING.md")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body).doc

		fileInfo := doc.Find("div.file-info-entry").First().Text()
		assert.Contains(t, fileInfo, "Stored with Git LFS")

		content := doc.Find("div.file-view").Text()
		assert.Contains(t, content, "Testing documents in LFS")
	})

	// check that an image is flagged with "Stored in Git LFS" and renders inline
	t.Run("Image", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/lfs/src/branch/main/jpeg.jpg")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body).doc

		fileInfo := doc.Find("div.file-info-entry").First().Text()
		assert.Contains(t, fileInfo, "Stored with Git LFS")

		src, exists := doc.Find(".file-view img").Attr("src")
		assert.True(t, exists, "The image should be in an <img> tag")
		assert.Equal(t, "/user2/lfs/media/branch/main/jpeg.jpg", src, "The image should use the /media link because it's in LFS")
	})

	// check that a binary file is flagged with "Stored in Git LFS" and renders a /media/ link instead of a /raw/ link
	t.Run("Binary", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/lfs/src/branch/main/crypt.bin")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body).doc

		fileInfo := doc.Find("div.file-info-entry").First().Text()
		assert.Contains(t, fileInfo, "Stored with Git LFS")

		rawLink, exists := doc.Find("div.file-view > div.view-raw > a").Attr("href")
		assert.True(t, exists, "Download link should render instead of content because this is a binary file")
		assert.Equal(t, "/user2/lfs/media/branch/main/crypt.bin", rawLink, "The download link should use the proper /media link because it's in LFS")
	})

	// check that a directory with a README file shows its text
	t.Run("Readme", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/lfs/src/branch/main/subdir")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body).doc

		content := doc.Find("div.file-view").Text()
		assert.Contains(t, content, "Testing READMEs in LFS")
	})
}
