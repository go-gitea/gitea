// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func createIssueConfig(t *testing.T, user *user_model.User, repo *repo_model.Repository, issueConfig map[string]any) {
	config, err := yaml.Marshal(issueConfig)
	assert.NoError(t, err)

	err = createOrReplaceFileInBranch(user, repo, ".gitea/ISSUE_TEMPLATE/config.yaml", repo.DefaultBranch, string(config))
	assert.NoError(t, err)
}

func getIssueConfig(t *testing.T, owner, repo string) api.IssueConfig {
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issue_config", owner, repo)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	issueConfig := DecodeJSON(t, resp, &api.IssueConfig{})

	return *issueConfig
}

func TestAPIRepoGetIssueConfig(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 49})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	t.Run("Default", func(t *testing.T) {
		issueConfig := getIssueConfig(t, owner.Name, repo.Name)

		assert.True(t, issueConfig.BlankIssuesEnabled)
		assert.Empty(t, issueConfig.ContactLinks)
	})

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issue_config/validate", owner.Name, repo.Name)

	t.Run("Validate/Valid", func(t *testing.T) {
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		issueConfigValidation := DecodeJSON(t, resp, &api.IssueConfigValidation{})

		assert.True(t, issueConfigValidation.Valid)
		assert.Empty(t, issueConfigValidation.Message)
	})

	t.Run("Paths", func(t *testing.T) {
		templateConfigCandidates := []string{
			".gitea/ISSUE_TEMPLATE/config",
			".gitea/issue_template/config",
			".github/ISSUE_TEMPLATE/config",
			".github/issue_template/config",
		}

		for _, candidate := range templateConfigCandidates {
			for _, extension := range []string{".yaml", ".yml"} {
				fullPath := candidate + extension
				t.Run(fullPath, func(t *testing.T) {
					configMap := make(map[string]any)
					configMap["blank_issues_enabled"] = false

					configData, err := yaml.Marshal(configMap)
					assert.NoError(t, err)

					_, err = createFile(owner, repo, fullPath, string(configData))
					assert.NoError(t, err)

					issueConfig := getIssueConfig(t, owner.Name, repo.Name)

					assert.False(t, issueConfig.BlankIssuesEnabled)
					assert.Empty(t, issueConfig.ContactLinks)

					_, err = deleteFileInBranch(owner, repo, fullPath, repo.DefaultBranch)
					assert.NoError(t, err)
				})
			}
		}
	})

	t.Run("DisableBlankIssues", func(t *testing.T) {
		config := make(map[string]any)
		config["blank_issues_enabled"] = false

		createIssueConfig(t, owner, repo, config)

		issueConfig := getIssueConfig(t, owner.Name, repo.Name)

		assert.False(t, issueConfig.BlankIssuesEnabled)
		assert.Empty(t, issueConfig.ContactLinks)
	})

	for _, name := range []string{"ContactLinks", "Full"} {
		t.Run(name, func(t *testing.T) {
			contactLink := make(map[string]string)
			contactLink["name"] = "TestName"
			contactLink["url"] = "https://example.com"
			contactLink["about"] = "TestAbout"

			config := make(map[string]any)
			if name == "Full" {
				config["blank_issues_enabled"] = false
			}
			config["contact_links"] = []map[string]string{contactLink}

			createIssueConfig(t, owner, repo, config)

			issueConfig := getIssueConfig(t, owner.Name, repo.Name)

			if name == "Full" {
				assert.False(t, issueConfig.BlankIssuesEnabled)
			} else {
				assert.True(t, issueConfig.BlankIssuesEnabled)
			}
			assert.Len(t, issueConfig.ContactLinks, 1)

			assert.Equal(t, "TestName", issueConfig.ContactLinks[0].Name)
			assert.Equal(t, "https://example.com", issueConfig.ContactLinks[0].URL)
			assert.Equal(t, "TestAbout", issueConfig.ContactLinks[0].About)
		})
	}

	t.Run("Validate/Invalid", func(t *testing.T) {
		config := make(map[string]any)
		config["blank_issues_enabled"] = "Test"

		createIssueConfig(t, owner, repo, config)

		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		issueConfigValidation := DecodeJSON(t, resp, &api.IssueConfigValidation{})

		assert.False(t, issueConfigValidation.Valid)
		assert.NotEmpty(t, issueConfigValidation.Message)
	})

	t.Run("RequiresCodeUnit", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 24})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadUser)

		for _, path := range []string{
			fmt.Sprintf("/api/v1/repos/%s/issue_config", repo.FullName()),
			fmt.Sprintf("/api/v1/repos/%s/issue_config/validate", repo.FullName()),
		} {
			req := NewRequest(t, "GET", path).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusForbidden)
		}
	})
}
