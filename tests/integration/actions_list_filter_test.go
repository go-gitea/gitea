// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestActionsListFilters(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, user5.Name)
	locale := translation.NewLocale("en-US")

	t.Run("FilterDropdowns", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions", user5.Name, repo.Name))
		resp := session.MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()

		assert.Contains(t, body, locale.TrString("actions.runs.event"))
		assert.Contains(t, body, locale.TrString("actions.runs.branch"))
		assert.Contains(t, body, locale.TrString("actions.runs.events_no_select"))
		assert.Contains(t, body, locale.TrString("actions.runs.branches_no_select"))
		assert.Contains(t, body, "event=push")
		assert.Contains(t, body, "branch=master")
	})

	t.Run("FilterByEventAndBranch", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions?event=push&branch=master", user5.Name, repo.Name))
		resp := session.MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()

		assert.Contains(t, body, "#187")
		assert.Contains(t, body, "#189")
		assert.NotContains(t, body, "#190")
	})

	t.Run("PaginationPreservesFilters", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions?event=push&branch=master&limit=1", user5.Name, repo.Name))
		resp := session.MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()

		assert.Contains(t, body, "event=push")
		assert.Contains(t, body, "branch=master")
	})
}
