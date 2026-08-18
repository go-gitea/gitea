// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func environmentWorkflow(branch, environment string) string {
	return fmt.Sprintf(`name: deploy
on:
  push:
    branches: [%s]
jobs:
  deploy:
    environment: %s
    runs-on: ubuntu-latest
    steps:
      - run: echo deploy
`, branch, environment)
}

func TestActionsEnvironment(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getTokenForLoggedInUser(t, loginUser(t, user2.Name),
			auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

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

		t.Run("AnAllowedBranchReceivesTheEnvironmentValues", func(t *testing.T) {
			opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "add allowed workflow", environmentWorkflow("main", "production"))
			createWorkflowFile(t, token, user2.Name, repo.Name, ".gitea/workflows/deploy.yml", opts)

			task := runner.fetchTask(t)
			assert.Equal(t, "env-token", task.Secrets["DEPLOY_TOKEN"], "the environment secret must override the repository one")
			assert.Equal(t, "https://prod.example.com", task.Vars["APP_URL"], "the environment variable must override the repository one")
			runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		})

		t.Run("SettingsPagesRender", func(t *testing.T) {
			session := loginUser(t, user2.Name)
			settingsURL := fmt.Sprintf("/%s/%s/settings/actions/environments", user2.Name, repo.Name)

			body := session.MakeRequest(t, NewRequest(t, "GET", settingsURL), http.StatusOK).Body.String()
			assert.Contains(t, body, "production")

			body = session.MakeRequest(t, NewRequest(t, "GET", settingsURL+"/production"), http.StatusOK).Body.String()
			assert.Contains(t, body, "DEPLOY_TOKEN", "the shared secrets partial must render")
			assert.Contains(t, body, "APP_URL", "the shared variables partial must render")

			policy := NewHTMLParser(t, strings.NewReader(body)).Find(`textarea[name="allowed_branch_patterns"]`)
			assert.Equal(t, "main", policy.Text(), "the branch policy must render into its textarea")
		})

		t.Run("TheSettingsPageWritesVariablesInTheEnvironmentScope", func(t *testing.T) {
			session := loginUser(t, user2.Name)
			envURL := fmt.Sprintf("/%s/%s/settings/actions/environments/production", user2.Name, repo.Name)

			req := NewRequestWithValues(t, "POST", envURL+"/variables/new", map[string]string{"name": "WEB_VAR", "data": "web-value"})
			session.MakeRequest(t, req, http.StatusOK)

			env := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionEnvironment{RepoID: repo.ID, LowerName: "production"})
			v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{RepoID: repo.ID, Name: "WEB_VAR"})
			assert.Equal(t, env.ID, v.EnvironmentID)

			req = NewRequest(t, "POST", fmt.Sprintf("%s/variables/%d/delete", envURL, v.ID))
			session.MakeRequest(t, req, http.StatusOK)
			unittest.AssertNotExistsBean(t, &actions_model.ActionVariable{ID: v.ID})
		})

		t.Run("AnEnvironmentNamedNewIsStillEditable", func(t *testing.T) {
			session := loginUser(t, user2.Name)
			collectionURL := fmt.Sprintf("/%s/%s/settings/actions/environments", user2.Name, repo.Name)

			req := NewRequestWithValues(t, "POST", collectionURL, map[string]string{"name": "new"})
			session.MakeRequest(t, req, http.StatusSeeOther)
			req = NewRequestWithValues(t, "POST", collectionURL+"/new", map[string]string{"allowed_branch_patterns": "main"})
			session.MakeRequest(t, req, http.StatusSeeOther)

			env := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionEnvironment{RepoID: repo.ID, LowerName: "new"})
			assert.Equal(t, "main", env.AllowedBranchPatterns, "the edit form must not be routed to creation")
		})

		// A job that may not deploy must fail rather than run with the environment's credentials withheld.
		failsToDeploy := func(t *testing.T, branch, environment string) {
			opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "add "+branch+" workflow", environmentWorkflow(branch, environment))
			opts.NewBranchName = branch
			createWorkflowFile(t, token, user2.Name, repo.Name, ".gitea/workflows/deploy-"+branch+".yml", opts)

			// the denial happens on the pick, so the runner has to ask before the job can fail
			require.Eventually(t, func() bool {
				task, _ := runner.fetchTaskOnce(t, 0)
				assert.Nil(t, task, "the job must not reach a runner")
				return unittest.GetCount(t, &actions_model.ActionRunJob{
					RepoID:          repo.ID,
					EnvironmentName: environment,
					Status:          actions_model.StatusFailure,
				}) == 1
			}, 5*time.Second, 100*time.Millisecond)
		}

		t.Run("ADisallowedBranchFailsTheJob", func(t *testing.T) { failsToDeploy(t, "feature", "production") })
		t.Run("AnUnusableEnvironmentNameFailsTheJob", func(t *testing.T) { failsToDeploy(t, "unusable", "deploy/prod") })
	})
}
