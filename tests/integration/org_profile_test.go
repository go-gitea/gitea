// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/shared/user"

	"github.com/stretchr/testify/assert"
)

func getCreateProfileReadmeFileOptions(content string) api.CreateFileOptions {
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))
	return api.CreateFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    "main",
			NewBranchName: "main",
			Message:       "create the profile README.md",
			Dates: api.CommitDateOptions{
				Author:    time.Unix(946684810, 0),
				Committer: time.Unix(978307190, 0),
			},
		},
		ContentBase64: contentEncoded,
	}
}

func createTestProfile(t *testing.T, orgName, profileRepoName, readmeContent string) {
	isPrivate := profileRepoName == user.RepoNameProfilePrivate

	ctx := NewAPITestContext(t, "user1", profileRepoName, auth_model.AccessTokenScopeAll)
	session := loginUser(t, "user1")
	tokenAdmin := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)

	// create repo
	doAPICreateOrganizationRepository(ctx, orgName, &api.CreateRepoOption{Name: profileRepoName, Private: isPrivate})(t)

	// create readme
	createFileOptions := getCreateProfileReadmeFileOptions(readmeContent)
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", orgName, profileRepoName, "README.md"), &createFileOptions).
		AddTokenAuth(tokenAdmin)
	MakeRequest(t, req, http.StatusCreated)
}

func TestOrgProfile(t *testing.T) {
	onGiteaRun(t, testOrgProfile)
}

func testOrgProfile(t *testing.T, u *url.URL) {
	const contentPublicReadme = "Public Readme Content"
	const contentPrivateReadme = "Private Readme Content"
	// HTML: "#org-home-view-as-dropdown" (indicate whether the view as dropdown menu is present)

	// PART 1: Test Both Private and Public
	createTestProfile(t, "org3", user.RepoNameProfile, contentPublicReadme)
	createTestProfile(t, "org3", user.RepoNameProfilePrivate, contentPrivateReadme)

	// Anonymous User
	req := NewRequest(t, "GET", "org3")
	resp := MakeRequest(t, req, http.StatusOK)
	bodyString := util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPublicReadme)
	assert.NotContains(t, bodyString, `id="org-home-view-as-dropdown"`)

	// Logged in but not member
	session := loginUser(t, "user24")
	req = NewRequest(t, "GET", "org3")
	resp = session.MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPublicReadme)
	assert.NotContains(t, bodyString, `id="org-home-view-as-dropdown"`)

	// Site Admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/org3")
	resp = session.MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPrivateReadme) // as an org member, default to show the private profile
	assert.Contains(t, bodyString, `id="org-home-view-as-dropdown"`)

	req = NewRequest(t, "GET", "/org3?view_as=member")
	resp = session.MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPrivateReadme)
	assert.Contains(t, bodyString, `id="org-home-view-as-dropdown"`)

	req = NewRequest(t, "GET", "/org3?view_as=public")
	resp = session.MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPublicReadme)
	assert.Contains(t, bodyString, `id="org-home-view-as-dropdown"`)

	// PART 2: Each org has either one of private pr public profile
	createTestProfile(t, "org41", user.RepoNameProfile, contentPublicReadme)
	createTestProfile(t, "org42", user.RepoNameProfilePrivate, contentPrivateReadme)

	// Anonymous User
	req = NewRequest(t, "GET", "/org41")
	resp = MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPublicReadme)
	assert.NotContains(t, bodyString, `id="org-home-view-as-dropdown"`)

	req = NewRequest(t, "GET", "/org42")
	resp = MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.NotContains(t, bodyString, contentPrivateReadme)
	assert.NotContains(t, bodyString, `id="org-home-view-as-dropdown"`)

	// Site Admin
	req = NewRequest(t, "GET", "/org41")
	resp = session.MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPublicReadme)
	assert.NotContains(t, bodyString, `id="org-home-view-as-dropdown"`)

	req = NewRequest(t, "GET", "/org42")
	resp = session.MakeRequest(t, req, http.StatusOK)
	bodyString = util.UnsafeBytesToString(resp.Body.Bytes())
	assert.Contains(t, bodyString, contentPrivateReadme)
	assert.NotContains(t, bodyString, `id="org-home-view-as-dropdown"`)
}
