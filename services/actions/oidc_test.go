// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/rand"
	"crypto/rsa"
	"strconv"
	"strings"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOIDCTestTask(t *testing.T) *actions_model.ActionTask {
	t.Helper()
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	run := &actions_model.ActionRun{
		Title:             "OIDC test",
		RepoID:            repo.ID,
		OwnerID:           repo.OwnerID,
		WorkflowID:        "oidc.yml",
		WorkflowPath:      ".gitea/workflows/oidc.yml",
		Index:             990001,
		TriggerUserID:     1,
		Ref:               "refs/heads/main",
		CommitSHA:         "source-sha",
		Event:             "push",
		EventPayload:      "{}",
		TriggerEvent:      "push",
		Status:            actions_model.StatusRunning,
		WorkflowRepoID:    repo.ID,
		WorkflowCommitSHA: "workflow-sha",
	}
	require.NoError(t, db.Insert(t.Context(), run))
	permissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	permissions.IDTokenAccessMode = perm.AccessModeWrite
	job := &actions_model.ActionRunJob{
		RunID:                   run.ID,
		RepoID:                  repo.ID,
		OwnerID:                 repo.OwnerID,
		CommitSHA:               run.CommitSHA,
		Name:                    "build",
		Attempt:                 1,
		WorkflowPayload:         []byte("name: Build\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: 'true'\n"),
		JobID:                   "build",
		Status:                  actions_model.StatusRunning,
		TokenPermissions:        &permissions,
		WorkflowSourceRepoID:    repo.ID,
		WorkflowSourceCommitSHA: "workflow-sha",
	}
	require.NoError(t, db.Insert(t.Context(), job))
	task := &actions_model.ActionTask{
		JobID:     job.ID,
		Attempt:   1,
		Status:    actions_model.StatusRunning,
		RepoID:    repo.ID,
		OwnerID:   repo.OwnerID,
		CommitSHA: run.CommitSHA,
	}
	require.NoError(t, db.Insert(t.Context(), task))
	require.NoError(t, task.LoadAttributes(t.Context()))
	return task
}

func useOIDCTestSigningKey(t *testing.T) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signingKey, err := oauth2_provider.CreateJWTSigningKey("RS256", key)
	require.NoError(t, err)
	oldSigningKey := oauth2_provider.DefaultSigningKey
	oldEnabled := setting.OAuth2.Enabled
	oauth2_provider.DefaultSigningKey = signingKey
	setting.OAuth2.Enabled = true
	t.Cleanup(func() {
		oauth2_provider.DefaultSigningKey = oldSigningKey
		setting.OAuth2.Enabled = oldEnabled
	})
}

func parseOIDCTestToken(t *testing.T, token string) *actionsOIDCClaims {
	t.Helper()
	claims := &actionsOIDCClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != oauth2_provider.DefaultSigningKey.SigningMethod().Alg() {
			return nil, jwt.ErrSignatureInvalid
		}
		return oauth2_provider.DefaultSigningKey.VerifyKey(), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	return claims
}

func TestActionsOIDCTokenClaims(t *testing.T) {
	task := newOIDCTestTask(t)
	useOIDCTestSigningKey(t)

	before := time.Now().UTC()
	token, err := CreateOIDCToken(t.Context(), task.ID, "sigstore")
	require.NoError(t, err)
	claims := parseOIDCTestToken(t, token)
	after := time.Now().UTC()

	run := task.Job.Run
	expectedSubject := "repo:" + strings.TrimPrefix(claims.RepositoryOwnerID, "+") + "/" + claims.RepositoryID + ":ref:refs/heads/main"
	assert.Equal(t, OIDCIssuer(), claims.Issuer)
	assert.Equal(t, expectedSubject, claims.Subject)
	assert.Equal(t, jwt.ClaimStrings{"sigstore"}, claims.Audience)
	assert.WithinRange(t, claims.IssuedAt.Time, before.Add(-time.Second), after.Add(time.Second))
	assert.Equal(t, claims.IssuedAt.Time, claims.NotBefore.Time)
	assert.Equal(t, actionsOIDCTokenExpiry, claims.ExpiresAt.Sub(claims.IssuedAt.Time))
	assert.NotEmpty(t, claims.ID)
	assert.Equal(t, run.TriggerUser.Name, claims.Actor)
	assert.Equal(t, "1", claims.ActorID)
	assert.Equal(t, run.Repo.FullName(), claims.Repository)
	assert.Equal(t, "4", claims.RepositoryID)
	assert.Equal(t, "Build", claims.Workflow)
	assert.Equal(t, run.Repo.FullName(), claims.WorkflowRepository)
	assert.Equal(t, strconv.FormatInt(run.Repo.ID, 10), claims.WorkflowRepositoryID)
	assert.Equal(t, run.Repo.FullName()+"/.gitea/workflows/oidc.yml@refs/heads/main", claims.WorkflowRef)
	assert.Equal(t, "workflow-sha", claims.WorkflowSHA)
	assert.Empty(t, claims.JobWorkflowRef)
	assert.Empty(t, claims.JobWorkflowSHA)
	assert.Equal(t, "source-sha", claims.SHA)
	assert.Equal(t, "branch", claims.RefType)
	assert.Equal(t, "self-hosted", claims.RunnerEnvironment)
	assert.NotEqual(t, claims.SHA, claims.WorkflowSHA)

	var rawClaims map[string]any
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := jwt.NewParser().DecodeSegment(parts[1])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &rawClaims))
	assert.NotContains(t, rawClaims, "environment")
	assert.NotContains(t, rawClaims, "job_attempt")
	assert.NotContains(t, rawClaims, "job_workflow_repository")
	assert.NotContains(t, rawClaims, "job_workflow_repository_id")
	assert.NotContains(t, rawClaims, "job_workflow_ref")
	assert.NotContains(t, rawClaims, "job_workflow_sha")
}

func TestActionsOIDCRunAttemptUsesGiteaContext(t *testing.T) {
	task := newOIDCTestTask(t)
	run := task.Job.Run
	attempt := &actions_model.ActionRunAttempt{
		RepoID: run.RepoID, RunID: run.ID, Attempt: 3, TriggerUserID: run.TriggerUserID, Status: actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), attempt))
	run.LatestAttemptID = attempt.ID
	task.Job.Attempt = 7
	task.Attempt = 9

	gitCtx := GenerateGiteaContext(t.Context(), run, nil, task.Job)
	claims, err := createOIDCClaims(t.Context(), task, "audience", time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, "3", gitCtx["run_attempt"])
	assert.Equal(t, gitCtx["run_attempt"], claims.RunAttempt)
}

func TestActionsOIDCSubjects(t *testing.T) {
	task := newOIDCTestTask(t)
	run := task.Job.Run
	repositoryIdentity := strconv.FormatInt(run.Repo.OwnerID, 10) + "/" + strconv.FormatInt(run.Repo.ID, 10)

	tests := []struct {
		name         string
		triggerEvent string
		ref          string
		expected     string
	}{
		{name: "branch", triggerEvent: "push", ref: "refs/heads/main", expected: "repo:" + repositoryIdentity + ":ref:refs/heads/main"},
		{name: "tag", triggerEvent: "push", ref: "refs/tags/v1", expected: "repo:" + repositoryIdentity + ":ref:refs/tags/v1"},
		{name: "pull request", triggerEvent: actions_module.GithubEventPullRequest, ref: "refs/pull/7/merge", expected: "repo:" + repositoryIdentity + ":pull_request"},
		{name: "pull request target", triggerEvent: actions_module.GithubEventPullRequestTarget, ref: "refs/heads/main", expected: "repo:" + repositoryIdentity + ":pull_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run.TriggerEvent = test.triggerEvent
			run.Ref = test.ref
			subject, err := buildOIDCSubject(run, test.ref)
			require.NoError(t, err)
			assert.Equal(t, test.expected, subject)
		})
	}
	assert.Equal(t, "value%3Awith%25delimiter", escapeOIDCSubjectValue("value:with%delimiter"))
}

func TestActionsOIDCRefs(t *testing.T) {
	task := newOIDCTestTask(t)
	run := task.Job.Run
	payload, err := json.Marshal(api.PullRequestPayload{PullRequest: &api.PullRequest{
		Base: &api.PRBranchInfo{Ref: "main", Sha: "base-sha"},
		Head: &api.PRBranchInfo{Ref: "feature", Sha: "head-sha"},
	}})
	require.NoError(t, err)
	run.Event = "pull_request"
	run.EventPayload = string(payload)
	run.TriggerEvent = actions_module.GithubEventPullRequestTarget
	gitCtx := GenerateGiteaContext(t.Context(), run, nil, task.Job)
	assert.Equal(t, "refs/heads/main", gitCtx["ref"])
	assert.Equal(t, "base-sha", gitCtx["sha"])
	assert.Equal(t, "main", gitCtx["base_ref"])
	assert.Equal(t, "feature", gitCtx["head_ref"])
}

func TestActionsOIDCReusableWorkflowIdentity(t *testing.T) {
	task := newOIDCTestTask(t)
	sourceRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	permissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	permissions.IDTokenAccessMode = perm.AccessModeWrite
	caller := &actions_model.ActionRunJob{
		RunID:                   task.Job.RunID,
		RepoID:                  task.Job.RepoID,
		OwnerID:                 task.Job.OwnerID,
		WorkflowPayload:         task.Job.WorkflowPayload,
		TokenPermissions:        &permissions,
		WorkflowSourceRepoID:    task.Job.Run.WorkflowRepoID,
		WorkflowSourceCommitSHA: task.Job.Run.WorkflowCommitSHA,
		IsReusableCaller:        true,
		CallUses:                "owner/repo/.gitea/workflows/reusable.yml@v1",
	}
	require.NoError(t, db.Insert(t.Context(), caller))
	task.Job.ParentJobID = caller.ID
	task.Job.WorkflowSourceRepoID = sourceRepo.ID
	task.Job.WorkflowSourceCommitSHA = "reusable-sha"
	_, err := db.GetEngine(t.Context()).ID(task.Job.ID).Cols("parent_job_id", "workflow_source_repo_id", "workflow_source_commit_sha").Update(task.Job)
	require.NoError(t, err)

	claims, err := createOIDCClaims(t.Context(), task, "audience", time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, task.Job.Run.Repo.FullName()+"/.gitea/workflows/oidc.yml@refs/heads/main", claims.WorkflowRef)
	assert.Equal(t, task.Job.Run.WorkflowCommitSHA, claims.WorkflowSHA)
	assert.Equal(t, sourceRepo.FullName()+"/.gitea/workflows/reusable.yml@v1", claims.JobWorkflowRef)
	assert.Equal(t, "reusable-sha", claims.JobWorkflowSHA)
	assert.Equal(t, sourceRepo.FullName(), claims.JobWorkflowRepository)
	assert.Equal(t, strconv.FormatInt(sourceRepo.ID, 10), claims.JobWorkflowRepositoryID)
}

func TestActionsOIDCAudience(t *testing.T) {
	task := newOIDCTestTask(t)
	useOIDCTestSigningKey(t)

	token, err := CreateOIDCToken(t.Context(), task.ID, "")
	require.NoError(t, err)
	claims := parseOIDCTestToken(t, token)
	assert.Equal(t, jwt.ClaimStrings{strings.TrimSuffix(setting.AppURL, "/") + "/" + task.Job.Run.Repo.OwnerName}, claims.Audience)

	for _, audience := range []string{" leading", "trailing ", "line\nbreak", strings.Repeat("a", actionsOIDCMaxAudienceLength+1)} {
		_, err := CreateOIDCToken(t.Context(), task.ID, audience)
		assert.ErrorIs(t, err, ErrOIDCInvalidAudience)
	}
}

func TestActionsOIDCEnabledWithoutOAuth2Provider(t *testing.T) {
	useOIDCTestSigningKey(t)
	oldActionsEnabled := setting.Actions.Enabled
	setting.Actions.Enabled = true
	setting.OAuth2.Enabled = false
	t.Cleanup(func() { setting.Actions.Enabled = oldActionsEnabled })
	assert.True(t, OIDCEnabled())
}

func TestActionsOIDCAuthorization(t *testing.T) {
	task := newOIDCTestTask(t)
	useOIDCTestSigningKey(t)

	allowed, err := TaskAllowsOIDCToken(t.Context(), task)
	require.NoError(t, err)
	assert.True(t, allowed)

	task.Job.TokenPermissions.IDTokenAccessMode = perm.AccessModeNone
	_, err = db.GetEngine(t.Context()).ID(task.Job.ID).Cols("token_permissions").Update(task.Job)
	require.NoError(t, err)
	_, err = CreateOIDCToken(t.Context(), task.ID, "audience")
	assert.ErrorIs(t, err, ErrOIDCPermissionDenied)

	task.Job.TokenPermissions.IDTokenAccessMode = perm.AccessModeWrite
	_, err = db.GetEngine(t.Context()).ID(task.Job.ID).Cols("token_permissions").Update(task.Job)
	require.NoError(t, err)
	task.IsForkPullRequest = true
	require.NoError(t, actions_model.UpdateTask(t.Context(), task, "is_fork_pull_request"))
	_, err = CreateOIDCToken(t.Context(), task.ID, "audience")
	assert.ErrorIs(t, err, ErrOIDCPermissionDenied)

	task.Status = actions_model.StatusSuccess
	_, err = db.GetEngine(t.Context()).ID(task.ID).Cols("status").Update(task)
	require.NoError(t, err)
	_, err = CreateOIDCToken(t.Context(), task.ID, "audience")
	assert.ErrorIs(t, err, ErrOIDCTaskNotRunning)

	_, err = CreateOIDCToken(t.Context(), -1, "audience")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrOIDCPermissionDenied)
}

func TestActionsOIDCTaskContext(t *testing.T) {
	task := newOIDCTestTask(t)
	useOIDCTestSigningKey(t)
	task.GenerateAndFillToken()

	contextStruct, err := generateTaskContext(t.Context(), task)
	require.NoError(t, err)
	contextMap := contextStruct.AsMap()
	assert.Equal(t, OIDCTokenRequestURL(), contextMap["actions_id_token_request_url"])
	assert.NotEmpty(t, contextMap["actions_id_token_request_token"])

	task.IsForkPullRequest = true
	contextStruct, err = generateTaskContext(t.Context(), task)
	require.NoError(t, err)
	contextMap = contextStruct.AsMap()
	assert.NotContains(t, contextMap, "actions_id_token_request_url")
	assert.NotContains(t, contextMap, "actions_id_token_request_token")

	task.IsForkPullRequest = false
	task.Job.TokenPermissions.IDTokenAccessMode = perm.AccessModeNone
	contextStruct, err = generateTaskContext(t.Context(), task)
	require.NoError(t, err)
	contextMap = contextStruct.AsMap()
	assert.NotContains(t, contextMap, "actions_id_token_request_url")
	assert.NotContains(t, contextMap, "actions_id_token_request_token")

	task.Job.TokenPermissions = nil
	contextStruct, err = generateTaskContext(t.Context(), task)
	require.NoError(t, err)
	contextMap = contextStruct.AsMap()
	assert.NotContains(t, contextMap, "actions_id_token_request_url")
	assert.NotContains(t, contextMap, "actions_id_token_request_token")
}
