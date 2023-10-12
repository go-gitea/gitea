// Copyright 2023 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func createFundingConfig(t *testing.T, user *user_model.User, repo *repo_model.Repository, fundingConfig map[string]any) {
	config, err := yaml.Marshal(fundingConfig)
	assert.NoError(t, err)
	assert.NoError(t, createOrReplaceFileInBranch(user, repo, ".gitea/FUNDING.yaml", repo.DefaultBranch, string(config)))
}

func getRepoFundingConfig(t *testing.T, repo *repo_model.Repository, token string) []*api.RepoFundingEntry {
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/funding?token=%s", repo.OwnerName, repo.Name, token)

	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var funding []*api.RepoFundingEntry

	DecodeJSON(t, resp, &funding)

	return funding
}

func TestAPIRepoFunding(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	assert.Len(t, getRepoFundingConfig(t, repo, token), 0)

	t.Run("SimpleConfig", func(t *testing.T) {
		config := make(map[string]any)
		config["custom"] = "https://example.com"
		// config["ko_fi"] = "test"

		createFundingConfig(t, owner, repo, config)

		funding := getRepoFundingConfig(t, repo, token)

		assert.Equal(t, "https://example.com", funding[0].Text)
		assert.Equal(t, "https://example.com", funding[0].URL)
		assert.Equal(t, setting.AppSubURL+"/assets/img/svg/octicon-link.svg", funding[0].Icon)

		// assert.Equal(t, "Ko-Fi/test", funding[1].Text)
		// assert.Equal(t, "https://ko-fi.com/test", funding[1].URL)
		// assert.Equal(t, setting.AppSubURL+"/assets/img/funding/ko_fi.svg", funding[1].Icon)
	})

	t.Run("StringArray", func(t *testing.T) {
		testSlice := make([]string, 2)
		testSlice[0] = "https://a.com"
		testSlice[1] = "https://b.com"

		config := make(map[string]any)
		config["custom"] = testSlice

		createFundingConfig(t, owner, repo, config)

		funding := getRepoFundingConfig(t, repo, token)

		assert.Equal(t, "https://a.com", funding[0].Text)
		assert.Equal(t, "https://a.com", funding[0].URL)
		assert.Equal(t, setting.AppSubURL+"/assets/img/svg/octicon-link.svg", funding[0].Icon)

		assert.Equal(t, "https://b.com", funding[1].Text)
		assert.Equal(t, "https://b.com", funding[1].URL)
		assert.Equal(t, setting.AppSubURL+"/assets/img/svg/octicon-link.svg", funding[1].Icon)
	})
}
