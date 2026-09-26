// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"path"
	"testing"

	"gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertLinkPageComplete(t *testing.T, session *TestSession, link string, containStrings ...string) {
	req := NewRequest(t, "GET", link)
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()), "Page did not complete: "+link)
	for _, s := range containStrings {
		assert.Contains(t, resp.Body.String(), s, "Page does not contain expected string: "+s)
	}
}

func TestLinks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("NoLogin", testLinksNoLogin)
	t.Run("RedirectsNoLogin", testLinksRedirectsNoLogin)
	t.Run("NoLoginNotExist", testLinksNoLoginNotExist)
	t.Run("AsUser", testLinksAsUser)
	t.Run("RepoCommon", testLinksRepoCommon)
	t.Run("ApiJson", testLinksApiJson)
}

func testLinksApiJson(t *testing.T) {
	defer test.MockVariableValue(&setting.AppVer, "1.2.3")()
	defer test.MockVariableValue(&setting.AppSubURL)()
	t.Run("Swagger", func(t *testing.T) {
		for _, subURL := range []string{"", "/sub"} {
			setting.AppSubURL = subURL
			resp := MakeRequest(t, NewRequest(t, "GET", "/swagger.v1.json"), http.StatusOK)
			decoded := DecodeJSON(t, resp, &struct {
				BasePath string `json:"basePath"`
				Info     struct {
					Version string `json:"version"`
				}
			}{})
			assert.Equal(t, subURL+"/api/v1", decoded.BasePath)
			assert.Equal(t, "1.2.3", decoded.Info.Version)
		}
	})
	t.Run("OpenAPI3", func(t *testing.T) {
		for _, subURL := range []string{"", "/sub"} {
			setting.AppSubURL = subURL
			resp := MakeRequest(t, NewRequest(t, "GET", "/openapi3.v1.json"), http.StatusOK)
			decoded := DecodeJSON(t, resp, &struct {
				Servers []struct {
					URL string `json:"url"`
				} `json:"servers"`
				Info struct {
					Version string `json:"version"`
				}
			}{})
			assert.Equal(t, subURL+"/api/v1", decoded.Servers[0].URL)
			assert.Equal(t, "1.2.3", decoded.Info.Version)
		}
	})
}

func testLinksNoLogin(t *testing.T) {
	links := []string{
		"/",
		"/explore/repos",
		"/explore/repos?q=test",
		"/explore/users",
		"/explore/users?q=test",
		"/explore/organizations",
		"/explore/organizations?q=test",
		"/user/sign_up",
		"/user/login",
		"/user/forgot_password",
		"/user2/repo1",
		"/user2/repo1/",
		"/user2/repo1/projects",
		"/user2/repo1/projects/1",
		"/user2/repo1/releases/tag/delete-tag", // It's the only one existing record on release.yml which has is_tag: true
		"/api/swagger",
	}
	for _, link := range links {
		assertLinkPageComplete(t, nil, link)
	}
	MakeRequest(t, NewRequest(t, "GET", "/.well-known/security.txt"), http.StatusOK)
}

func testLinksRedirectsNoLogin(t *testing.T) {
	redirects := []struct{ from, to string }{
		{"/user2/repo1/commits/master", "/user2/repo1/commits/branch/master"},
		{"/user2/repo1/src/master", "/user2/repo1/src/branch/master"},
		{"/user2/repo1/src/master/a%2fb.txt", "/user2/repo1/src/branch/master/a%2fb.txt"},
		{"/user2/repo1/src/master/directory/file.txt?a=1", "/user2/repo1/src/branch/master/directory/file.txt?a=1"},
		{"/user2/repo1/src/branch/master/directory/file.txt?raw=1&other=2", "/user2/repo1/raw/branch/master/directory/file.txt"},
		{"/user2/repo1/tree/a%2fb?a=1", "/user2/repo1/src/a%2fb?a=1"},
		{"/user2/repo1/blob/123456/%20?a=1", "/user2/repo1/src/commit/123456/%20?a=1"},
		{"/user/avatar/GhosT/-1", "/assets/img/avatar_default.png"},
		{"/user/avatar/Gitea-ActionS/0", "/assets/img/avatar_default.png"},
		{"/api/v1/swagger", "/api/swagger"},
	}
	for _, c := range redirects {
		req := NewRequest(t, "GET", c.from)
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, path.Join(setting.AppSubURL, c.to), test.RedirectURL(resp))
	}
}

func testLinksNoLoginNotExist(t *testing.T) {
	links := []string{
		"/user5/repo4/projects",
		"/user5/repo4/projects/3",
	}

	for _, link := range links {
		req := NewRequest(t, "GET", link)
		MakeRequest(t, req, http.StatusNotFound)
	}
}

func testLinksAsUser(t *testing.T) {
	session := loginUser(t, "user2")
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

	for _, link := range links {
		assertLinkPageComplete(t, session, link)
	}

	reqAPI := NewRequestf(t, "GET", "/api/v1/users/user2/repos")
	respAPI := MakeRequest(t, reqAPI, http.StatusOK)
	apiRepos := DecodeJSON(t, respAPI, []*api.Repository{})
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
			link = fmt.Sprintf("/user2/%s%s", repo.Name, link)
			assertLinkPageComplete(t, session, link)
		}
	}
}

func testLinksRepoCommon(t *testing.T) {
	// repo1 has enabled almost features, so we can test most links
	repoLink := "/user2/repo1"

	err := db.Insert(t.Context(), &actions.ActionRun{Title: "", RepoID: 1})
	require.NoError(t, err)

	links := map[string][]string{
		"/actions":  {"(empty commit message)"},
		"/packages": {},
		"/projects": {},
	}

	// anonymous user
	for link, strs := range links {
		assertLinkPageComplete(t, nil, repoLink+link, strs...)
	}

	// admin/owner user
	session := loginUser(t, "user1")
	for link, strs := range links {
		assertLinkPageComplete(t, session, repoLink+link, strs...)
	}

	// non-admin non-owner user
	session = loginUser(t, "user2")
	for link, strs := range links {
		assertLinkPageComplete(t, session, repoLink+link, strs...)
	}
}
