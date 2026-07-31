// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func testRepoMigrate(t testing.TB, session *TestSession, cloneAddr, repoName string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", structs.PlainGitService)) // render plain git migration page
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")

	uid, exists := htmlDoc.doc.Find("#uid").Attr("value")
	assert.True(t, exists, "The template has changed")

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"clone_addr": cloneAddr,
		"uid":        uid,
		"repo_name":  repoName,
		"service":    fmt.Sprintf("%d", structs.PlainGitService),
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)

	return resp
}

func TestRepoMigrate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	testRepoMigrate(t, session, "https://github.com/go-gitea/test_repo.git", "git")
}

// TestRepoMigrationUI verifies the per-service migration forms render, including the
// newly added bitbucket.org service so its UI does not regress.
func TestRepoMigrationUI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	for _, service := range []structs.GitServiceType{
		structs.PlainGitService,
		structs.GithubService,
		structs.GitlabService,
		structs.BitbucketService,
	} {
		req := NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", service))
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		serviceValue, exists := htmlDoc.doc.Find(`#service_type`).Attr("value")
		assert.True(t, exists, "service_type input missing for %s", service.Title())
		assert.Equal(t, fmt.Sprintf("%d", service), serviceValue)
	}
}
