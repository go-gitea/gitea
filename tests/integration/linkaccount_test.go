// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/tests"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
)

func TestLinkAccountChoose(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	username := "linkaccountuser"
	email := "linkaccountuser@example.com"
	password := "linkaccountuser"
	defer createUser(t, username, email, password)()

	defer func() {
		testMiddlewareHook = nil
	}()

	for _, testCase := range []struct {
		gothUser  goth.User
		signupTab string
		signinTab string
	}{
		{
			gothUser:  goth.User{},
			signupTab: "item active",
			signinTab: "item ",
		},
		{
			gothUser: goth.User{
				Email: email,
			},
			signupTab: "item ",
			signinTab: "item active",
		},
	} {
		testMiddlewareHook = func(ctx *gitea_context.Context) {
			ctx.Session.Set("linkAccountGothUser", testCase.gothUser)
		}

		req := NewRequest(t, "GET", "/user/link_account")
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, resp.Code, http.StatusOK, resp.Body)
		doc := NewHTMLParser(t, resp.Body)

		class, exists := doc.Find(`.new-menu-inner .item[data-tab="auth-link-signup-tab"]`).Attr("class")
		assert.True(t, exists, resp.Body)
		assert.Equal(t, testCase.signupTab, class)

		class, exists = doc.Find(`.new-menu-inner .item[data-tab="auth-link-signin-tab"]`).Attr("class")
		assert.True(t, exists)
		assert.Equal(t, testCase.signinTab, class)
	}
}
