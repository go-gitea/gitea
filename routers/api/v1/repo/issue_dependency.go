// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/convert"
)

// GetIssueDependencies list an issue's dependencies
func GetIssueDependencies(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/dependencies issue issueListIssueDependencies
	// ---
	// summary: List an issue's dependencies, i.e all issues that block this issue.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// If this issue's repository does not enable dependencies then there can be no dependencies by default
	if !ctx.Repo.Repository.IsDependenciesEnabled(ctx) {
		ctx.NotFound()
		return
	}

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	// 1. We must be able to read this issue
	if !ctx.Repo.Permission.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	limit := ctx.FormInt("limit")
	if limit == 0 {
		limit = setting.API.DefaultPagingNum
	} else if limit > setting.API.MaxResponseItems {
		limit = setting.API.MaxResponseItems
	}

	canWrite := ctx.Repo.Permission.CanWriteIssuesOrPulls(issue.IsPull)

	blockerIssues := make([]*issues_model.Issue, 0, limit)

	// 2. Get the issues this issue depends on, i.e. the `<#b>`: `<issue> <- <#b>`
	blockersInfo, err := issue.BlockedByDependencies(ctx, db.ListOptions{
		Page:     page,
		PageSize: limit,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "BlockedByDependencies", err)
		return
	}

	var lastRepoID int64
	var lastPerm access_model.Permission
	for _, blocker := range blockersInfo {
		// Get the permissions for this repository
		perm := lastPerm
		if lastRepoID != blocker.Repository.ID {
			if blocker.Repository.ID == ctx.Repo.Repository.ID {
				perm = ctx.Repo.Permission
			} else {
				var err error
				perm, err = access_model.GetUserRepoPermission(ctx, &blocker.Repository, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}
			}
			lastRepoID = blocker.Repository.ID
		}

		// check permission
		if !perm.CanReadIssuesOrPulls(blocker.Issue.IsPull) {
			if !canWrite {
				hiddenBlocker := &issues_model.DependencyInfo{
					Issue: issues_model.Issue{
						Title: "HIDDEN",
					},
				}
				blocker = hiddenBlocker
			} else {
				confidentialBlocker := &issues_model.DependencyInfo{
					Issue: issues_model.Issue{
						RepoID:   blocker.Issue.RepoID,
						Index:    blocker.Index,
						Title:    blocker.Title,
						IsClosed: blocker.IsClosed,
						IsPull:   blocker.IsPull,
					},
					Repository: repo_model.Repository{
						ID:        blocker.Issue.Repo.ID,
						Name:      blocker.Issue.Repo.Name,
						OwnerName: blocker.Issue.Repo.OwnerName,
					},
				}
				confidentialBlocker.Issue.Repo = &confidentialBlocker.Repository
				blocker = confidentialBlocker
			}
		}
		blockerIssues = append(blockerIssues, &blocker.Issue)
	}

	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, blockerIssues))
}

// CreateIssueDependency create a new issue dependencies
func CreateIssueDependency(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/dependencies issue issueCreateIssueDependencies
	// ---
	// summary: Make the issue in the url depend on the issue in the form.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueMeta"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Issue"
	//   "404":
	//     description: the issue does not exist
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	// We want to make <:index> depend on <Form>, i.e. <:index> is the target
	target := getParamsIssue(ctx)
	if ctx.Written() {
		return
	}

	// and <Form> represents the dependency
	form := web.GetForm(ctx).(*api.IssueMeta)
	dependency := getFormIssue(ctx, form)
	if ctx.Written() {
		return
	}

	dependencyPerm := getPermissionForRepo(ctx, target.Repo)
	if ctx.Written() {
		return
	}

	createIssueDependency(ctx, target, dependency, ctx.Repo.Permission, *dependencyPerm)
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, target))
}

// RemoveIssueDependency remove an issue dependency
func RemoveIssueDependency(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/dependencies issue issueRemoveIssueDependencies
	// ---
	// summary: Remove an issue dependency
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueMeta"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Issue"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	// We want to make <:index> depend on <Form>, i.e. <:index> is the target
	target := getParamsIssue(ctx)
	if ctx.Written() {
		return
	}

	// and <Form> represents the dependency
	form := web.GetForm(ctx).(*api.IssueMeta)
	dependency := getFormIssue(ctx, form)
	if ctx.Written() {
		return
	}

	dependencyPerm := getPermissionForRepo(ctx, target.Repo)
	if ctx.Written() {
		return
	}

	removeIssueDependency(ctx, target, dependency, ctx.Repo.Permission, *dependencyPerm)
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, target))
}

// GetIssueBlocks list issues that are blocked by this issue
func GetIssueBlocks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/blocks issue issueListBlocks
	// ---
	// summary: List issues that are blocked by this issue
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// We need to list the issues that DEPEND on this issue not the other way round
	// Therefore whether dependencies are enabled or not in this repository is potentially irrelevant.

	issue := getParamsIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.Repo.Permission.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	limit := ctx.FormInt("limit")
	if limit <= 1 {
		limit = setting.API.DefaultPagingNum
	}

	skip := (page - 1) * limit
	max := page * limit

	deps, err := issue.BlockingDependencies(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "BlockingDependencies", err)
		return
	}

	var lastRepoID int64
	var lastPerm access_model.Permission

	var issues []*issues_model.Issue
	for i, depMeta := range deps {
		if i < skip || i >= max {
			continue
		}

		// Get the permissions for this repository
		perm := lastPerm
		if lastRepoID != depMeta.Repository.ID {
			if depMeta.Repository.ID == ctx.Repo.Repository.ID {
				perm = ctx.Repo.Permission
			} else {
				var err error
				perm, err = access_model.GetUserRepoPermission(ctx, &depMeta.Repository, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}
			}
			lastRepoID = depMeta.Repository.ID
		}

		if !perm.CanReadIssuesOrPulls(depMeta.Issue.IsPull) {
			continue
		}

		depMeta.Issue.Repo = &depMeta.Repository
		issues = append(issues, &depMeta.Issue)
	}

	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, issues))
}

// CreateIssueBlocking block the issue given in the body by the issue in path
func CreateIssueBlocking(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/blocks issue issueCreateIssueBlocking
	// ---
	// summary: Block the issue given in the body by the issue in path
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueMeta"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Issue"
	//   "404":
	//     description: the issue does not exist

	dependency := getParamsIssue(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.IssueMeta)
	target := getFormIssue(ctx, form)
	if ctx.Written() {
		return
	}

	targetPerm := getPermissionForRepo(ctx, target.Repo)
	if ctx.Written() {
		return
	}

	createIssueDependency(ctx, target, dependency, *targetPerm, ctx.Repo.Permission)
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, dependency))
}

// RemoveIssueBlocking unblock the issue given in the body by the issue in path
func RemoveIssueBlocking(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/blocks issue issueRemoveIssueBlocking
	// ---
	// summary: Unblock the issue given in the body by the issue in path
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueMeta"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Issue"
	//   "404":
	//     "$ref": "#/responses/notFound"

	dependency := getParamsIssue(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.IssueMeta)
	target := getFormIssue(ctx, form)
	if ctx.Written() {
		return
	}

	targetPerm := getPermissionForRepo(ctx, target.Repo)
	if ctx.Written() {
		return
	}

	removeIssueDependency(ctx, target, dependency, *targetPerm, ctx.Repo.Permission)
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, dependency))
}

func getParamsIssue(ctx *context.APIContext) *issues_model.Issue {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return nil
	}
	issue.Repo = ctx.Repo.Repository
	return issue
}

func getFormIssue(ctx *context.APIContext, form *api.IssueMeta) *issues_model.Issue {
	var repo *repo_model.Repository
	if form.Owner != ctx.Repo.Repository.OwnerName || form.Name != ctx.Repo.Repository.Name {
		if !setting.Service.AllowCrossRepositoryDependencies {
			ctx.JSON(http.StatusBadRequest, "CrossRepositoryDependencies not enabled")
			return nil
		}
		var err error
		repo, err = repo_model.GetRepositoryByOwnerAndName(ctx, form.Owner, form.Name)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				ctx.NotFound("IsErrRepoNotExist", err)
			} else {
				ctx.Error(http.StatusInternalServerError, "GetRepositoryByOwnerAndName", err)
			}
			return nil
		}
	} else {
		repo = ctx.Repo.Repository
	}

	issue, err := issues_model.GetIssueByIndex(ctx, repo.ID, form.Index)
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return nil
	}
	issue.Repo = repo
	return issue
}

func getPermissionForRepo(ctx *context.APIContext, repo *repo_model.Repository) *access_model.Permission {
	if repo.ID == ctx.Repo.Repository.ID {
		return &ctx.Repo.Permission
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return nil
	}

	return &perm
}

func createIssueDependency(ctx *context.APIContext, target, dependency *issues_model.Issue, targetPerm, dependencyPerm access_model.Permission) {
	if target.Repo.IsArchived || !target.Repo.IsDependenciesEnabled(ctx) {
		// The target's repository doesn't have dependencies enabled
		ctx.NotFound()
		return
	}

	if !targetPerm.CanWriteIssuesOrPulls(target.IsPull) {
		// We can't write to the target
		ctx.NotFound()
		return
	}

	if !dependencyPerm.CanReadIssuesOrPulls(dependency.IsPull) {
		// We can't read the dependency
		ctx.NotFound()
		return
	}

	err := issues_model.CreateIssueDependency(ctx, ctx.Doer, target, dependency)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateIssueDependency", err)
		return
	}
}

func removeIssueDependency(ctx *context.APIContext, target, dependency *issues_model.Issue, targetPerm, dependencyPerm access_model.Permission) {
	if target.Repo.IsArchived || !target.Repo.IsDependenciesEnabled(ctx) {
		// The target's repository doesn't have dependencies enabled
		ctx.NotFound()
		return
	}

	if !targetPerm.CanWriteIssuesOrPulls(target.IsPull) {
		// We can't write to the target
		ctx.NotFound()
		return
	}

	if !dependencyPerm.CanReadIssuesOrPulls(dependency.IsPull) {
		// We can't read the dependency
		ctx.NotFound()
		return
	}

	err := issues_model.RemoveIssueDependency(ctx, ctx.Doer, target, dependency, issues_model.DependencyTypeBlockedBy)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateIssueDependency", err)
		return
	}
}
