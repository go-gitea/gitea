// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	actions_service "gitea.dev/services/actions"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oidcIntegrationClaims struct {
	jwt.RegisteredClaims
	Repository           string `json:"repository"`
	RepositoryID         string `json:"repository_id"`
	RepositoryOwnerID    string `json:"repository_owner_id"`
	JobID                string `json:"job_id"`
	Workflow             string `json:"workflow"`
	WorkflowRepository   string `json:"workflow_repository"`
	WorkflowRepositoryID string `json:"workflow_repository_id"`
	WorkflowRef          string `json:"workflow_ref"`
	WorkflowSHA          string `json:"workflow_sha"`
	JobWorkflowRef       string `json:"job_workflow_ref"`
	JobWorkflowSHA       string `json:"job_workflow_sha"`
	RunnerEnvironment    string `json:"runner_environment"`
	Ref                  string `json:"ref"`
	SHA                  string `json:"sha"`
}

func oidcJWKPublicKey(t *testing.T, jwk map[string]string) *rsa.PublicKey {
	t.Helper()
	n, err := base64.RawURLEncoding.DecodeString(jwk["n"])
	require.NoError(t, err)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk["e"])
	require.NoError(t, err)
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: e}
}

func TestActionsOIDCTokenIntegration(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		accessToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repo := createActionsTestRepo(t, accessToken, "actions-oidc", false)
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
    runs-on: ubuntu-latest
    steps:
      - run: echo oidc
`
		workflowPath := ".gitea/workflows/oidc.yml"
		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+workflowPath, workflowContent)
		createWorkflowFile(t, accessToken, user2.Name, repo.Name, workflowPath, opts)

		task := runner.fetchTask(t)
		contextMap := task.Context.AsMap()
		requestURL, ok := contextMap["actions_id_token_request_url"].(string)
		require.True(t, ok)
		requestToken, ok := contextMap["actions_id_token_request_token"].(string)
		require.True(t, ok)
		parsedURL, err := url.Parse(requestURL)
		require.NoError(t, err)
		assert.Equal(t, "/api/actions/oidc/token", parsedURL.Path)
		assert.Empty(t, parsedURL.RawQuery)
		assert.True(t, strings.HasSuffix(requestURL, "/api/actions/oidc/token?"))

		resp := MakeRequest(t, NewRequest(t, http.MethodGet, "/api/actions/oidc/.well-known/openid-configuration"), http.StatusOK)
		var discovery map[string]any
		DecodeJSON(t, resp, &discovery)
		assert.Equal(t, actions_service.OIDCIssuer(), discovery["issuer"])
		assert.NotContains(t, discovery, "token_endpoint")
		jwksURI, ok := discovery["jwks_uri"].(string)
		require.True(t, ok)
		assert.Equal(t, actions_service.OIDCIssuer()+"/jwks", jwksURI)
		parsedJWKSURL, err := url.Parse(jwksURI)
		require.NoError(t, err)
		claimsSupported, ok := discovery["claims_supported"].([]any)
		require.True(t, ok)
		assert.NotContains(t, claimsSupported, "environment")

		resp = MakeRequest(t, NewRequest(t, http.MethodGet, parsedJWKSURL.RequestURI()), http.StatusOK)
		var jwks struct {
			Keys []map[string]string `json:"keys"`
		}
		DecodeJSON(t, resp, &jwks)
		require.Len(t, jwks.Keys, 1)
		assert.Equal(t, "RSA", jwks.Keys[0]["kty"])
		assert.Equal(t, "sig", jwks.Keys[0]["use"])
		assert.NotContains(t, jwks.Keys[0], "d")

		badRequest := NewRequest(t, http.MethodGet, parsedURL.RequestURI())
		badRequest.Header.Set("Authorization", "Bearer invalid")
		resp = MakeRequest(t, badRequest, http.StatusUnauthorized)
		assert.Equal(t, `Bearer realm="Gitea Actions OIDC"`, resp.Header().Get("WWW-Authenticate"))

		audienceURL, err := url.Parse(requestURL + "&audience=integration-test")
		require.NoError(t, err)
		tokenRequest := NewRequest(t, http.MethodGet, audienceURL.RequestURI())
		tokenRequest.Header.Set("Authorization", "bearer "+requestToken)
		resp = MakeRequest(t, tokenRequest, http.StatusOK)
		assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
		assert.Equal(t, "no-cache", resp.Header().Get("Pragma"))
		var tokenResponse struct {
			Value string `json:"value"`
		}
		DecodeJSON(t, resp, &tokenResponse)
		require.NotEmpty(t, tokenResponse.Value)

		claims := &oidcIntegrationClaims{}
		parsed, err := jwt.ParseWithClaims(tokenResponse.Value, claims, func(token *jwt.Token) (any, error) {
			assert.Equal(t, jwks.Keys[0]["alg"], token.Method.Alg())
			assert.Equal(t, jwks.Keys[0]["kid"], token.Header["kid"])
			return oidcJWKPublicKey(t, jwks.Keys[0]), nil
		})
		require.NoError(t, err)
		require.True(t, parsed.Valid)

		assert.Equal(t, actions_service.OIDCIssuer(), claims.Issuer)
		assert.Equal(t, jwt.ClaimStrings{"integration-test"}, claims.Audience)
		assert.Equal(t, 5*time.Minute, claims.ExpiresAt.Sub(claims.IssuedAt.Time))
		assert.Equal(t, repo.FullName, claims.Repository)
		assert.Equal(t, "oidc-job", claims.JobID)
		assert.Equal(t, "OIDC", claims.Workflow)
		assert.Equal(t, repo.FullName, claims.WorkflowRepository)
		assert.Equal(t, claims.RepositoryID, claims.WorkflowRepositoryID)
		assert.Equal(t, "self-hosted", claims.RunnerEnvironment)
		assert.Equal(t, "repo:"+claims.RepositoryOwnerID+"/"+claims.RepositoryID+":ref:"+claims.Ref, claims.Subject)

		refValue, ok := contextMap["ref"].(string)
		require.True(t, ok)
		shaValue, ok := contextMap["sha"].(string)
		require.True(t, ok)
		assert.Equal(t, refValue, claims.Ref)
		assert.Equal(t, shaValue, claims.SHA)
		assert.Equal(t, repo.FullName+"/"+workflowPath+"@"+refValue, claims.WorkflowRef)
		assert.Equal(t, shaValue, claims.WorkflowSHA)
		assert.Empty(t, claims.JobWorkflowRef)
		assert.Empty(t, claims.JobWorkflowSHA)

		_, taskJob, _ := getTaskAndJobAndRunByTaskID(t, task.Id)
		require.NotNil(t, taskJob.TokenPermissions)
		taskJob.TokenPermissions.IDTokenAccessMode = perm.AccessModeNone
		_, err = db.GetEngine(t.Context()).ID(taskJob.ID).Cols("token_permissions").Update(taskJob)
		require.NoError(t, err)
		revokedRequest := NewRequest(t, http.MethodGet, audienceURL.RequestURI())
		revokedRequest.Header.Set("Authorization", "Bearer "+requestToken)
		resp = MakeRequest(t, revokedRequest, http.StatusUnauthorized)
		assert.Equal(t, `Bearer realm="Gitea Actions OIDC"`, resp.Header().Get("WWW-Authenticate"))

		runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		stoppedRequest := NewRequest(t, http.MethodGet, audienceURL.RequestURI())
		stoppedRequest.Header.Set("Authorization", "Bearer "+requestToken)
		resp = MakeRequest(t, stoppedRequest, http.StatusUnauthorized)
		assert.Equal(t, `Bearer realm="Gitea Actions OIDC"`, resp.Header().Get("WWW-Authenticate"))
	})
}
