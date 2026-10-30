// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func testRepoMigrate(t testing.TB, session *TestSession, cloneAddr, repoName string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", structs.PlainGitService)) // render plain git migration page
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")

	uid, exists := htmlDoc.doc.Find(`input[name="uid"]`).Attr("value")
	assert.True(t, exists, "The template has changed")

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"clone_addr": cloneAddr,
		"uid":        uid,
		"repo_name":  repoName,
		"service":    fmt.Sprintf("%d", structs.PlainGitService),
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestRepoMigrate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	resp := testRepoMigrate(t, session, "https://github.com/go-gitea/test_repo.git", "git")
	assert.Equal(t, "/user2/git", *test.ParseJSONRedirect(resp.Body.Bytes()).Redirect)

	t.Run("ProviderEntries", func(t *testing.T) {
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/repo/migrate"), http.StatusOK)
		entries := NewHTMLParser(t, resp.Body).Find(".migrate-entry-title").FilterFunction(func(_ int, s *goquery.Selection) bool {
			return strings.Contains(s.Text(), "Bitbucket")
		})
		assert.Equal(t, 1, entries.Length())
		assert.Equal(t, "Bitbucket", strings.TrimSpace(entries.Text()))
	})

	t.Run("BitbucketVariants", func(t *testing.T) {
		for _, service := range structs.BitbucketService.Variants() {
			resp := session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", service)), http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			assert.Equal(t, fmt.Sprint(service), htmlDoc.GetInputValueByName("service"))
			assert.Equal(t, 2, htmlDoc.Find(".ui.compact.menu a.item").Length())
			assert.Contains(t, htmlDoc.Find(".ui.compact.menu a.item.active").AttrOr("href", ""), fmt.Sprintf("service_type=%d&", service))
			assert.Equal(t, service == structs.BitbucketServerService, htmlDoc.Find(`input[name="auth_username"]`).Length() == 1)
		}
	})
}
