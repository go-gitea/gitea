// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoReparent(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Use user1 (admin)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session := loginUser(t, doer.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	sourceRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	targetRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})

	// 1. Start reparenting
	// Since user1 is admin, it might try to accept immediately.
	// But repo 10 owner is user12. Repo 11 owner is user13.
	// user1 (admin) CAN accept for anyone.
	// So it should be immediate.
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/reparent", sourceRepo.OwnerName, sourceRepo.Name), &api.ReparentRepoOption{
		NewOwner: targetRepo.OwnerName,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK) // Immediate success because user1 is admin

	// Verify database state
	sourceRepo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	assert.True(t, sourceRepo.IsFork)
	assert.Equal(t, targetRepo.ID, sourceRepo.ForkID)
	assert.Equal(t, 0, sourceRepo.NumForks)

	targetRepo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	assert.False(t, targetRepo.IsFork)
	assert.Equal(t, int64(0), targetRepo.ForkID)
	assert.Equal(t, 1, targetRepo.NumForks)

	// Test with a non-admin to see 202 Accepted
	// Reset state first? No, let's use another pair if possible or just trust unit tests for the flow.
	// Actually, let's use user13 (owner of fork) to request reparenting of user12's repo.

	// Prepare another fork
	// Repo 1 is user2/repo1. user4 forks it.
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session4 := loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session4, auth_model.AccessTokenScopeWriteRepository)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	// user4 forks repo1
	reqFork := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", repo1.OwnerName, repo1.Name), &api.CreateForkOption{})
	reqFork.AddTokenAuth(token4)
	respFork := MakeRequest(t, reqFork, http.StatusAccepted)
	var forkRes api.Repository
	DecodeJSON(t, respFork, &forkRes)

	// Now user2 (owner) requests reparenting
	session2 := loginUser(t, "user2")
	token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteRepository)
	reqReparent := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/reparent", repo1.OwnerName, repo1.Name), &api.ReparentRepoOption{
		NewOwner: user4.Name,
	}).AddTokenAuth(token2)
	MakeRequest(t, reqReparent, http.StatusAccepted)

	// Verify status
	repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, repo_model.RepositoryPendingReparent, repo1.Status)

	// user4 (target owner) accepts
	reqAccept := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/reparent/accept", repo1.OwnerName, repo1.Name)).AddTokenAuth(token4)
	MakeRequest(t, reqAccept, http.StatusAccepted)

	// Verify final state
	repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.True(t, repo1.IsFork)
	assert.Equal(t, forkRes.ID, repo1.ForkID)
}

func TestAPIRepoReparentNoFork(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 (owner of repo1, which is public)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session2 := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteRepository)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	// user5 will be the new owner, but he hasn't forked repo1 yet
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	// user2 (owner) requests reparenting to user5
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/reparent", user2.Name, repo1.Name), &api.ReparentRepoOption{
		NewOwner: user5.Name,
	}).AddTokenAuth(token2)
	MakeRequest(t, req, http.StatusAccepted)

	// user5 (the target owner) accepts. This should trigger fork creation and swap.
	session5 := loginUser(t, user5.Name)
	token5 := getTokenForLoggedInUser(t, session5, auth_model.AccessTokenScopeWriteRepository)
	reqAccept := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/reparent/accept", user2.Name, repo1.Name)).AddTokenAuth(token5)
	MakeRequest(t, reqAccept, http.StatusAccepted)

	// Verify repo1 is now a fork
	repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.True(t, repo1.IsFork)
	assert.NotEqual(t, int64(0), repo1.ForkID)

	// Verify the new parent (created from fork) is NOT a fork
	newParent := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo1.ForkID})
	assert.False(t, newParent.IsFork)
	assert.Equal(t, user5.ID, newParent.OwnerID)
}

func TestAPIRepoReparentPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user30/repo30 (ID 30 in repository.yml? No, let's check repo ID for user30)
	// Actually, let's just use user2/repo1 (ID 1) again, it's safe because of PrepareTestEnv.
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// user10 (unrelated)
	user10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	session10 := loginUser(t, user10.Name)
	token10 := getTokenForLoggedInUser(t, session10, auth_model.AccessTokenScopeWriteRepository)

	// 1. user10 (unrelated) tries to initiate reparent of user2/repo1 to user11
	user11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/reparent", user2.Name, repo1.Name), &api.ReparentRepoOption{
		NewOwner: user11.Name,
	}).AddTokenAuth(token10)
	MakeRequest(t, req, http.StatusForbidden)

	// 2. user5 (target owner) initiates reparent of user2/repo1 to himself.
	// Should be 403 Forbidden now (only Source Owner or Admin can initiate).
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	session5 := loginUser(t, user5.Name)
	token5 := getTokenForLoggedInUser(t, session5, auth_model.AccessTokenScopeWriteRepository)

	reqAuto := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/reparent", user2.Name, repo1.Name), &api.ReparentRepoOption{
		NewOwner: user5.Name,
	}).AddTokenAuth(token5)
	MakeRequest(t, reqAuto, http.StatusForbidden)
}
