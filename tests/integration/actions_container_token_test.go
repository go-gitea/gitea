// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	packages_service "gitea.dev/services/packages"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

func TestActionsContainerToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()
	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: task.RepoID})
	require.NoError(t, repo.LoadOwner(ctx))
	if !repo.UnitEnabled(ctx, unit.TypePackages) {
		require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypePackages, Config: &repo_model.UnitConfig{}}))
	}
	task.GenerateAndFillToken()
	task.Status = actions_model.StatusRunning
	require.NoError(t, actions_model.UpdateTask(ctx, task, "token_hash", "token_salt", "token_last_eight", "status"))
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
	permissions := repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	permissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeWrite
	job.TokenPermissions = &permissions
	_, err := db.GetEngine(ctx).ID(job.ID).Cols("token_permissions").Update(job)
	require.NoError(t, err)
	req := NewRequest(t, http.MethodGet, "/v2/token")
	req.SetBasicAuth("gitea-actions", task.Token)
	response := MakeRequest(t, req, http.StatusOK)
	var auth struct {
		Token string `json:"token"`
	}
	DecodeJSON(t, response, &auth)
	require.NotEmpty(t, auth.Token)
	bearer := "Bearer " + auth.Token
	base := "/v2/" + repo.Owner.LowerName + "/at-auth"
	request := func(method, path string) *RequestWrapper {
		return NewRequest(t, method, path).AddTokenAuth(bearer)
	}
	response = MakeRequest(t, request(http.MethodPost, base+"/blobs/uploads"), http.StatusAccepted)
	location := response.Header().Get("Location")
	require.Contains(t, location, ".")
	MakeRequest(t, request(http.MethodGet, location), http.StatusNoContent)
	otherTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 48})
	otherJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: otherTask.JobID})
	otherJob.TokenPermissions = &permissions
	_, err = db.GetEngine(ctx).ID(otherJob.ID).Cols("token_permissions").Update(otherJob)
	require.NoError(t, err)
	otherBearer, err := packages_service.CreateAuthorizationToken(user_model.NewActionsUserWithTaskID(otherTask.ID), auth_model.AccessTokenScopeWritePackage)
	require.NoError(t, err)
	MakeRequest(t, NewRequest(t, http.MethodGet, location).AddTokenAuth("Bearer "+otherBearer), http.StatusUnauthorized)
	MakeRequest(t, request(http.MethodGet, strings.Split(location, ".")[0]), http.StatusUnauthorized)
	MakeRequest(t, request(http.MethodGet, strings.Replace(location, "/at-auth/", "/other/", 1)), http.StatusUnauthorized)
	MakeRequest(t, request(http.MethodPost, "/v2/"+repo.Owner.LowerName+"/other/blobs/uploads"), http.StatusAccepted)
	MakeRequest(t, request(http.MethodPost, "/v2/user2/at-auth/blobs/uploads"), http.StatusUnauthorized)
	generic := NewRequestWithBody(t, http.MethodPut, "/api/packages/"+repo.Owner.LowerName+"/generic/at-auth/1/test", strings.NewReader("test"))
	generic.SetBasicAuth("gitea-actions", task.Token)
	MakeRequest(t, generic, http.StatusCreated)
	created := unittest.AssertExistsAndLoadBean(t, &packages_model.Package{OwnerID: repo.OwnerID, Type: packages_model.TypeGeneric, Name: "at-auth"})
	require.Equal(t, repo.ID, created.RepoID)
	permissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeRead
	_, err = db.GetEngine(ctx).ID(job.ID).Cols("token_permissions").Update(job)
	require.NoError(t, err)
	MakeRequest(t, request(http.MethodGet, base+"/tags/list"), http.StatusNotFound)
	MakeRequest(t, request(http.MethodPost, base+"/blobs/uploads"), http.StatusUnauthorized)
	permissions.UnitAccessModes[unit.TypePackages] = perm.AccessModeWrite
	_, err = db.GetEngine(ctx).ID(job.ID).Cols("token_permissions").Update(job)
	require.NoError(t, err)
	task.IsForkPullRequest = true
	require.NoError(t, actions_model.UpdateTask(ctx, task, "is_fork_pull_request"))
	MakeRequest(t, request(http.MethodPost, base+"/blobs/uploads"), http.StatusUnauthorized)
	task.IsForkPullRequest = false
	require.NoError(t, actions_model.UpdateTask(ctx, task, "is_fork_pull_request"))
	pkg := &packages_model.Package{OwnerID: repo.OwnerID, RepoID: repo.ID + 1, Type: packages_model.TypeContainer, Name: "at-auth", LowerName: "at-auth"}
	require.NoError(t, db.Insert(ctx, pkg))
	MakeRequest(t, request(http.MethodPost, base+"/blobs/uploads"), http.StatusUnauthorized)
	require.NoError(t, packages_model.DeletePackageByID(ctx, pkg.ID))
	task.Status = actions_model.StatusSuccess
	_, err = db.GetEngine(ctx).ID(task.ID).Cols("status").Update(task)
	require.NoError(t, err)
	MakeRequest(t, request(http.MethodGet, location), http.StatusUnauthorized)
	req = NewRequest(t, http.MethodGet, "/v2/token")
	req.SetBasicAuth("gitea-actions", task.Token)
	MakeRequest(t, req, http.StatusUnauthorized)
}
