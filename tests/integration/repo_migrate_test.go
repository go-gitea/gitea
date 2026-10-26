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
	"github.com/stretchr/testify/require"
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
		variants := structs.BitbucketService.Variants()
		for _, service := range variants {
			resp := session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", service)), http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			serviceValue, _ := htmlDoc.Find(`input[name="service"]`).Attr("value")
			assert.Equal(t, fmt.Sprintf("%d", service), serviceValue)
			items := htmlDoc.Find(".ui.compact.menu a.item")
			require.Equal(t, 2, items.Length())
			for i, variant := range variants {
				href, _ := items.Eq(i).Attr("href")
				assert.Contains(t, href, fmt.Sprintf("service_type=%d&", variant))
				assert.Equal(t, variant == service, items.Eq(i).HasClass("active"))
			}
		}
	})
}
