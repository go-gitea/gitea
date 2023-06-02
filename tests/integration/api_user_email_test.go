// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListEmails(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	req := NewRequest(t, "GET", "/api/v1/user/emails?token="+token)
	resp := MakeRequest(t, req, http.StatusOK)

	var emails []*api.Email
	DecodeJSON(t, resp, &emails)

	assert.EqualValues(t, []*api.Email{
		{
			Email:    "user2@example.com",
			Verified: true,
			Primary:  true,
		},
		{
			Email:    "user2-2@example.com",
			Verified: false,
			Primary:  false,
		},
	}, emails)
}

func TestAPIAddEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	opts := api.CreateEmailOption{
		Emails: []string{"user101@example.com"},
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/emails?token="+token, &opts)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	opts = api.CreateEmailOption{
		Emails: []string{"user2-3@example.com"},
	}
	req = NewRequestWithJSON(t, "POST", "/api/v1/user/emails?token="+token, &opts)
	resp := MakeRequest(t, req, http.StatusCreated)

	var emails []*api.Email
	DecodeJSON(t, resp, &emails)
	assert.EqualValues(t, []*api.Email{
		{
			Email:    "user2-3@example.com",
			Verified: true,
			Primary:  false,
		},
	}, emails)

	opts = api.CreateEmailOption{
		Emails: []string{"notAEmail"},
	}
	req = NewRequestWithJSON(t, "POST", "/api/v1/user/emails?token="+token, &opts)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPIDeleteEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	opts := api.DeleteEmailOption{
		Emails: []string{"user2-3@example.com"},
	}
	req := NewRequestWithJSON(t, "DELETE", "/api/v1/user/emails?token="+token, &opts)
	MakeRequest(t, req, http.StatusNotFound)

	opts = api.DeleteEmailOption{
		Emails: []string{"user2-2@example.com"},
	}
	req = NewRequestWithJSON(t, "DELETE", "/api/v1/user/emails?token="+token, &opts)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "GET", "/api/v1/user/emails?token="+token)
	resp := MakeRequest(t, req, http.StatusOK)

	var emails []*api.Email
	DecodeJSON(t, resp, &emails)
	assert.EqualValues(t, []*api.Email{
		{
			Email:    "user2@example.com",
			Verified: true,
			Primary:  true,
		},
	}, emails)
}
