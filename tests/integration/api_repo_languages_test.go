// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRepoLanguages(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")

		// Request editor page
		req := NewRequest(t, "GET", "/user2/repo1/_new/master/")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body)
		lastCommit := doc.GetInputValueByName("last_commit")
		assert.NotEmpty(t, lastCommit)

		// Save new file to master branch
		req = NewRequestWithValues(t, "POST", "/user2/repo1/_new/master/", map[string]string{
			"_csrf":         doc.GetCSRF(),
			"last_commit":   lastCommit,
			"tree_path":     "test.go",
			"content":       "package main",
			"commit_choice": "direct",
		})
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.NotEmpty(t, test.RedirectURL(resp))

		// let gitea calculate language stats
		time.Sleep(time.Second)

		// Save new file to master branch
		req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/languages")
		resp = MakeRequest(t, req, http.StatusOK)

		var languages map[string]int64
		DecodeJSON(t, resp, &languages)

		assert.InDeltaMapValues(t, map[string]int64{"Go": 12}, languages, 0)
	})
}
