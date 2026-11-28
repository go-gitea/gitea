// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestActionsEnvironment(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getUserToken(t, user2.Name, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-environment", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		defer doAPIDeleteRepository(NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository))(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		repoURL := fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo.Name)
		envURL := repoURL + "/environments/production"

		// Repository-scoped values under the same names, to prove the environment overrides them.
		req := NewRequestWithJSON(t, "PUT", repoURL+"/actions/secrets/DEPLOY_TOKEN",
			&api.CreateOrUpdateSecretOption{Data: "repo-token"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "POST", repoURL+"/actions/variables/APP_URL",
			&api.CreateVariableOption{Value: "https://repo.example.com"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PUT", envURL,
			&api.CreateOrUpdateEnvironmentOption{AllowedBranchPatterns: []string{"main"}}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "PUT", envURL+"/secrets/DEPLOY_TOKEN",
			&api.CreateOrUpdateSecretOption{Data: "env-token"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "POST", envURL+"/variables/APP_URL",
			&api.CreateVariableOption{Value: "https://prod.example.com"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		createRepoWorkflowFile(t, user2, token, repo, ".gitea/workflows/deploy.yml", `name: deploy
on: push
jobs:
  deploy:
    environment: production
    runs-on: ubuntu-latest
    steps:
      - run: echo deploy
`)

		task := runner.fetchTask(t)
		assert.Equal(t, "env-token", task.Secrets["DEPLOY_TOKEN"], "the environment secret must override the repository one")
		assert.Equal(t, "https://prod.example.com", task.Vars["APP_URL"], "the environment variable must override the repository one")
		runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
	})
}

func TestActionsEnvironmentSettings(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, "user2")
	collectionURL := "/user2/repo1/settings/actions/environments"
	envURL := collectionURL + "/staging"

	session.MakeRequest(t, NewRequestWithValues(t, "POST", collectionURL, map[string]string{"name": "staging"}), http.StatusOK)
	session.MakeRequest(t, NewRequestWithValues(t, "POST", envURL, map[string]string{"allowed_branch_patterns": "main"}), http.StatusOK)
	env := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionEnvironment{RepoID: repo.ID, LowerName: "staging"})
	assert.Equal(t, "main", env.AllowedBranchPatterns)

	body := session.MakeRequest(t, NewRequest(t, "GET", collectionURL), http.StatusOK).Body.String()
	assert.Contains(t, body, envURL)
	page := session.MakeRequest(t, NewRequest(t, "GET", envURL), http.StatusOK)
	assert.Equal(t, "main", NewHTMLParser(t, page.Body).Find(`textarea[name="allowed_branch_patterns"]`).Text())

	session.MakeRequest(t, NewRequestWithValues(t, "POST", envURL+"/variables/new", map[string]string{"name": "WEB_VAR", "data": "web-value"}), http.StatusOK)
	v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{RepoID: repo.ID, Name: "WEB_VAR"})
	assert.Equal(t, env.ID, v.EnvironmentID)
	assert.Contains(t, session.MakeRequest(t, NewRequest(t, "GET", envURL), http.StatusOK).Body.String(), "WEB_VAR")

	// the repository-level routes must not reach an environment variable
	session.MakeRequest(t, NewRequest(t, "POST", fmt.Sprintf("/user2/repo1/settings/actions/variables/%d/delete", v.ID)), http.StatusNotFound)
	session.MakeRequest(t, NewRequest(t, "POST", fmt.Sprintf("%s/variables/%d/delete", envURL, v.ID)), http.StatusOK)
	unittest.AssertNotExistsBean(t, &actions_model.ActionVariable{ID: v.ID})
}
