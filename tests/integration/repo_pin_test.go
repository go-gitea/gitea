// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"path"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
)

func TestUserRepoPin(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user2")
		unittest.AssertNotExistsBean(t, &repo_model.Pin{UID: 2, RepoID: 3})
		testUserPinRepo(t, session, "org3", "repo3", true, false)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Pin{UID: 2, RepoID: 3})
		testUserPinRepo(t, session, "org3", "repo3", false, false)
		unittest.AssertNotExistsBean(t, &repo_model.Pin{UID: 2, RepoID: 3})
	})
}

func TestOrgRepoPin(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user2")
		unittest.AssertNotExistsBean(t, &repo_model.Pin{UID: 3, RepoID: 3})
		testUserPinRepo(t, session, "org3", "repo3", true, true)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Pin{UID: 3, RepoID: 3})
		testUserPinRepo(t, session, "org3", "repo3", false, true)
		unittest.AssertNotExistsBean(t, &repo_model.Pin{UID: 3, RepoID: 3})
	})
}

func testUserPinRepo(t *testing.T, session *TestSession, user, repo string, pin, org bool) error {
	var action string
	if pin {
		action = "pin"
	} else {
		action = "unpin"
	}

	if org {
		action += "-org"
	}

	// Get repo page to get the CSRF token
	reqPage := NewRequest(t, "GET", path.Join(user, repo))
	respPage := session.MakeRequest(t, reqPage, http.StatusOK)

	htmlDoc := NewHTMLParser(t, respPage.Body)

	reqPath := path.Join(user, repo, "action", action)
	req := NewRequestWithValues(t, "POST", reqPath, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	return nil
}
