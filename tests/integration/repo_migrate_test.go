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

// TestRepoMigrateForeignSSHKeyOwner ensures a user cannot authenticate a
// migration with another account's managed SSH key.
func TestRepoMigrateForeignSSHKeyOwner(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", structs.PlainGitService))
	htmlDoc := NewHTMLParser(t, session.MakeRequest(t, req, http.StatusOK).Body)
	link, _ := htmlDoc.doc.Find("form.ui.form").Attr("action")
	uid, _ := htmlDoc.doc.Find("#uid").Attr("value")

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"clone_addr":       "https://gitea.com/gitea/test_repo.git",
		"uid":              uid,
		"repo_name":        "foreign-ssh-key",
		"service":          fmt.Sprintf("%d", structs.PlainGitService),
		"ssh_key_owner_id": "1", // user1, neither the doer nor the migration target
	})
	// rejected before any migration work: the form is re-rendered, not redirected
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, NewHTMLParser(t, resp.Body).doc.Find(".ui.negative.message").Text(),
		"You can only authenticate with your own managed SSH key")
}
