// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestRevokeOAuth2GrantOfOtherUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// grant 1 belongs to user1, so user2 must not be able to act on it
	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user/settings/applications/oauth2/1/revoke/1", nil)
	session.MakeRequest(t, req, http.StatusNotFound)

	grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1})
	assert.EqualValues(t, 1, grant.UserID)
}
