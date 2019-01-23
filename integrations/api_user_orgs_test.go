// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	api "code.gitea.io/sdk/gitea"
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

	assert.Equal(t, []*api.Organization{
		{
			ID:          3,
			UserName:    "user3",
			FullName:    "User Three",
			AvatarURL:   "https://secure.gravatar.com/avatar/97d6d9441ff85fdc730e02a6068d267b?d=identicon",
			Description: "",
			Website:     "",
			Location:    "",
		},
	}, orgs)
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

	assert.Equal(t, []*api.Organization{
		{
			ID:          3,
			UserName:    "user3",
			FullName:    "User Three",
			AvatarURL:   "https://secure.gravatar.com/avatar/97d6d9441ff85fdc730e02a6068d267b?d=identicon",
			Description: "",
			Website:     "",
			Location:    "",
		},
	}, orgs)
}
