// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCodespaceQueuesCreateWhenManagerMatches(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	insertServiceDefaultDevContainerTemplate(t)
	insertServiceManagerWithTags(t, 2, "default")
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	result, err := createConfirmedCodespace(t, CreateCodespaceOptions{
		User:    user,
		Repo:    repo,
		RefType: "branch",
		RefName: "master",
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusCreating, result.Status)
	assert.Equal(t, "default", result.EnvironmentTag)

	row := loadServiceCodespace(t, result.CodespaceUUID)
	assert.Equal(t, user.ID, row.UserID)
	assert.Equal(t, repo.ID, row.RepoID)
	assert.Equal(t, "branch", row.RefType)
	assert.Equal(t, "master", row.RefName)
	assert.NotEmpty(t, row.CommitSHA)
	assert.Equal(t, codespace_model.DevContainerSourceTemplate, row.DevContainerSource)
	assert.Empty(t, row.DevContainerPath)
	assert.JSONEq(t, `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`, row.DevContainerContent)
	assert.Equal(t, codespace_model.OperationCreate, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
	assert.EqualValues(t, 1, row.OperationRVersion)
}

func TestCreateCodespaceUsesCreatorManagerForOrganizationRepository(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	insertServiceDefaultDevContainerTemplate(t)
	insertServiceManagerWithTags(t, 2, "default")
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	result, err := createConfirmedCodespace(t, CreateCodespaceOptions{
		User:    user,
		Repo:    repo,
		RefType: "branch",
		RefName: "master",
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.StatusCreating, result.Status)

	row := loadServiceCodespace(t, result.CodespaceUUID)
	assert.Equal(t, user.ID, row.UserID)
	assert.Equal(t, repo.ID, row.RepoID)
}

func TestCreateCodespaceRequiresAvailableEnvironment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	insertServiceDefaultDevContainerTemplate(t)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	plan, err := PrepareCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo})
	require.NoError(t, err)
	assert.Empty(t, plan.Environments)
	_, err = CreateCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo, RequestHash: plan.RequestHash})
	require.ErrorIs(t, err, ErrCreateEnvironmentUnavailable)
}

func TestListVisibleCreateEnvironmentsMergesManagerDeclarations(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	site := insertServiceManagerWithTags(t, 0, "standard", "gpu")
	markServiceManagerOnline(t, site, `[{"tag":"standard","description":"General development"},{"tag":"gpu","description":"Accelerated"}]`)
	personal := insertServiceManagerWithTags(t, 2, "standard", "personal")
	markServiceManagerOnline(t, personal, `[{"tag":"standard","description":"General development"},{"tag":"personal"}]`)
	foreign := insertServiceManagerWithTags(t, 4, "foreign")
	markServiceManagerOnline(t, foreign, `[{"tag":"foreign"}]`)

	environments, err := listVisibleCreateEnvironments(t.Context(), 2, "standard")
	require.NoError(t, err)
	require.Len(t, environments, 3)
	assert.Equal(t, CreateEnvironmentOption{Tag: "gpu", Description: "Accelerated", Site: true, Online: true}, environments[0])
	assert.Equal(t, CreateEnvironmentOption{Tag: "personal", Personal: true, Online: true}, environments[1])
	assert.Equal(t, CreateEnvironmentOption{Tag: "standard", Description: "General development", Site: true, Personal: true, Online: true, Selected: true}, environments[2])
}

func TestCreateCodespaceRequiresExplicitEnvironmentSelection(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	insertServiceDefaultDevContainerTemplate(t)
	insertServiceManagerWithTags(t, 2, "default")
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	plan, err := PrepareCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo})
	require.NoError(t, err)

	countBefore, err := db.GetEngine(t.Context()).Count(new(codespace_model.Codespace))
	require.NoError(t, err)
	_, err = CreateCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo, RequestHash: plan.RequestHash})
	require.ErrorIs(t, err, ErrCreateEnvironmentUnavailable)
	countAfter, err := db.GetEngine(t.Context()).Count(new(codespace_model.Codespace))
	require.NoError(t, err)
	assert.Equal(t, countBefore, countAfter)
}

func TestCreateCodespaceRejectsDisabledCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	_, err := PrepareCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo})
	require.ErrorIs(t, err, ErrCreateStateUnavailable)
}

func TestCreateCodespaceRequiresUserAndRepository(t *testing.T) {
	for name, opts := range map[string]CreateCodespaceOptions{
		"user":       {RequestHash: "confirmed", Repo: &repo_model.Repository{ID: 1}},
		"repository": {RequestHash: "confirmed", User: &user_model.User{ID: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := CreateCodespace(t.Context(), opts)
			require.Error(t, err)
		})
	}
}

func TestCreateCodespacePersistsPullRefAndValidatesGitProtocol(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolSSH, false, false, []string{
		"gitea.example.com " + testGitSSHPublicKey,
	})

	insertServiceDefaultDevContainerTemplate(t)
	insertServiceManagerWithTags(t, 0, "default")
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	result, err := createConfirmedCodespace(t, CreateCodespaceOptions{
		User:    user,
		Repo:    repo,
		RefType: "pull",
		RefName: "3",
	})
	require.NoError(t, err)
	plan, err := PrepareCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo, RefType: "pull", RefName: "3"})
	require.NoError(t, err)
	assert.Equal(t, "3", plan.RefName)
	require.NotNil(t, plan.PullRequest)
	assert.Equal(t, int64(3), plan.PullRequest.Index)
	assert.Equal(t, "user2/repo1", plan.PullRequest.HeadRepoFullName)
	assert.Equal(t, "branch2", plan.PullRequest.HeadBranch)

	row := loadServiceCodespace(t, result.CodespaceUUID)
	assert.Equal(t, "pull", row.RefType)
	assert.Equal(t, "refs/pull/3/head", row.RefName)
	assert.NotEmpty(t, row.CommitSHA)
}

func TestPrepareCodespaceRequiresForkPullSourceAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	require.NoError(t, db.Insert(t.Context(), &git_model.Branch{RepoID: 11, Name: "branch2"}))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	_, _, runErr := gitcmd.NewCommand("fast-import").WithRepo(repo.CodeStorageRepo()).WithStdinBytes([]byte(`commit refs/heads/master
committer user11 <user11@example.com> 1714310400 +0000
data <<COMMIT
Add a fork pull request development container
COMMIT
from refs/heads/master^0
M 100644 inline .devcontainer.json
data <<CONFIG
{"image":"debian:12","secrets":{"fork_secret":{"description":"Fork secret"}}}
CONFIG
`)).RunStdString(t.Context())
	require.NoError(t, runErr)
	_, _, runErr = gitcmd.NewCommand("update-ref", "refs/pull/1/head", "refs/heads/master").WithRepo(repo.CodeStorageRepo()).RunStdString(t.Context())
	require.NoError(t, runErr)
	plan, err := PrepareCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo, RefType: "pull", RefName: "1"})
	require.NoError(t, err)
	require.NotNil(t, plan.PullRequest)
	assert.True(t, plan.PullRequest.IsFork)
	assert.False(t, plan.SecretInjectionAllowed)
	assert.Empty(t, plan.AvailableSecrets)
	assert.Equal(t, []CreateRecommendedSecret{{Name: "FORK_SECRET", Description: "Fork secret"}}, plan.RecommendedSecrets)
	assert.Equal(t, "user13/repo11", plan.PullRequest.HeadRepoFullName)
	require.Len(t, plan.Permissions, 1)
	assert.Equal(t, "user13/repo11", plan.Permissions[0].RepositoryFullName)
	assert.Equal(t, unit.TypeCode, plan.Permissions[0].UnitType)
	assert.Equal(t, perm.AccessModeWrite, plan.Permissions[0].Mode)
	assert.True(t, plan.Permissions[0].Required)

	insertServiceManagerWithTags(t, user.ID, "default")
	_, err = CreateCodespace(t.Context(), CreateCodespaceOptions{
		User: user, Repo: repo, RefType: "pull", RefName: "1",
		RequestHash:             plan.RequestHash,
		EnvironmentTag:          "default",
		RecommendedSecretValues: map[string]string{"FORK_SECRET": "submitted-value"},
	})
	require.NoError(t, err)
	unittest.AssertNotExistsBean(t, &codespace_model.UserSecret{UserID: user.ID, Name: "FORK_SECRET"})
}

func TestDevContainerSelectionAndRecommendedSecrets(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, CreateUserSecret(t.Context(), user, "runtime_token", "secret-value", false, []int64{repo.ID}))
	_, _, runErr := gitcmd.NewCommand("fast-import").WithRepo(repo.CodeStorageRepo()).WithStdinBytes([]byte(`commit refs/heads/master
committer user2 <user2@example.com> 1714310400 +0000
data <<COMMIT
Add development containers
COMMIT
from refs/heads/master^0
M 100644 inline .devcontainer.json
data <<ROOT
{"name":"Root","image":"debian:12","secrets":{"database_password":{"description":"Database password"}}}
ROOT
M 100644 inline .devcontainer/node/devcontainer.json
data <<NODE
{"name":"Node","image":"node:24"}
NODE
`)).RunStdString(t.Context())
	require.NoError(t, runErr)

	rootPlan, err := PrepareCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo})
	require.NoError(t, err)
	require.Len(t, rootPlan.DevContainerOptions, 2)
	assert.Equal(t, devContainerRootPath, rootPlan.DevContainerOptions[0].Path)
	assert.True(t, rootPlan.DevContainerOptions[0].Selected)
	assert.Equal(t, []CreateRecommendedSecret{{Name: "DATABASE_PASSWORD", Description: "Database password"}}, rootPlan.RecommendedSecrets)
	assert.True(t, rootPlan.SecretInjectionAllowed)
	assert.Equal(t, []CreateSecretSummary{{Name: "RUNTIME_TOKEN"}}, rootPlan.AvailableSecrets)

	insertServiceManagerWithTags(t, user.ID, "default")
	secretOptions := CreateCodespaceOptions{
		User: user, Repo: repo, RequestHash: rootPlan.RequestHash + "changed",
		EnvironmentTag:          "default",
		RecommendedSecretValues: map[string]string{"DATABASE_PASSWORD": "database-value"},
	}
	_, err = CreateCodespace(t.Context(), secretOptions)
	require.ErrorIs(t, err, ErrCreateRequestChanged)
	unittest.AssertNotExistsBean(t, &codespace_model.UserSecret{UserID: user.ID, Name: "DATABASE_PASSWORD"})

	secretOptions.RequestHash = rootPlan.RequestHash
	_, err = CreateCodespace(t.Context(), secretOptions)
	require.NoError(t, err)
	resolvedSecrets, err := resolveUserSecretsForRepository(t.Context(), user.ID, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, []RuntimeSecret{
		{Name: "DATABASE_PASSWORD", Value: "database-value"},
		{Name: "RUNTIME_TOKEN", Value: "secret-value"},
	}, resolvedSecrets)

	nodePath := ".devcontainer/node/devcontainer.json"
	nodePlan, err := PrepareCodespace(t.Context(), CreateCodespaceOptions{User: user, Repo: repo, DevContainerSelection: nodePath})
	require.NoError(t, err)
	assert.Equal(t, nodePath, nodePlan.DevContainerOptions[1].Path)
	assert.True(t, nodePlan.DevContainerOptions[1].Selected)
	assert.NotEqual(t, rootPlan.RequestHash, nodePlan.RequestHash)
}

func TestNormalizePermissionGrants(t *testing.T) {
	permissions := []CreatePermissionRequest{
		{RepositoryFullName: "owner/source", UnitName: "code", Mode: perm.AccessModeWrite, ModeName: "write", FormName: "permission_1_1", Required: true},
		{RepositoryFullName: "owner/dependency", UnitName: "code", Mode: perm.AccessModeWrite, ModeName: "write", FormName: "permission_2_1"},
		{RepositoryFullName: "owner/dependency", UnitName: "issues", Mode: perm.AccessModeRead, ModeName: "read", FormName: "permission_2_2"},
	}
	grants, err := normalizePermissionGrants(permissions, map[string]string{
		"permission_1_1": "none",
		"permission_2_1": "read",
		"permission_2_2": "none",
	})
	require.NoError(t, err)
	assert.Equal(t, []perm.AccessMode{perm.AccessModeWrite, perm.AccessModeRead, perm.AccessModeNone}, grants)

	_, err = normalizePermissionGrants(permissions[2:], map[string]string{"permission_2_2": "write"})
	require.Error(t, err)
}

func TestCreateCodespacePersistsPermissionAuthorization(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	insertServiceManagerWithTags(t, 2, "default")
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_, _, runErr := gitcmd.NewCommand("fast-import").WithRepo(repo.CodeStorageRepo()).WithStdinBytes([]byte(`commit refs/heads/master
committer user2 <user2@example.com> 1714310400 +0000
data <<COMMIT
Add Codespace permissions
COMMIT
from refs/heads/master^0
M 100644 inline .devcontainer/devcontainer.json
data <<CONFIG
{
  // Gitea reads permissions only from the selected Dev Container configuration.
  "image": "debian:12",
  "customizations": {
    "gitea": {
      "repositories": {
        "user2/repo2": {
          "permissions": {
            "code": "write",
            "issues": "read",
          },
        },
      },
    },
  },
}
CONFIG
`)).RunStdString(t.Context())
	require.NoError(t, runErr)
	opts := CreateCodespaceOptions{User: user, Repo: repo, RefType: "branch", RefName: repo.DefaultBranch}
	plan, err := PrepareCodespace(t.Context(), opts)
	require.NoError(t, err)
	require.Len(t, plan.Permissions, 2)

	authorizationsBefore, err := db.GetEngine(t.Context()).Count(new(codespace_model.PermissionAuthorization))
	require.NoError(t, err)
	_, err = CreateCodespace(t.Context(), CreateCodespaceOptions{
		User: user, Repo: repo, RefType: "branch", RefName: repo.DefaultBranch,
		RequestHash: plan.RequestHash + "changed",
	})
	require.ErrorIs(t, err, ErrCreateRequestChanged)
	authorizationsAfter, err := db.GetEngine(t.Context()).Count(new(codespace_model.PermissionAuthorization))
	require.NoError(t, err)
	assert.Equal(t, authorizationsBefore, authorizationsAfter)

	opts.RequestHash = plan.RequestHash
	opts.EnvironmentTag = "default"
	opts.PermissionGrants = map[string]string{}
	for _, permission := range plan.Permissions {
		opts.PermissionGrants[permission.FormName] = map[string]string{"code": "read", "issues": "none"}[permission.UnitName]
	}
	first, err := CreateCodespace(t.Context(), opts)
	require.NoError(t, err)
	firstCodespace := loadServiceCodespace(t, first.CodespaceUUID)
	assert.Equal(t, codespace_model.DevContainerSourceRepository, firstCodespace.DevContainerSource)
	assert.Equal(t, devContainerPrimaryPath, firstCodespace.DevContainerPath)
	assert.Empty(t, firstCodespace.DevContainerContent)
	require.Positive(t, firstCodespace.PermissionAuthorizationID)
	authorization := unittest.AssertExistsAndLoadBean(t, &codespace_model.PermissionAuthorization{ID: firstCodespace.PermissionAuthorizationID})
	assert.Equal(t, user.ID, authorization.UserID)
	assert.Equal(t, repo.ID, authorization.SourceRepoID)

	var rules []*codespace_model.PermissionRepository
	require.NoError(t, db.GetEngine(t.Context()).Where("authorization_id = ?", authorization.ID).Asc("unit_type").Find(&rules))
	require.Len(t, rules, 2)
	var codeRule *codespace_model.PermissionRepository
	for _, rule := range rules {
		switch rule.UnitType {
		case unit.TypeCode:
			codeRule = rule
			assert.Equal(t, perm.AccessModeWrite, rule.RequestedMode)
			assert.Equal(t, perm.AccessModeRead, rule.GrantedMode)
		case unit.TypeIssues:
			assert.Equal(t, perm.AccessModeRead, rule.RequestedMode)
			assert.Equal(t, perm.AccessModeNone, rule.GrantedMode)
		default:
			t.Fatalf("unexpected permission unit %d", rule.UnitType)
		}
	}

	second, err := CreateCodespace(t.Context(), opts)
	require.NoError(t, err)
	assert.Equal(t, authorization.ID, loadServiceCodespace(t, second.CodespaceUUID).PermissionAuthorizationID)

	require.NotNil(t, codeRule)
	require.NoError(t, ReducePermissionRepository(t.Context(), user.ID, authorization.ID, codeRule.TargetRepoID, codeRule.UnitType, perm.AccessModeNone))
	third, err := CreateCodespace(t.Context(), opts)
	require.NoError(t, err)
	assert.NotEqual(t, authorization.ID, loadServiceCodespace(t, third.CodespaceUUID).PermissionAuthorizationID)
}

func createConfirmedCodespace(t *testing.T, opts CreateCodespaceOptions) (*CreateCodespaceResult, error) {
	t.Helper()
	plan, err := PrepareCodespace(t.Context(), opts)
	if err != nil {
		return nil, err
	}
	opts.RequestHash = plan.RequestHash
	if opts.EnvironmentTag == "" && len(plan.Environments) > 0 {
		opts.EnvironmentTag = plan.Environments[0].Tag
	}
	opts.PermissionGrants = make(map[string]string, len(plan.Permissions))
	for _, permission := range plan.Permissions {
		opts.PermissionGrants[permission.FormName] = permission.ModeName
	}
	return CreateCodespace(t.Context(), opts)
}

func insertServiceManagerWithTags(t *testing.T, userID int64, tags ...string) *codespace_model.Manager {
	t.Helper()
	environments := make([]ManagerEnvironmentDeclaration, 0, len(tags))
	for _, tag := range tags {
		environments = append(environments, ManagerEnvironmentDeclaration{Tag: tag})
	}
	tagsJSON, err := json.Marshal(environments)
	require.NoError(t, err)
	manager := &codespace_model.Manager{
		Name:           "manager",
		UserID:         userID,
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       string(tagsJSON),
		CreatedUnix:    time.Now().Unix(),
		LastOnlineUnix: time.Now().Unix(),
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	return manager
}

func insertServiceDevContainerTemplate(t *testing.T, userID int64, name, content string) *codespace_model.DevContainerTemplate {
	t.Helper()
	template := &codespace_model.DevContainerTemplate{
		UserID:      userID,
		Name:        name,
		Content:     content,
		CreatedUnix: time.Now().Unix(),
		UpdatedUnix: time.Now().Unix(),
	}
	require.NoError(t, db.Insert(t.Context(), template))
	return template
}

func insertServiceDefaultDevContainerTemplate(t *testing.T) *codespace_model.DevContainerTemplate {
	t.Helper()
	return insertServiceDevContainerTemplate(t, 0, "Default", `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`)
}
