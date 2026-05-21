// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oidcTestClaims struct {
	jwt.RegisteredClaims
	Actor                string `json:"actor"`
	Repository           string `json:"repository"`
	RunID                int64  `json:"run_id"`
	JobID                string `json:"job_id"`
	Ref                  string `json:"ref"`
	WorkflowRef          string `json:"workflow_ref"`
	WorkflowSHA          string `json:"workflow_sha"`
	JobWorkflowRef       string `json:"job_workflow_ref"`
	JobWorkflowSHA       string `json:"job_workflow_sha"`
	RunnerEnvironment    string `json:"runner_environment"`
	RepositoryVisibility string `json:"repository_visibility"`
}

func TestActionsOIDCTokenClaims(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	require.NoError(t, oauth2_provider.InitSigningKey())

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 51})
	require.NoError(t, task.LoadJob(t.Context()))

	perms := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	perms.IDTokenAccessMode = perm.AccessModeWrite
	task.Job.TokenPermissions = &perms
	_, err := db.GetEngine(t.Context()).ID(task.Job.ID).Cols("token_permissions").Update(task.Job)
	require.NoError(t, err)

	allowed, err := TaskAllowsOIDCToken(t.Context(), task)
	require.NoError(t, err)
	assert.True(t, allowed)

	token, err := CreateOIDCToken(t.Context(), task, "test-audience")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	var claims oidcTestClaims
	signingKey := oauth2_provider.DefaultSigningKey
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if t.Method == nil || t.Method.Alg() != signingKey.SigningMethod().Alg() {
			return nil, jwt.ErrSignatureInvalid
		}
		return signingKey.VerifyKey(), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	assert.Equal(t, OIDCIssuer(), claims.Issuer)
	assert.Contains(t, claims.Audience, "test-audience")
	assert.Equal(t, task.Job.Run.Repo.FullName(), claims.Repository)
	assert.Equal(t, task.Job.Run.TriggerUser.Name, claims.Actor)
	assert.Equal(t, task.Job.Run.ID, claims.RunID)
	assert.Equal(t, task.Job.JobID, claims.JobID)
	assert.Equal(t, task.Job.Run.Ref, claims.Ref)
	assert.Equal(t, "self-hosted", claims.RunnerEnvironment)
	assert.Equal(t, buildWorkflowRef(task.Job.Run), claims.WorkflowRef)
	assert.Equal(t, task.Job.Run.CommitSHA, claims.WorkflowSHA)
	assert.Equal(t, buildWorkflowRef(task.Job.Run), claims.JobWorkflowRef)
	assert.Equal(t, task.Job.Run.CommitSHA, claims.JobWorkflowSHA)
	assert.Equal(t, repositoryVisibility(task.Job.Run.Repo), claims.RepositoryVisibility)

	perms.IDTokenAccessMode = perm.AccessModeNone
	task.Job.TokenPermissions = &perms
	_, err = db.GetEngine(t.Context()).ID(task.Job.ID).Cols("token_permissions").Update(task.Job)
	require.NoError(t, err)

	allowed, err = TaskAllowsOIDCToken(t.Context(), task)
	require.NoError(t, err)
	assert.False(t, allowed)
}
