// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryVisibilityChange(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	t.Run("MakePrivateRequiresCorrectName", func(t *testing.T) {
		// Wrong name should be rejected with a JSON error
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings", map[string]string{
			"action":            "visibility",
			"private":           "true",
			"confirm_repo_name": "wrong-name",
		})
		resp := session.MakeRequest(t, req, http.StatusBadRequest)
		assert.NotEmpty(t, test.ParseJSONError(resp.Body.Bytes()).ErrorMessage)

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		assert.False(t, repo1.IsPrivate)

		// Correct full name (owner/repo) should succeed with a JSON redirect
		req = NewRequestWithValues(t, "POST", "/user2/repo1/settings", map[string]string{
			"action":            "visibility",
			"private":           "true",
			"confirm_repo_name": "user2/repo1",
		})
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.NotEmpty(t, test.ParseJSONRedirect(resp.Body.Bytes()).Redirect)

		repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		assert.True(t, repo1.IsPrivate)
	})

	t.Run("MakePublicDoesNotRequireName", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST", "/user2/repo2/settings", map[string]string{
			"action":  "visibility",
			"private": "false",
		})
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.NotEmpty(t, test.ParseJSONRedirect(resp.Body.Bytes()).Redirect)

		repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		assert.False(t, repo2.IsPrivate)
	})
}
