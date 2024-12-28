// Copyright 2019 The Gitea Authors. All rights reserved.
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

	"github.com/stretchr/testify/assert"
)

func getCreateProfileReadmeFileOptions(profileType string) api.CreateFileOptions {
	content := fmt.Sprintf("# %s", profileType)
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

func createTestProfile(t *testing.T, orgName, profileType string) {
	repoName := ".profile"
	isPrivate := false
	if profileType == "Private" {
		repoName = ".profile-private"
		isPrivate = true
	}

	ctx := NewAPITestContext(t, "user1", repoName, auth_model.AccessTokenScopeAll)
	session := loginUser(t, "user1")
	tokenAdmin := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)

	// create repo
	t.Run("CreateOrganization"+profileType+"ProfileRepo", doAPICreateOrganizationRepository(ctx, orgName, &api.CreateRepoOption{
		Name:    repoName,
		Private: isPrivate,
	}))

	// create readme
	createFileOptions := getCreateProfileReadmeFileOptions(profileType)
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", orgName, repoName, "README.md"), &createFileOptions).
		AddTokenAuth(tokenAdmin)
	MakeRequest(t, req, http.StatusCreated)
}

func TestOrgProfile(t *testing.T) {
	onGiteaRun(t, testOrgProfile)
}

func testOrgProfile(t *testing.T, u *url.URL) {
	// html #user-content-public (markdown title of public profile)
	// html #user-content-private (markdown title of private profile)
	// html #profile_view_as_dropdown (indicate whether the view as dropdown menu is present)

	// PART 1: Test Both Private and Public
	createTestProfile(t, "org3", "Public")
	createTestProfile(t, "org3", "Private")

	// Anonymous User
	req := NewRequest(t, "GET", "org3")
	resp := MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	profileDivs := htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 1, profileDivs.Length())

	dropDownDiv := htmlDoc.doc.Find("#profile_view_as_dropdown")
	assert.EqualValues(t, 0, dropDownDiv.Length())

	// Logged in but not member
	session := loginUser(t, "user24")
	req = NewRequest(t, "GET", "org3")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	profileDivs = htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 1, profileDivs.Length())

	dropDownDiv = htmlDoc.doc.Find("#profile_view_as_dropdown")
	assert.EqualValues(t, 0, dropDownDiv.Length())

	// Site Admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "org3")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	profileDivs = htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 1, profileDivs.Length())

	dropDownDiv = htmlDoc.doc.Find("#profile_view_as_dropdown")
	assert.EqualValues(t, 1, dropDownDiv.Length())

	req = NewRequest(t, "GET", "/org3?view_as=member")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	profileDivs = htmlDoc.doc.Find("#user-content-private")
	assert.EqualValues(t, 1, profileDivs.Length())

	req = NewRequest(t, "GET", "/org3?view_as=public")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	profileDivs = htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 1, profileDivs.Length())

	// PART 2: Each org has either one of private pr public profile
	createTestProfile(t, "org41", "Public")
	createTestProfile(t, "org42", "Private")

	// Anonymous User
	req = NewRequest(t, "GET", "/org41")
	resp = MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	profileDivs = htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 1, profileDivs.Length())
	dropDownDiv = htmlDoc.doc.Find("#profile_view_as_dropdown")
	assert.EqualValues(t, 0, dropDownDiv.Length())

	req = NewRequest(t, "GET", "/org42")
	resp = MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	profileDivs = htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 0, profileDivs.Length())
	profileDivs = htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 0, profileDivs.Length())
	dropDownDiv = htmlDoc.doc.Find("#profile_view_as_dropdown")
	assert.EqualValues(t, 0, dropDownDiv.Length())

	// Site Admin
	req = NewRequest(t, "GET", "/org41")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	profileDivs = htmlDoc.doc.Find("#user-content-public")
	assert.EqualValues(t, 1, profileDivs.Length())
	dropDownDiv = htmlDoc.doc.Find("#profile_view_as_dropdown")
	assert.EqualValues(t, 0, dropDownDiv.Length())

	req = NewRequest(t, "GET", "/org42")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	profileDivs = htmlDoc.doc.Find("#user-content-private")
	assert.EqualValues(t, 1, profileDivs.Length())
	dropDownDiv = htmlDoc.doc.Find("#profile_view_as_dropdown")
	assert.EqualValues(t, 0, dropDownDiv.Length())
}
