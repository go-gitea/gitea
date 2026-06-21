// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func enableRepoDependencies(t *testing.T, repoID int64) {
	t.Helper()

	repoUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repoID, Type: unit.TypeIssues})
	repoUnit.IssuesConfig().EnableDependencies = true
	assert.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), repoUnit))
}

func TestAPICreateIssueDependencyCrossRepoPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	targetRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	targetIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: targetRepo.ID, Index: 1})
	dependencyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.True(t, dependencyRepo.IsPrivate)
	dependencyIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: dependencyRepo.ID, Index: 1})

	enableRepoDependencies(t, targetIssue.RepoID)
	enableRepoDependencies(t, dependencyRepo.ID)

	// remove user 40 access from target repository
	_, err := db.DeleteByID[access_model.Access](t.Context(), 30)
	assert.NoError(t, err)

	url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/dependencies", "user2", "repo1", targetIssue.Index)
	dependencyMeta := &api.IssueMeta{
		Owner: "org3",
		Name:  "repo3",
		Index: dependencyIssue.Index,
	}

	user40 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 40})
	// user40 has no access to both target issue and dependency issue
	writerToken := getUserToken(t, "user40", auth_model.AccessTokenScopeWriteIssue)
	req := NewRequestWithJSON(t, "POST", url, dependencyMeta).
		AddTokenAuth(writerToken)
	MakeRequest(t, req, http.StatusNotFound)
	unittest.AssertNotExistsBean(t, &issues_model.IssueDependency{
		IssueID:      targetIssue.ID,
		DependencyID: dependencyIssue.ID,
	})

	// add user40 as a collaborator to dependency repository with read permission
	assert.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), dependencyRepo, user40, perm.AccessModeRead))

	// try again after getting read permission to dependency repository
	req = NewRequestWithJSON(t, "POST", url, dependencyMeta).
		AddTokenAuth(writerToken)
	MakeRequest(t, req, http.StatusNotFound)
	unittest.AssertNotExistsBean(t, &issues_model.IssueDependency{
		IssueID:      targetIssue.ID,
		DependencyID: dependencyIssue.ID,
	})

	// add user40 as a collaborator to target repository with write permission
	assert.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), targetRepo, user40, perm.AccessModeWrite))

	req = NewRequestWithJSON(t, "POST", url, dependencyMeta).
		AddTokenAuth(writerToken)
	MakeRequest(t, req, http.StatusCreated)
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueDependency{
		IssueID:      targetIssue.ID,
		DependencyID: dependencyIssue.ID,
	})
}

func TestAPIDeleteIssueDependencyCrossRepoPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	targetRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	targetIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: targetRepo.ID, Index: 1})
	dependencyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.True(t, dependencyRepo.IsPrivate)
	dependencyIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: dependencyRepo.ID, Index: 1})

	enableRepoDependencies(t, targetIssue.RepoID)
	enableRepoDependencies(t, dependencyRepo.ID)

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.NoError(t, issues_model.CreateIssueDependency(t.Context(), user1, targetIssue, dependencyIssue))

	// remove user 40 access from target repository
	_, err := db.DeleteByID[access_model.Access](t.Context(), 30)
	assert.NoError(t, err)

	url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/dependencies", "user2", "repo1", targetIssue.Index)
	dependencyMeta := &api.IssueMeta{
		Owner: "org3",
		Name:  "repo3",
		Index: dependencyIssue.Index,
	}

	user40 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 40})
	// user40 has no access to both target issue and dependency issue
	writerToken := getUserToken(t, "user40", auth_model.AccessTokenScopeWriteIssue)
	req := NewRequestWithJSON(t, "DELETE", url, dependencyMeta).
		AddTokenAuth(writerToken)
	MakeRequest(t, req, http.StatusNotFound)
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueDependency{
		IssueID:      targetIssue.ID,
		DependencyID: dependencyIssue.ID,
	})

	// add user40 as a collaborator to dependency repository with read permission
	assert.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), dependencyRepo, user40, perm.AccessModeRead))

	// try again after getting read permission to dependency repository
	req = NewRequestWithJSON(t, "DELETE", url, dependencyMeta).
		AddTokenAuth(writerToken)
	MakeRequest(t, req, http.StatusNotFound)
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueDependency{
		IssueID:      targetIssue.ID,
		DependencyID: dependencyIssue.ID,
	})

	// add user40 as a collaborator to target repository with write permission
	assert.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), targetRepo, user40, perm.AccessModeWrite))

	req = NewRequestWithJSON(t, "DELETE", url, dependencyMeta).
		AddTokenAuth(writerToken)
	MakeRequest(t, req, http.StatusCreated)
	unittest.AssertNotExistsBean(t, &issues_model.IssueDependency{
		IssueID:      targetIssue.ID,
		DependencyID: dependencyIssue.ID,
	})
}
