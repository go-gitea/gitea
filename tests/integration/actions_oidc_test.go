// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	actions_service "gitea.dev/services/actions"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsOIDC(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getTokenForLoggedInUser(t, loginUser(t, user2.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		repo := createActionsTestRepo(t, token, "actions-oidc", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		workflowPath := ".gitea/workflows/oidc.yml"
		createWorkflowFile(t, token, user2.Name, repo.Name, workflowPath, getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+workflowPath, `name: deploy
on: push
jobs:
  plain:
    runs-on: ubuntu-latest
    steps:
      - run: echo
  oidc:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - run: echo
`))
		tasks := map[string]*runnerv1.Task{}
		for range 2 {
			task := runner.fetchTask(t)
			tasks[task.Context.GetFields()["job"].GetStringValue()] = task
		}
		assert.NotContains(t, tasks["plain"].Context.GetFields(), "actions_id_token_request_token")
		taskContext := tasks["oidc"].Context.GetFields()
		requestToken := taskContext["actions_id_token_request_token"].GetStringValue()
		tokenURL, err := url.Parse(taskContext["actions_id_token_request_url"].GetStringValue() + "&audience=sts.example.com")
		require.NoError(t, err)
		requestIDToken := func(bearer string, status int) *httptest.ResponseRecorder {
			return MakeRequest(t, NewRequest(t, "GET", tokenURL.RequestURI()).AddTokenAuth(bearer), status)
		}

		resp := MakeRequest(t, NewRequest(t, "GET", "/api/actions/oidc/.well-known/openid-configuration"), http.StatusOK)
		discovery := DecodeJSON(t, resp, &struct {
			JWKSURI         string   `json:"jwks_uri"`
			ClaimsSupported []string `json:"claims_supported"`
		}{})
		assert.Contains(t, discovery.ClaimsSupported, "job_workflow_ref")
		jwksURL, err := url.Parse(discovery.JWKSURI)
		require.NoError(t, err)
		resp = MakeRequest(t, NewRequest(t, "GET", jwksURL.RequestURI()), http.StatusOK)
		jwk := DecodeJSON(t, resp, &struct{ Keys []map[string]string }{}).Keys[0]
		require.Equal(t, "AQAB", jwk["e"])
		modulus, err := base64.RawURLEncoding.DecodeString(jwk["n"])
		require.NoError(t, err)

		claims := jwt.MapClaims{}
		idToken := DecodeJSON(t, requestIDToken(requestToken, http.StatusOK), &struct{ Value string }{}).Value
		_, err = jwt.ParseWithClaims(idToken, claims, func(*jwt.Token) (any, error) {
			return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: 65537}, nil
		}, jwt.WithIssuer(actions_service.OIDCIssuer()), jwt.WithAudience("sts.example.com"))
		require.NoError(t, err)
		workflowRef := repo.FullName + "/" + workflowPath + "@refs/heads/" + repo.DefaultBranch
		assert.Equal(t, fmt.Sprintf("repo:%s@%d/%s@%d:ref:refs/heads/%s", user2.Name, user2.ID, repo.Name, repo.ID, repo.DefaultBranch), claims["sub"])
		assert.Equal(t, "sts.example.com", claims["aud"])
		assert.Equal(t, "deploy", claims["workflow"])
		assert.Equal(t, workflowRef, claims["workflow_ref"])
		assert.Equal(t, workflowRef, claims["job_workflow_ref"])
		assert.Equal(t, "public", claims["repository_visibility"])

		requestIDToken(taskContext["gitea_runtime_token"].GetStringValue(), http.StatusUnauthorized)
		_, job, _ := getTaskAndJobAndRunByTaskID(t, tasks["oidc"].Id)
		job.TokenPermissions.IDToken = false
		_, err = actions_model.UpdateRunJob(t.Context(), job, nil, "token_permissions")
		require.NoError(t, err)
		requestIDToken(requestToken, http.StatusUnauthorized)
		job.TokenPermissions.IDToken = true
		_, err = actions_model.UpdateRunJob(t.Context(), job, nil, "token_permissions")
		require.NoError(t, err)
		runner.execTask(t, tasks["oidc"], &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		requestIDToken(requestToken, http.StatusUnauthorized)
	})
}
