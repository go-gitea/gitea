// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsJobTokenPermissiveAccess(t *testing.T) {
	cases := []struct {
		name   string
		isFork bool

		ownerPermMode repo_model.ActionsTokenPermissionMode
		ownerMaxPerms map[unit_model.Type]perm.AccessMode

		repoPermMode repo_model.ActionsTokenPermissionMode
		repoMaxPerms map[unit_model.Type]perm.AccessMode

		expectGitAccess perm.AccessMode
	}{
		{
			name:            "OwnerConfig-Permissive",
			ownerPermMode:   repo_model.ActionsTokenPermissionModePermissive,
			expectGitAccess: perm.AccessModeWrite,
		},
		{
			name:            "OwnerConfig-Permissive-CodeNone",
			ownerPermMode:   repo_model.ActionsTokenPermissionModePermissive,
			ownerMaxPerms:   map[unit_model.Type]perm.AccessMode{unit_model.TypeCode: perm.AccessModeNone},
			expectGitAccess: perm.AccessModeNone,
		},
		{
			name:            "OwnerConfig-Restricted",
			ownerPermMode:   repo_model.ActionsTokenPermissionModeRestricted,
			expectGitAccess: perm.AccessModeRead,
		},

		// repo uses its own settings, so owner settings should not affect it
		{
			name:            "SameRepo-Permissive",
			ownerPermMode:   repo_model.ActionsTokenPermissionModeRestricted,
			ownerMaxPerms:   map[unit_model.Type]perm.AccessMode{unit_model.TypeCode: perm.AccessModeNone},
			repoPermMode:    repo_model.ActionsTokenPermissionModePermissive,
			expectGitAccess: perm.AccessModeWrite,
		},
		{
			name:            "SameRepo-Permissive-CodeNone",
			ownerPermMode:   repo_model.ActionsTokenPermissionModePermissive,
			ownerMaxPerms:   map[unit_model.Type]perm.AccessMode{unit_model.TypeCode: perm.AccessModeRead},
			repoPermMode:    repo_model.ActionsTokenPermissionModePermissive,
			repoMaxPerms:    map[unit_model.Type]perm.AccessMode{unit_model.TypeCode: perm.AccessModeNone},
			expectGitAccess: perm.AccessModeNone,
		},
		{
			name:            "SameRepo-Restricted",
			repoPermMode:    repo_model.ActionsTokenPermissionModeRestricted,
			expectGitAccess: perm.AccessModeRead,
		},

		// forks should be always restricted to max read access for code
		{
			name:            "Fork-Permissive",
			repoPermMode:    repo_model.ActionsTokenPermissionModePermissive,
			isFork:          true,
			expectGitAccess: perm.AccessModeRead,
		},
		{
			name:            "Fork-Restricted",
			repoPermMode:    repo_model.ActionsTokenPermissionModeRestricted,
			isFork:          true,
			expectGitAccess: perm.AccessModeRead,
		},
		{
			name:            "Fork-Restricted-CodeNone",
			repoPermMode:    repo_model.ActionsTokenPermissionModeRestricted,
			repoMaxPerms:    map[unit_model.Type]perm.AccessMode{unit_model.TypeCode: perm.AccessModeNone},
			isFork:          true,
			expectGitAccess: perm.AccessModeNone,
		},
	}

	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: task.RepoID})
		repoActionsUnit := repo.MustGetUnit(t.Context(), unit_model.TypeActions)
		repoActionsCfg := repoActionsUnit.ActionsConfig()
		ownerActionsCfg, err := actions_model.GetOwnerActionsConfig(t.Context(), repo.OwnerID)
		require.NoError(t, err)

		_, err = db.GetEngine(t.Context()).ID(task.RepoID).Cols("is_private").Update(&repo_model.Repository{IsPrivate: true})
		require.NoError(t, err)

		assertRespCodeForSuccess := func(t *testing.T, resp *httptest.ResponseRecorder, succeed bool) {
			if succeed {
				assert.True(t, 200 <= resp.Code && resp.Code < 300, "Expected success status code, got %d", resp.Code)
			} else {
				assert.True(t, 400 <= resp.Code && resp.Code < 500, "Expected client error status code, got %d", resp.Code)
			}
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				// prepare owner's token permissions settings
				ownerActionsCfg.TokenPermissionMode = tt.ownerPermMode
				ownerActionsCfg.MaxTokenPermissions = util.Iif(tt.ownerMaxPerms == nil, nil, &repo_model.ActionsTokenPermissions{UnitAccessModes: tt.ownerMaxPerms})
				require.NoError(t, actions_model.SetOwnerActionsConfig(t.Context(), repo.OwnerID, ownerActionsCfg))

				// prepare repo's token permissions settings
				repoActionsCfg.OverrideOwnerConfig = tt.repoPermMode != "" || tt.repoMaxPerms != nil
				repoActionsCfg.TokenPermissionMode = tt.repoPermMode
				repoActionsCfg.MaxTokenPermissions = util.Iif(tt.repoMaxPerms == nil, nil, &repo_model.ActionsTokenPermissions{UnitAccessModes: tt.repoMaxPerms})
				require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), repoActionsUnit))

				// prepare task and its token
				task.GenerateAndFillToken()
				task.Status = actions_model.StatusRunning
				task.IsForkPullRequest = tt.isFork
				err := actions_model.UpdateTask(t.Context(), task, "token_hash", "token_salt", "token_last_eight", "status", "is_fork_pull_request")
				require.NoError(t, err)

				require.NoError(t, task.LoadJob(t.Context()))
				require.NoError(t, task.Job.LoadRun(t.Context()))
				task.Job.Run.IsForkPullRequest = tt.isFork
				require.NoError(t, actions_model.UpdateRun(t.Context(), task.Job.Run, "is_fork_pull_request"))

				testURL := *u
				testURL.User = url.UserPassword("gitea-actions", task.Token)

				t.Run("ReadGitContent", func(t *testing.T) {
					testURL.Path = "/user5/repo4.git/HEAD"
					resp := MakeRequest(t, NewRequest(t, "GET", testURL.String()), NoExpectedStatus)
					assertRespCodeForSuccess(t, resp, tt.expectGitAccess != perm.AccessModeNone)

					testURL.Path = "/user5/repo4.git/info/lfs/locks"
					req := NewRequest(t, "GET", testURL.String()).SetHeader("Accept", lfs.MediaType)
					resp = MakeRequest(t, req, NoExpectedStatus)
					assertRespCodeForSuccess(t, resp, tt.expectGitAccess != perm.AccessModeNone)
				})

				t.Run("WriteGitContent", func(t *testing.T) {
					req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/contents/test-filename", repo.FullName()), &structs.CreateFileOptions{
						FileOptions:   structs.FileOptions{NewBranchName: "new-branch" + t.Name()},
						ContentBase64: base64.StdEncoding.EncodeToString([]byte(`dummy content`)),
					}).AddTokenAuth(task.Token)
					resp := MakeRequest(t, req, NoExpectedStatus)
					assertRespCodeForSuccess(t, resp, tt.expectGitAccess == perm.AccessModeWrite)

					testURL.Path = "/user5/repo4.git/info/lfs/objects/batch"
					req = NewRequestWithJSON(t, "POST", testURL.String(), lfs.BatchRequest{Operation: "upload"}).SetHeader("Accept", lfs.MediaType)
					resp = MakeRequest(t, req, NoExpectedStatus)
					assertRespCodeForSuccess(t, resp, tt.expectGitAccess == perm.AccessModeWrite)
				})

				t.Run("NoOtherPermissions", func(t *testing.T) {
					req := NewRequest(t, "DELETE", "/api/v1/repos/"+repo.FullName()).AddTokenAuth(task.Token)
					resp := MakeRequest(t, req, NoExpectedStatus)
					assertRespCodeForSuccess(t, resp, false)
				})
			})
		}
	})
}

func TestActionsCrossRepoAccess(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)

		// 1. Create Organization
		orgName := "org-cross-test"
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &structs.CreateOrgOption{
			UserName: orgName,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		owner, err := org_model.GetOrgByName(t.Context(), orgName)
		require.NoError(t, err)

		// 2. Create Two Repositories in owner
		createRepoInOrg := func(name string) int64 {
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &structs.CreateRepoOption{
				Name:     name,
				AutoInit: true,
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusCreated)
			var repo structs.Repository
			DecodeJSON(t, resp, &repo)
			return repo.ID
		}

		repoAID := createRepoInOrg("repo-A")
		repoBID := createRepoInOrg("repo-B")

		// 3. Enable Actions in Repo A (Source) and Repo B (Target)
		enableActions := func(repoID int64) {
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repoID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{
					TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
				},
			})
			require.NoError(t, err)
		}

		enableActions(repoAID)
		enableActions(repoBID)

		// 4. Create Task in Repo A, and use A's token to access B
		taskA := createActionTask(t, repoAID, false)
		testCtxA := APITestContext{
			Session:  emptyTestSession(t),
			Token:    taskA.Token,
			Username: orgName,
			Reponame: "repo-B",
		}

		testCtxA.ExpectedCode = http.StatusOK
		t.Run("PublicCrossRepoAccess", doAPIGetRepository(testCtxA, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, "repo-B", r.Name)
		}))

		// make repo-B be private
		req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/org-cross-test/repo-B", &structs.EditRepoOption{Private: new(true)}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		testCtxA.ExpectedCode = http.StatusNotFound
		t.Run("NoPrivateCrossRepoAccess", doAPIGetRepository(testCtxA, nil))

		ownerActionsCfg := actions_model.OwnerActionsConfig{AllowedCrossRepoIDs: []int64{repoBID}}
		require.NoError(t, actions_model.SetOwnerActionsConfig(t.Context(), owner.ID, ownerActionsCfg))

		testCtxA.ExpectedCode = http.StatusOK
		t.Run("AccessToSelectedPrivateRepo", doAPIGetRepository(testCtxA, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, "repo-B", r.Name)
		}))

		t.Run("RepoTransfer", func(t *testing.T) {
			ownerActionsCfg, err := actions_model.GetOwnerActionsConfig(t.Context(), owner.ID)
			require.NoError(t, err)
			assert.Contains(t, ownerActionsCfg.AllowedCrossRepoIDs, repoBID)

			// Transfer Repository to user4
			req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/repo-B/transfer", orgName), &structs.TransferRepoOption{
				NewOwner: "user4",
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)

			// Accept transfer as user4
			session4 := loginUser(t, "user4")
			token4 := getTokenForLoggedInUser(t, session4, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
			req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/repo-B/transfer/accept", orgName)).AddTokenAuth(token4)
			MakeRequest(t, req, http.StatusAccepted)

			// Verify it is removed from the org's config
			ownerActionsCfg, err = actions_model.GetOwnerActionsConfig(t.Context(), owner.ID)
			require.NoError(t, err)
			assert.NotContains(t, ownerActionsCfg.AllowedCrossRepoIDs, repoBID)
		})
	})
}

func TestActionsJobTokenPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("WriteIssue", TestActionsJobTokenPermissionsWriteIssue)
}

func TestActionsJobTokenPermissionsWriteIssue(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
	require.Equal(t, repo.ID, task.RepoID)

	require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{
		RepoID: repo.ID,
		Type:   unit_model.TypeActions,
		Config: &repo_model.ActionsConfig{},
	}))

	repoActionsUnit := repo.MustGetUnit(t.Context(), unit_model.TypeActions)
	repoActionsCfg := repoActionsUnit.ActionsConfig()
	repoActionsCfg.OverrideOwnerConfig = true
	repoActionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionModePermissive
	repoActionsCfg.MaxTokenPermissions = nil
	require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), repoActionsUnit))

	task.GenerateAndFillToken()
	task.Status = actions_model.StatusRunning
	require.NoError(t, actions_model.UpdateTask(t.Context(), task, "token_hash", "token_salt", "token_last_eight", "status"))

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)

	labelURL := fmt.Sprintf("/api/v1/repos/%s/%s/labels", user.Name, repo.Name)
	req := NewRequestWithJSON(t, "POST", labelURL, &structs.CreateLabelOption{
		Name:  "task-label",
		Color: "0e8a16",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	label := DecodeJSON(t, resp, &structs.Label{})

	issueURL := fmt.Sprintf("/api/v1/repos/%s/%s/issues", user.Name, repo.Name)
	req = NewRequestWithJSON(t, "POST", issueURL, &structs.CreateIssueOption{
		Title: "issue for actions token label deletion",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	issue := DecodeJSON(t, resp, &structs.Issue{})

	taskToken := task.Token
	require.NotEmpty(t, taskToken)

	issueLabelsURL := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels", user.Name, repo.Name, issue.Index)
	req = NewRequestWithJSON(t, "POST", issueLabelsURL, &structs.IssueLabelsOption{
		Labels: []any{label.ID},
	}).AddTokenAuth(taskToken)
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", issueLabelsURL, label.ID)).AddTokenAuth(taskToken)
	MakeRequest(t, req, http.StatusNoContent)
}

func createActionTask(t *testing.T, repoID int64, isFork bool) *actions_model.ActionTask {
	job := &actions_model.ActionRunJob{
		RepoID:            repoID,
		Status:            actions_model.StatusRunning,
		IsForkPullRequest: isFork,
		JobID:             "test_job",
		Name:              "test_job",
	}
	require.NoError(t, db.Insert(t.Context(), job))
	task := &actions_model.ActionTask{
		JobID:             job.ID,
		RepoID:            repoID,
		Status:            actions_model.StatusRunning,
		IsForkPullRequest: isFork,
	}
	task.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), task))
	return task
}

func TestActionsTokenPermissionsPersistenceWithWorkflow(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// create repos
		repo1 := createActionsTestRepo(t, token, "actions-permission-repo1", false)
		repo2 := createActionsTestRepo(t, token, "actions-permission-repo2", true)

		// add repo2 to owner-level cross-repo access list
		req := NewRequestWithValues(t, "POST", "/user/settings/actions/general", map[string]string{
			"cross_repo_add_target":      "true",
			"cross_repo_add_target_name": repo2.Name,
		})
		session.MakeRequest(t, req, http.StatusOK)

		// create the runner for repo1
		runner1 := newMockRunner()
		runner1.registerAsRepoRunner(t, user2.Name, repo1.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// set repo1 actions token permission mode to "permissive"
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo1.Name), map[string]string{
			"token_permission_mode": "permissive",
			"override_owner_config": "true",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// set repo2 actions token permission mode to "restricted", and set max permissions
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo2.Name), map[string]string{
			"token_permission_mode":  "restricted",
			"override_owner_config":  "true",
			"enable_max_permissions": "true",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeReleases)): "read",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// create a workflow file with "permission" keyword for repo1
		wfTreePath := ".gitea/workflows/test_permissions.yml"
		wfFileContent := `name: Test Permissions
on:
  push:
    paths:
      - '.gitea/workflows/test_permissions.yml'

jobs:
  job-override:
    runs-on: ubuntu-latest
    permissions:
      code: write
    steps:
      - run: echo "test perms"
`
		opts := getWorkflowCreateFileOptions(user2, repo1.DefaultBranch, "create "+wfTreePath, wfFileContent)
		createWorkflowFile(t, token, user2.Name, repo1.Name, wfTreePath, opts)

		task1 := runner1.fetchTask(t)
		task1Token := task1.Secrets["GITEA_TOKEN"]
		require.NotEmpty(t, task1Token)

		// should fail: target repo does not allow code access
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo2.Name)).AddTokenAuth(task1Token)
		MakeRequest(t, req, http.StatusNotFound)

		// set repo2 max permission to "read" so that the actions token can access code
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo2.Name), map[string]string{
			"token_permission_mode":  "restricted",
			"override_owner_config":  "true",
			"enable_max_permissions": "true",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeCode)):     "read",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeReleases)): "read",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// should succeed: target repo now allows code read access for this token
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo2.Name)).AddTokenAuth(task1Token)
		MakeRequest(t, req, http.StatusOK)
		// but it should not have write access
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s/%s.git/info/lfs/objects/batch", user2.Name, repo2.Name), lfs.BatchRequest{Operation: "upload"}).
			SetHeader("Accept", lfs.MediaType).
			AddBasicAuth("gitea-actions", task1Token)
		MakeRequest(t, req, http.StatusUnauthorized)

		// set repo1&repo2 max permission to "write" so that the actions token can access code
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo1.Name), map[string]string{
			"token_permission_mode":  "restricted",
			"override_owner_config":  "true",
			"enable_max_permissions": "true",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeCode)): "write",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo2.Name), map[string]string{
			"token_permission_mode":  "restricted",
			"override_owner_config":  "true",
			"enable_max_permissions": "true",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeCode)): "write",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// now task1 has write access to repo1, but still only read access to repo2 (different repo)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s/%s.git/info/lfs/objects/batch", user2.Name, repo1.Name), lfs.BatchRequest{Operation: "upload"}).
			SetHeader("Accept", lfs.MediaType).
			AddBasicAuth("gitea-actions", task1Token)
		MakeRequest(t, req, http.StatusOK)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s/%s.git/info/lfs/objects/batch", user2.Name, repo2.Name), lfs.BatchRequest{Operation: "upload"}).
			SetHeader("Accept", lfs.MediaType).
			AddBasicAuth("gitea-actions", task1Token)
		MakeRequest(t, req, http.StatusUnauthorized)
	})
}
