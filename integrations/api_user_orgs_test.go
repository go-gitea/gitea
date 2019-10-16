// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestUserOrgs(t *testing.T) {
	prepareTestEnv(t)
	adminUsername := "user1"
	normalUsername := "user2"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/users/%s/orgs?token=%s", normalUsername, token)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var orgs []*api.Organization
	DecodeJSON(t, resp, &orgs)
	assert.Equal(t, 1, len(orgs))

	user3 := models.AssertExistsAndLoadBean(t, &models.User{Name: "user3"}).(*models.User)
	apiURL := fmt.Sprintf("%sapi/v1/orgs/%s", setting.AppURL, user3.LowerName)
	expectedOrg := &api.Organization{
		ID:               3,
		UserName:         user3.Name,
		FullName:         user3.FullName,
		AvatarURL:        user3.AvatarLink(),
		URL:              apiURL,
		ReposURL:         apiURL + "/repos",
		MembersURL:       apiURL + "/members{/member}",
		TeamsURL:         apiURL + "/teams",
		PublicMembersURL: apiURL + "/public_members{/member}",
		Description:      "",
		Website:          "",
		Location:         "",
		Visibility:       "public",
		PublicRepoCount:  1,
		Created:          orgs[0].Created,
		Updated:          orgs[0].Updated,
	}

	assert.Equal(t, expectedOrg, orgs[0])
}

func TestMyOrgs(t *testing.T) {
	prepareTestEnv(t)

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequest(t, "GET", "/api/v1/user/orgs?token="+token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var orgs []*api.Organization
	DecodeJSON(t, resp, &orgs)
	assert.Equal(t, 1, len(orgs))

	user3 := models.AssertExistsAndLoadBean(t, &models.User{Name: "user3"}).(*models.User)
	apiURL := setting.AppURL + "api/v1/orgs/" + user3.LowerName
	apiURL := fmt.Sprintf("%sapi/v1/orgs/%s", setting.AppURL, user3.LowerName)
	expectedOrg := &api.Organization{
		ID:               3,
		UserName:         user3.Name,
		FullName:         user3.FullName,
		AvatarURL:        user3.AvatarLink(),
		URL:              apiURL,
		ReposURL:         apiURL + "/repos",
		HooksURL:         apiURL + "/hooks",
		MembersURL:       apiURL + "/members{/member}",
		TeamsURL:         apiURL + "/teams",
		PublicMembersURL: apiURL + "/public_members{/member}",
		Description:      "",
		Website:          "",
		Location:         "",
		Visibility:       "public",
		PublicRepoCount:  1,
		Created:          orgs[0].Created,
		Updated:          orgs[0].Updated,
	}

	assert.Equal(t, expectedOrg, orgs[0])
}
