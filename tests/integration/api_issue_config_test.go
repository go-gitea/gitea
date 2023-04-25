// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIReposGetDefaultIssueConfig(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issue_config", owner.Name, repo.Name)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var issueConfig api.IssueConfig
	DecodeJSON(t, resp, &issueConfig)

	assert.True(t, issueConfig.BlankIssuesEnabled)
	assert.Equal(t, issueConfig.ContactLinks, make([]api.IssueConfigContactLink, 0))
}

func TestAPIReposValidateDefaultIssueConfig(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issue_config/validate", owner.Name, repo.Name)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var issueConfigValidation api.IssueConfigValidation
	DecodeJSON(t, resp, &issueConfigValidation)

	assert.True(t, issueConfigValidation.Valid)
	assert.Empty(t, issueConfigValidation.Message)
}
