// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
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
	"gitea.dev/modules/test"
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

func TestAPIIssueDependencyIncludes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Index: 1})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Index: 2})
	issue3 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Index: 3})

	enableRepoDependencies(t, repo.ID)

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.NoError(t, issues_model.CreateIssueDependency(t.Context(), user1, issue1, issue2))
	assert.NoError(t, issues_model.CreateIssueDependency(t.Context(), user1, issue1, issue3))

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeReadIssue)

	t.Run("GetIssueWithIncludes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?includes=dependencies", owner.Name, repo.Name, issue1.Index)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiIssue api.Issue
		DecodeJSON(t, resp, &apiIssue)

		ref2 := &api.IssueMeta{Owner: owner.Name, Name: repo.Name, Index: issue2.Index}
		ref3 := &api.IssueMeta{Owner: owner.Name, Name: repo.Name, Index: issue3.Index}
		assert.ElementsMatch(t, []*api.IssueMeta{ref2, ref3}, apiIssue.BlockedBy)
		assert.Empty(t, apiIssue.Blocking)
	})

	t.Run("GetIssueBlockingDirection", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?includes=dependencies", owner.Name, repo.Name, issue2.Index)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiIssue api.Issue
		DecodeJSON(t, resp, &apiIssue)

		ref1 := &api.IssueMeta{Owner: owner.Name, Name: repo.Name, Index: issue1.Index}
		assert.Empty(t, apiIssue.BlockedBy)
		assert.ElementsMatch(t, []*api.IssueMeta{ref1}, apiIssue.Blocking)
	})

	t.Run("GetIssueWithoutIncludes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", owner.Name, repo.Name, issue1.Index)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var raw map[string]any
		DecodeJSON(t, resp, &raw)

		assert.NotContains(t, raw, "blocked_by", "blocked_by must be omitted without includes=dependencies")
		assert.NotContains(t, raw, "blocking", "blocking must be omitted without includes=dependencies")
	})

	t.Run("ListIssuesWithIncludes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := fmt.Sprintf("/api/v1/repos/%s/%s/issues?includes=dependencies&state=open", owner.Name, repo.Name)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var issues []*api.Issue
		DecodeJSON(t, resp, &issues)

		found := false
		for _, iss := range issues {
			if iss.Index == issue1.Index {
				ref2 := &api.IssueMeta{Owner: owner.Name, Name: repo.Name, Index: issue2.Index}
				ref3 := &api.IssueMeta{Owner: owner.Name, Name: repo.Name, Index: issue3.Index}
				assert.ElementsMatch(t, []*api.IssueMeta{ref2, ref3}, iss.BlockedBy)
				found = true
				break
			}
		}
		assert.True(t, found, "issue1 should be in list results")
	})

	t.Run("InvalidIncludes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?includes=invalid", owner.Name, repo.Name, issue1.Index)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("EmptyDepsReturnEmptyArrays", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		issue4 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Index: 4})
		url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?includes=dependencies", owner.Name, repo.Name, issue4.Index)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var raw map[string]any
		DecodeJSON(t, resp, &raw)

		assert.Equal(t, []any{}, raw["blocked_by"], "blocked_by must be an empty array, not null or missing")
		assert.Equal(t, []any{}, raw["blocking"], "blocking must be an empty array, not null or missing")
	})

	t.Run("CrossRepoDepsFilteredByPermission", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		publicRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		assert.False(t, publicRepo.IsPrivate)
		assert.True(t, privateRepo.IsPrivate)

		publicIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 1})
		privateIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: privateRepo.ID, Index: 1})

		enableRepoDependencies(t, publicRepo.ID)
		enableRepoDependencies(t, privateRepo.ID)

		assert.NoError(t, issues_model.CreateIssueDependency(t.Context(), user1, publicIssue, privateIssue))

		outsideUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		outsideToken := getUserToken(t, outsideUser.Name, auth_model.AccessTokenScopeReadIssue)

		publicOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: publicRepo.OwnerID})
		url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?includes=dependencies", publicOwner.Name, publicRepo.Name, publicIssue.Index)
		req := NewRequest(t, "GET", url).AddTokenAuth(outsideToken)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiIssue api.Issue
		DecodeJSON(t, resp, &apiIssue)

		for _, dep := range apiIssue.BlockedBy {
			assert.NotEqual(t, privateRepo.Name, dep.Name,
				"private repo dependency should be filtered out, but found %s/%s#%d", dep.Owner, dep.Name, dep.Index)
		}
	})
}

func TestWebDeleteIssueDependencyCrossRepoPermission(t *testing.T) {
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

	user40 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 40})
	assert.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), targetRepo, user40, perm.AccessModeWrite))

	session := loginUser(t, user40.Name)
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/user2/repo1/issues/%d/dependency/delete", targetIssue.Index), map[string]string{
		"removeDependencyID": strconv.FormatInt(dependencyIssue.ID, 10),
		"dependencyType":     "blockedBy",
	})
	resp := session.MakeRequest(t, req, http.StatusBadRequest)
	assert.Equal(t, "Permission denied.", test.ParseJSONError(resp.Body.Bytes()).ErrorMessage)
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueDependency{IssueID: targetIssue.ID, DependencyID: dependencyIssue.ID})

	assert.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), dependencyRepo, user40, perm.AccessModeRead))

	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/user2/repo1/issues/%d/dependency/delete", targetIssue.Index), map[string]string{
		"removeDependencyID": strconv.FormatInt(dependencyIssue.ID, 10),
		"dependencyType":     "blockedBy",
	})
	session.MakeRequest(t, req, http.StatusOK)
	unittest.AssertNotExistsBean(t, &issues_model.IssueDependency{IssueID: targetIssue.ID, DependencyID: dependencyIssue.ID})
}

func TestAPIGetIssueBlocks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	publicRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.False(t, publicRepo.IsPrivate)
	assert.True(t, privateRepo.IsPrivate)

	// repo1#1 blocks repo1#2 and repo2#1
	blocker := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 1})
	publicBlocked := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 2})
	privateBlocked := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: privateRepo.ID, Index: 1})

	enableRepoDependencies(t, publicRepo.ID)
	enableRepoDependencies(t, privateRepo.ID)

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.NoError(t, issues_model.CreateIssueDependency(t.Context(), user1, publicBlocked, blocker))
	assert.NoError(t, issues_model.CreateIssueDependency(t.Context(), user1, privateBlocked, blocker))

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: publicRepo.OwnerID})
	url := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/blocks", owner.Name, publicRepo.Name, blocker.Index)

	t.Run("OwnerSeesBoth", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeReadIssue)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var blocked []*api.Issue
		DecodeJSON(t, resp, &blocked)

		got := make([]string, 0, len(blocked))
		for _, iss := range blocked {
			got = append(got, fmt.Sprintf("%s#%d", iss.Repo.FullName, iss.Index))
		}
		assert.ElementsMatch(t, []string{
			fmt.Sprintf("%s#%d", publicRepo.FullName(), publicBlocked.Index),
			fmt.Sprintf("%s#%d", privateRepo.FullName(), privateBlocked.Index),
		}, got)
	})

	t.Run("OutsiderSeesOnlyPublic", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		outsider := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		token := getUserToken(t, outsider.Name, auth_model.AccessTokenScopeReadIssue)
		req := NewRequest(t, "GET", url).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var blocked []*api.Issue
		DecodeJSON(t, resp, &blocked)

		assert.Len(t, blocked, 1)
		assert.Equal(t, publicRepo.FullName(), blocked[0].Repo.FullName)
		assert.Equal(t, publicBlocked.Index, blocked[0].Index)
	})
}
