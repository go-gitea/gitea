// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/gitdiff"
	issue_service "code.gitea.io/gitea/services/issue"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIViewPulls(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls?state=all", owner.Name, repo.Name).
		AddTokenAuth(ctx.Token)
	resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

	var pulls []*api.PullRequest
	DecodeJSON(t, resp, &pulls)
	expectedLen := unittest.GetCount(t, &issues_model.Issue{RepoID: repo.ID}, unittest.Cond("is_pull = ?", true))
	assert.Len(t, pulls, expectedLen)

	assert.Len(t, pulls, 3)
	pull := pulls[0]
	assert.EqualValues(t, 1, pull.Poster.ID)
	assert.Len(t, pull.RequestedReviewers, 2)
	assert.Empty(t, pull.RequestedReviewersTeams)
	assert.EqualValues(t, 5, pull.RequestedReviewers[0].ID)
	assert.EqualValues(t, 6, pull.RequestedReviewers[1].ID)

	if assert.EqualValues(t, 5, pull.ID) {
		resp = ctx.Session.MakeRequest(t, NewRequest(t, "GET", pull.DiffURL), http.StatusOK)
		bs, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		patch, err := gitdiff.ParsePatch(t.Context(), 1000, 5000, 10, bytes.NewReader(bs), "")
		assert.NoError(t, err)
		if assert.Len(t, patch.Files, 1) {
			assert.Equal(t, "File-WoW", patch.Files[0].Name)
			// FIXME: The old name should be empty if it's a file add type
			assert.Equal(t, "File-WoW", patch.Files[0].OldName)
			assert.EqualValues(t, 1, patch.Files[0].Addition)
			assert.EqualValues(t, 0, patch.Files[0].Deletion)
			assert.Equal(t, gitdiff.DiffFileAdd, patch.Files[0].Type)
		}

		t.Run(fmt.Sprintf("APIGetPullFiles_%d", pull.ID),
			doAPIGetPullFiles(ctx, pull, func(t *testing.T, files []*api.ChangedFile) {
				if assert.Len(t, files, 1) {
					assert.Equal(t, "File-WoW", files[0].Filename)
					assert.Empty(t, files[0].PreviousFilename)
					assert.EqualValues(t, 1, files[0].Additions)
					assert.EqualValues(t, 1, files[0].Changes)
					assert.EqualValues(t, 0, files[0].Deletions)
					assert.Equal(t, "added", files[0].Status)
				}
			}))
	}

	pull = pulls[1]
	assert.EqualValues(t, 1, pull.Poster.ID)
	assert.Len(t, pull.RequestedReviewers, 4)
	assert.Empty(t, pull.RequestedReviewersTeams)
	assert.EqualValues(t, 3, pull.RequestedReviewers[0].ID)
	assert.EqualValues(t, 4, pull.RequestedReviewers[1].ID)
	assert.EqualValues(t, 2, pull.RequestedReviewers[2].ID)
	assert.EqualValues(t, 5, pull.RequestedReviewers[3].ID)

	if assert.EqualValues(t, 2, pull.ID) {
		resp = ctx.Session.MakeRequest(t, NewRequest(t, "GET", pull.DiffURL), http.StatusOK)
		bs, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		patch, err := gitdiff.ParsePatch(t.Context(), 1000, 5000, 10, bytes.NewReader(bs), "")
		assert.NoError(t, err)
		if assert.Len(t, patch.Files, 1) {
			assert.Equal(t, "README.md", patch.Files[0].Name)
			assert.Equal(t, "README.md", patch.Files[0].OldName)
			assert.EqualValues(t, 4, patch.Files[0].Addition)
			assert.EqualValues(t, 1, patch.Files[0].Deletion)
			assert.Equal(t, gitdiff.DiffFileChange, patch.Files[0].Type)
		}

		t.Run(fmt.Sprintf("APIGetPullFiles_%d", pull.ID),
			doAPIGetPullFiles(ctx, pull, func(t *testing.T, files []*api.ChangedFile) {
				if assert.Len(t, files, 1) {
					assert.Equal(t, "README.md", files[0].Filename)
					// FIXME: The PreviousFilename name should be the same as Filename if it's a file change
					assert.Equal(t, "", files[0].PreviousFilename)
					assert.EqualValues(t, 4, files[0].Additions)
					assert.EqualValues(t, 1, files[0].Deletions)
					assert.Equal(t, "changed", files[0].Status)
				}
			}))
	}

	pull = pulls[0]
	assert.EqualValues(t, 1, pull.Poster.ID)
	assert.Len(t, pull.RequestedReviewers, 2)
	assert.Empty(t, pull.RequestedReviewersTeams)
	assert.EqualValues(t, 5, pull.RequestedReviewers[0].ID)

	if assert.EqualValues(t, 5, pull.ID) {
		resp = ctx.Session.MakeRequest(t, NewRequest(t, "GET", pull.DiffURL), http.StatusOK)
		bs, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		patch, err := gitdiff.ParsePatch(t.Context(), 1000, 5000, 10, bytes.NewReader(bs), "")
		assert.NoError(t, err)
		assert.Len(t, patch.Files, 1)

		t.Run(fmt.Sprintf("APIGetPullFiles_%d", pull.ID),
			doAPIGetPullFiles(ctx, pull, func(t *testing.T, files []*api.ChangedFile) {
				assert.Len(t, files, 1)
			}))
	}
}

func TestAPIViewPullsByBaseHead(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/master/branch2", owner.Name, repo.Name).
		AddTokenAuth(ctx.Token)
	resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

	pull := &api.PullRequest{}
	DecodeJSON(t, resp, pull)
	assert.EqualValues(t, 3, pull.Index)
	assert.EqualValues(t, 2, pull.ID)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/master/branch-not-exist", owner.Name, repo.Name).
		AddTokenAuth(ctx.Token)
	ctx.Session.MakeRequest(t, req, http.StatusNotFound)
}

// TestAPIMergePullWIP ensures that we can't merge a WIP pull request
func TestAPIMergePullWIP(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{Status: issues_model.PullRequestStatusMergeable}, unittest.Cond("has_merged = ?", false))
	pr.LoadIssue(db.DefaultContext)
	issue_service.ChangeTitle(db.DefaultContext, pr.Issue, owner, setting.Repository.PullRequest.WorkInProgressPrefixes[0]+" "+pr.Issue.Title)

	// force reload
	pr.LoadAttributes(db.DefaultContext)

	assert.Contains(t, pr.Issue.Title, setting.Repository.PullRequest.WorkInProgressPrefixes[0])

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", owner.Name, repo.Name, pr.Index), &forms.MergePullRequestForm{
		MergeMessageField: pr.Issue.Title,
		Do:                string(repo_model.MergeStyleMerge),
	}).AddTokenAuth(token)

	MakeRequest(t, req, http.StatusMethodNotAllowed)
}

func TestAPICreatePullSuccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	// repo10 have code, pulls units.
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	// repo11 only have code unit but should still create pulls
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	owner11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo11.OwnerID})

	session := loginUser(t, owner11.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &api.CreatePullRequestOption{
		Head:  fmt.Sprintf("%s:master", owner11.Name),
		Base:  "master",
		Title: "create a failure pr",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
	MakeRequest(t, req, http.StatusUnprocessableEntity) // second request should fail
}

func TestAPICreatePullBasePermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	// repo10 have code, pulls units.
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	// repo11 only have code unit but should still create pulls
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	session := loginUser(t, user4.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	opts := &api.CreatePullRequestOption{
		Head:  fmt.Sprintf("%s:master", repo11.OwnerName),
		Base:  "master",
		Title: "create a failure pr",
	}
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &opts).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	// add user4 to be a collaborator to base repo
	ctx := NewAPITestContext(t, repo10.OwnerName, repo10.Name, auth_model.AccessTokenScopeWriteRepository)
	t.Run("AddUser4AsCollaborator", doAPIAddCollaborator(ctx, user4.Name, perm.AccessModeRead))

	// create again
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &opts).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

func TestAPICreatePullHeadPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	// repo10 have code, pulls units.
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	// repo11 only have code unit but should still create pulls
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	session := loginUser(t, user4.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	opts := &api.CreatePullRequestOption{
		Head:  fmt.Sprintf("%s:master", repo11.OwnerName),
		Base:  "master",
		Title: "create a failure pr",
	}
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &opts).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	// add user4 to be a collaborator to head repo with read permission
	ctx := NewAPITestContext(t, repo11.OwnerName, repo11.Name, auth_model.AccessTokenScopeWriteRepository)
	t.Run("AddUser4AsCollaboratorWithRead", doAPIAddCollaborator(ctx, user4.Name, perm.AccessModeRead))
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &opts).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	// add user4 to be a collaborator to head repo with write permission
	t.Run("AddUser4AsCollaboratorWithWrite", doAPIAddCollaborator(ctx, user4.Name, perm.AccessModeWrite))
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &opts).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

func TestAPICreatePullSameRepoSuccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner.Name, repo.Name), &api.CreatePullRequestOption{
		Head:  fmt.Sprintf("%s:pr-to-update", owner.Name),
		Base:  "master",
		Title: "successfully create a PR between branches of the same repository",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
	MakeRequest(t, req, http.StatusUnprocessableEntity) // second request should fail
}

func TestAPICreatePullWithFieldsSuccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// repo10 have code, pulls units.
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	// repo11 only have code unit but should still create pulls
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	owner11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo11.OwnerID})

	session := loginUser(t, owner11.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	opts := &api.CreatePullRequestOption{
		Head:      fmt.Sprintf("%s:master", owner11.Name),
		Base:      "master",
		Title:     "create a failure pr",
		Body:      "foobaaar",
		Milestone: 5,
		Assignees: []string{owner10.Name},
		Labels:    []int64{5},
	}

	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), opts).
		AddTokenAuth(token)

	res := MakeRequest(t, req, http.StatusCreated)
	pull := new(api.PullRequest)
	DecodeJSON(t, res, pull)

	assert.NotNil(t, pull.Milestone)
	assert.EqualValues(t, opts.Milestone, pull.Milestone.ID)
	if assert.Len(t, pull.Assignees, 1) {
		assert.EqualValues(t, opts.Assignees[0], owner10.Name)
	}
	assert.NotNil(t, pull.Labels)
	assert.EqualValues(t, opts.Labels[0], pull.Labels[0].ID)
}

func TestAPICreatePullWithFieldsFailure(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// repo10 have code, pulls units.
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	// repo11 only have code unit but should still create pulls
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	owner11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo11.OwnerID})

	session := loginUser(t, owner11.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	opts := &api.CreatePullRequestOption{
		Head: fmt.Sprintf("%s:master", owner11.Name),
		Base: "master",
	}

	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Title = "is required"

	opts.Milestone = 666
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Milestone = 5

	opts.Assignees = []string{"qweruqweroiuyqweoiruywqer"}
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Assignees = []string{owner10.LoginName}

	opts.Labels = []int64{55555}
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Labels = []int64{5}
}

func TestAPIEditPull(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})

	session := loginUser(t, owner10.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	title := "create a success pr"
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &api.CreatePullRequestOption{
		Head:  "develop",
		Base:  "master",
		Title: title,
	}).AddTokenAuth(token)
	apiPull := new(api.PullRequest)
	resp := MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, apiPull)
	assert.EqualValues(t, "master", apiPull.Base.Name)

	newTitle := "edit a this pr"
	newBody := "edited body"
	req = NewRequestWithJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", owner10.Name, repo10.Name, apiPull.Index), &api.EditPullRequestOption{
		Base:  "feature/1",
		Title: newTitle,
		Body:  &newBody,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, apiPull)
	assert.EqualValues(t, "feature/1", apiPull.Base.Name)
	// check comment history
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
	err := pull.LoadIssue(db.DefaultContext)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pull.Issue.ID, OldTitle: title, NewTitle: newTitle})
	unittest.AssertExistsAndLoadBean(t, &issues_model.ContentHistory{IssueID: pull.Issue.ID, ContentText: newBody, IsFirstCreated: false})

	req = NewRequestWithJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", owner10.Name, repo10.Name, pull.Index), &api.EditPullRequestOption{
		Base: "not-exist",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func doAPIGetPullFiles(ctx APITestContext, pr *api.PullRequest, callback func(*testing.T, []*api.ChangedFile)) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/files", ctx.Username, ctx.Reponame, pr.Index)).
			AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode == 0 {
			ctx.ExpectedCode = http.StatusOK
		}
		resp := ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)

		files := make([]*api.ChangedFile, 0, 1)
		DecodeJSON(t, resp, &files)

		if callback != nil {
			callback(t, files)
		}
	}
}

func TestAPICommitPullRequest(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	mergedCommitSHA := "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3"
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/commits/%s/pull", owner.Name, repo.Name, mergedCommitSHA).AddTokenAuth(ctx.Token)
	ctx.Session.MakeRequest(t, req, http.StatusOK)

	invalidCommitSHA := "abcd1234abcd1234abcd1234abcd1234abcd1234"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/commits/%s/pull", owner.Name, repo.Name, invalidCommitSHA).AddTokenAuth(ctx.Token)
	ctx.Session.MakeRequest(t, req, http.StatusNotFound)
}
