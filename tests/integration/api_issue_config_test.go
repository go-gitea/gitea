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
	"gopkg.in/yaml.v3"
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

	var issueConfig api.IssueConfig
	DecodeJSON(t, resp, &issueConfig)

	return issueConfig
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

	t.Run("DisableBlankIssues", func(t *testing.T) {
		config := make(map[string]any)
		config["blank_issues_enabled"] = false

		createIssueConfig(t, owner, repo, config)

		issueConfig := getIssueConfig(t, owner.Name, repo.Name)

		assert.False(t, issueConfig.BlankIssuesEnabled)
		assert.Empty(t, issueConfig.ContactLinks)
	})

	t.Run("ContactLinks", func(t *testing.T) {
		contactLink := make(map[string]string)
		contactLink["name"] = "TestName"
		contactLink["url"] = "https://example.com"
		contactLink["about"] = "TestAbout"

		config := make(map[string]any)
		config["contact_links"] = []map[string]string{contactLink}

		createIssueConfig(t, owner, repo, config)

		issueConfig := getIssueConfig(t, owner.Name, repo.Name)

		assert.True(t, issueConfig.BlankIssuesEnabled)
		assert.Len(t, issueConfig.ContactLinks, 1)

		assert.Equal(t, "TestName", issueConfig.ContactLinks[0].Name)
		assert.Equal(t, "https://example.com", issueConfig.ContactLinks[0].URL)
		assert.Equal(t, "TestAbout", issueConfig.ContactLinks[0].About)
	})

	t.Run("Full", func(t *testing.T) {
		contactLink := make(map[string]string)
		contactLink["name"] = "TestName"
		contactLink["url"] = "https://example.com"
		contactLink["about"] = "TestAbout"

		config := make(map[string]any)
		config["blank_issues_enabled"] = false
		config["contact_links"] = []map[string]string{contactLink}

		createIssueConfig(t, owner, repo, config)

		issueConfig := getIssueConfig(t, owner.Name, repo.Name)

		assert.False(t, issueConfig.BlankIssuesEnabled)
		assert.Len(t, issueConfig.ContactLinks, 1)

		assert.Equal(t, "TestName", issueConfig.ContactLinks[0].Name)
		assert.Equal(t, "https://example.com", issueConfig.ContactLinks[0].URL)
		assert.Equal(t, "TestAbout", issueConfig.ContactLinks[0].About)
	})
}

func TestAPIRepoIssueConfigPaths(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 49})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

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

				_, err = createFileInBranch(owner, repo, fullPath, repo.DefaultBranch, string(configData))
				assert.NoError(t, err)

				issueConfig := getIssueConfig(t, owner.Name, repo.Name)

				assert.False(t, issueConfig.BlankIssuesEnabled)
				assert.Empty(t, issueConfig.ContactLinks)

				_, err = deleteFileInBranch(owner, repo, fullPath, repo.DefaultBranch)
				assert.NoError(t, err)
			})
		}
	}
}

func TestAPIRepoValidateIssueConfig(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 49})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issue_config/validate", owner.Name, repo.Name)

	t.Run("Valid", func(t *testing.T) {
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		var issueConfigValidation api.IssueConfigValidation
		DecodeJSON(t, resp, &issueConfigValidation)

		assert.True(t, issueConfigValidation.Valid)
		assert.Empty(t, issueConfigValidation.Message)
	})

	t.Run("Invalid", func(t *testing.T) {
		config := make(map[string]any)
		config["blank_issues_enabled"] = "Test"

		createIssueConfig(t, owner, repo, config)

		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		var issueConfigValidation api.IssueConfigValidation
		DecodeJSON(t, resp, &issueConfigValidation)

		assert.False(t, issueConfigValidation.Valid)
		assert.NotEmpty(t, issueConfigValidation.Message)
	})
}
