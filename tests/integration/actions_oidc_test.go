// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/oauth2_provider"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oidcIntegrationClaims struct {
	jwt.RegisteredClaims
	Repository           string `json:"repository"`
	JobID                string `json:"job_id"`
	WorkflowRef          string `json:"workflow_ref"`
	WorkflowSHA          string `json:"workflow_sha"`
	JobWorkflowRef       string `json:"job_workflow_ref"`
	JobWorkflowSHA       string `json:"job_workflow_sha"`
	RunnerEnvironment    string `json:"runner_environment"`
	RepositoryVisibility string `json:"repository_visibility"`
	Environment          string `json:"environment"`
}

func TestActionsOIDCTokenIntegration(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repo := createActionsTestRepo(t, token, "actions-oidc", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		workflowContent := `name: OIDC
on:
  push:
    paths:
      - '.gitea/workflows/oidc.yml'
permissions:
  id-token: write

jobs:
  oidc-job:
    environment: production
    runs-on: ubuntu-latest
    steps:
      - run: echo oidc
`
		workflowPath := ".gitea/workflows/oidc.yml"
		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+workflowPath, workflowContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, workflowPath, opts)

		task := runner.fetchTask(t)
		contextMap := task.Context.AsMap()
		requestURL, ok := contextMap["actions_id_token_request_url"].(string)
		require.True(t, ok)
		requestToken, ok := contextMap["actions_id_token_request_token"].(string)
		require.True(t, ok)

		parsedURL, err := url.Parse(requestURL)
		require.NoError(t, err)
		query := parsedURL.Query()
		query.Set("audience", "integration-test")
		parsedURL.RawQuery = query.Encode()

		req := NewRequest(t, http.MethodGet, parsedURL.RequestURI())
		req.Header.Set("Authorization", "Bearer "+requestToken)
		resp := MakeRequest(t, req, http.StatusOK)
		var tokenResp struct {
			Value string `json:"value"`
		}
		DecodeJSON(t, resp, &tokenResp)
		require.NotEmpty(t, tokenResp.Value)

		var claims oidcIntegrationClaims
		signingKey := oauth2_provider.DefaultSigningKey
		parsed, err := jwt.ParseWithClaims(tokenResp.Value, &claims, func(t *jwt.Token) (any, error) {
			if t.Method == nil || t.Method.Alg() != signingKey.SigningMethod().Alg() {
				return nil, jwt.ErrSignatureInvalid
			}
			return signingKey.VerifyKey(), nil
		})
		require.NoError(t, err)
		require.True(t, parsed.Valid)

		assert.Equal(t, actions_service.OIDCIssuer(), claims.Issuer)
		assert.Contains(t, claims.Audience, "integration-test")
		assert.Equal(t, repo.FullName, claims.Repository)
		assert.Equal(t, "oidc-job", claims.JobID)
		assert.Equal(t, "self-hosted", claims.RunnerEnvironment)
		assert.Equal(t, "public", claims.RepositoryVisibility)
		assert.Equal(t, "production", claims.Environment)

		refValue, ok := contextMap["ref"].(string)
		require.True(t, ok)
		shaValue, ok := contextMap["sha"].(string)
		require.True(t, ok)
		workflowRef := repo.FullName + "/" + workflowPath + "@" + refValue
		assert.Equal(t, workflowRef, claims.WorkflowRef)
		assert.Equal(t, shaValue, claims.WorkflowSHA)
		assert.Equal(t, workflowRef, claims.JobWorkflowRef)
		assert.Equal(t, shaValue, claims.JobWorkflowSHA)

		runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
	})
}
