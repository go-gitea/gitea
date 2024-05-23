// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/forms"
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

	pull := pulls[0]
	if assert.EqualValues(t, 5, pull.ID) {
		resp = ctx.Session.MakeRequest(t, NewRequest(t, "GET", pull.DiffURL), http.StatusOK)
		_, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		// TODO: use diff to generate stats to test against

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
