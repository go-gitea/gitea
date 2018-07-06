// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

// TestAPICreateAndDeleteToken tests that token that was just created can be deleted
func TestAPICreateAndDeleteToken(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")

	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"name": "test-key-1",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)

	// api.AccessToken
	var newAccessToken api.AccessToken
	DecodeJSON(t, resp, &newAccessToken)
	models.AssertExistsAndLoadBean(t, &models.AccessToken{
		ID:   newAccessToken.ID,
		Name: newAccessToken.Title,
		Sha1: newAccessToken.Sha1,
	})

	req = NewRequestf(t, "DELETE", "/api/v1/users/user1/tokens/%d", newAccessToken.ID)
	session.MakeRequest(t, req, http.StatusNoContent)
	models.AssertNotExistsBean(t, &models.AccessToken{ID: newAccessToken.ID})
}

// TestAPIDeleteMissingToken ensures that error is thrown when token not found
func TestAPIDeleteMissingToken(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")

	req := NewRequestf(t, "DELETE", "/api/v1/users/user1/tokens/%d", models.NonexistentID)
	session.MakeRequest(t, req, http.StatusNotFound)
}