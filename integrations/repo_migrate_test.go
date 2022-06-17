// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/modules/structs"

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
		"_csrf":      htmlDoc.GetCSRF(),
		"clone_addr": cloneAddr,
		"uid":        uid,
		"repo_name":  repoName,
		"service":    fmt.Sprintf("%d", structs.PlainGitService),
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)

	return resp
}

func TestRepoMigrate(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")
	testRepoMigrate(t, session, "https://github.com/go-gitea/test_repo.git", "git")
}
