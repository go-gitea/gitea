// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"path"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestLinksNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	links := []string{
		"/explore/repos",
		"/explore/repos?q=test",
		"/explore/users",
		"/explore/users?q=test",
		"/explore/organizations",
		"/explore/organizations?q=test",
		"/",
		"/user/sign_up",
		"/user/login",
		"/user/forgot_password",
		"/api/swagger",
		"/user2/repo1",
		"/user2/repo1/",
		"/user2/repo1/projects",
		"/user2/repo1/projects/1",
		"/assets/img/404.png",
		"/assets/img/500.png",
		"/.well-known/security.txt",
	}

	for _, link := range links {
		req := NewRequest(t, "GET", link)
		MakeRequest(t, req, http.StatusOK)
	}
}

func TestRedirectsNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	redirects := map[string]string{
		"/user2/repo1/commits/master":                "/user2/repo1/commits/branch/master",
		"/user2/repo1/src/master":                    "/user2/repo1/src/branch/master",
		"/user2/repo1/src/master/file.txt":           "/user2/repo1/src/branch/master/file.txt",
		"/user2/repo1/src/master/directory/file.txt": "/user2/repo1/src/branch/master/directory/file.txt",
		"/user/avatar/Ghost/-1":                      "/assets/img/avatar_default.png",
		"/api/v1/swagger":                            "/api/swagger",
	}
	for link, redirectLink := range redirects {
		req := NewRequest(t, "GET", link)
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assert.EqualValues(t, path.Join(setting.AppSubURL, redirectLink), test.RedirectURL(resp))
	}
}

func TestNoLoginNotExist(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	links := []string{
		"/user5/repo4/projects",
		"/user5/repo4/projects/3",
	}

	for _, link := range links {
		req := NewRequest(t, "GET", link)
		MakeRequest(t, req, http.StatusNotFound)
	}
}

func testLinksAsUser(userName string, t *testing.T) {
	links := []string{
		"/explore/repos",
		"/explore/repos?q=test",
		"/explore/users",
		"/explore/users?q=test",
		"/explore/organizations",
		"/explore/organizations?q=test",
		"/",
		"/user/forgot_password",
		"/api/swagger",
		"/issues",
		"/issues?type=your_repositories&repos=[0]&sort=&state=open",
		"/issues?type=assigned&repos=[0]&sort=&state=open",
		"/issues?type=your_repositories&repos=[0]&sort=&state=closed",
		"/issues?type=assigned&repos=[]&sort=&state=closed",
		"/issues?type=assigned&sort=&state=open",
		"/issues?type=created_by&repos=[1,2]&sort=&state=closed",
		"/issues?type=created_by&repos=[1,2]&sort=&state=open",
		"/pulls",
		"/pulls?type=your_repositories&repos=[2]&sort=&state=open",
		"/pulls?type=assigned&repos=[]&sort=&state=open",
		"/pulls?type=created_by&repos=[0]&sort=&state=open",
		"/pulls?type=your_repositories&repos=[0]&sort=&state=closed",
		"/pulls?type=assigned&repos=[0]&sort=&state=closed",
		"/pulls?type=created_by&repos=[0]&sort=&state=closed",
		"/milestones",
		"/milestones?sort=mostcomplete&state=closed",
		"/milestones?type=your_repositories&sort=mostcomplete&state=closed",
		"/milestones?sort=&repos=[1]&state=closed",
		"/milestones?sort=&repos=[1]&state=open",
		"/milestones?repos=[0]&sort=mostissues&state=open",
		"/notifications",
		"/repo/create",
		"/repo/migrate",
		"/org/create",
		"/user2",
		"/user2?tab=stars",
		"/user2?tab=activity",
		"/user/settings",
		"/user/settings/account",
		"/user/settings/security",
		"/user/settings/security/two_factor/enroll",
		"/user/settings/keys",
		"/user/settings/organization",
		"/user/settings/repos",
	}

	session := loginUser(t, userName)
	for _, link := range links {
		req := NewRequest(t, "GET", link)
		session.MakeRequest(t, req, http.StatusOK)
	}

	reqAPI := NewRequestf(t, "GET", "/api/v1/users/%s/repos", userName)
	respAPI := MakeRequest(t, reqAPI, http.StatusOK)

	var apiRepos []*api.Repository
	DecodeJSON(t, respAPI, &apiRepos)

	repoLinks := []string{
		"",
		"/issues",
		"/pulls",
		"/commits/branch/master",
		"/graph",
		"/settings",
		"/settings/collaboration",
		"/settings/branches",
		"/settings/hooks",
		// FIXME: below links should return 200 but 404 ??
		//"/settings/hooks/git",
		//"/settings/hooks/git/pre-receive",
		//"/settings/hooks/git/update",
		//"/settings/hooks/git/post-receive",
		"/settings/keys",
		"/releases",
		"/releases/new",
		//"/wiki/_pages",
		"/wiki/?action=_new",
		"/activity",
	}

	for _, repo := range apiRepos {
		for _, link := range repoLinks {
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s%s", userName, repo.Name, link))
			session.MakeRequest(t, req, http.StatusOK)
		}
	}
}

func TestLinksLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	testLinksAsUser("user2", t)
}

func TestRepoLinks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// repo1 has enabled almost features, so we can test most links
	repoLink := "/user2/repo1"
	links := []string{
		"/actions",
		"/packages",
		"/projects",
	}

	// anonymous user
	for _, link := range links {
		req := NewRequest(t, "GET", repoLink+link)
		MakeRequest(t, req, http.StatusOK)
	}

	// admin/owner user
	session := loginUser(t, "user1")
	for _, link := range links {
		req := NewRequest(t, "GET", repoLink+link)
		session.MakeRequest(t, req, http.StatusOK)
	}

	// non-admin non-owner user
	session = loginUser(t, "user2")
	for _, link := range links {
		req := NewRequest(t, "GET", repoLink+link)
		session.MakeRequest(t, req, http.StatusOK)
	}
}
