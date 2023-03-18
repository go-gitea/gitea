// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
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
	// summary: List an issue's dependencies
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

	if !ctx.Repo.Repository.IsDependenciesEnabled(ctx) {
		ctx.NotFound()
		return
	}

	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if issue.IsPull {
		if !ctx.Repo.CanRead(unit.TypePullRequests) {
			ctx.NotFound()
			return
		}
	} else {
		if !ctx.Repo.CanRead(unit.TypeIssues) {
			ctx.NotFound()
			return
		}
	}

	deps, err := issue.BlockedByDependencies(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "BlockedByDependencies", err)
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

	var issues []*issues_model.Issue
	for i, depMeta := range deps {
		if i < skip || i >= max {
			continue
		}

		perm, err := access_model.GetUserRepoPermission(ctx, &depMeta.Repository, ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}
		if depMeta.Issue.IsPull {
			if !perm.CanRead(unit.TypePullRequests) {
				continue
			}
		} else {
			if !perm.CanRead(unit.TypeIssues) {
				continue
			}
		}

		depMeta.Issue.Repo = &depMeta.Repository
		issues = append(issues, &depMeta.Issue)
	}

	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, issues))
}

// CreateIssueDependency create a new issue dependencies
func CreateIssueDependency(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/dependencies issue issueCreateIssueDependencies
	// ---
	// summary: Create a new issue dependencies
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

	createIssueDependency(ctx, issues_model.DependencyTypeBlockedBy)
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

	removeIssueDependency(ctx, issues_model.DependencyTypeBlockedBy)
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

	if !ctx.Repo.Repository.IsDependenciesEnabled(ctx) {
		ctx.NotFound()
		return
	}

	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if issue.IsPull {
		if !ctx.Repo.CanRead(unit.TypePullRequests) {
			ctx.NotFound()
			return
		}
	} else {
		if !ctx.Repo.CanRead(unit.TypeIssues) {
			ctx.NotFound()
			return
		}
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

	var issues []*issues_model.Issue
	for i, depMeta := range deps {
		if i < skip || i >= max {
			continue
		}

		perm, err := access_model.GetUserRepoPermission(ctx, &depMeta.Repository, ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}
		if depMeta.Issue.IsPull {
			if !perm.CanRead(unit.TypePullRequests) {
				continue
			}
		} else {
			if !perm.CanRead(unit.TypeIssues) {
				continue
			}
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

	createIssueDependency(ctx, issues_model.DependencyTypeBlocking)
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

	removeIssueDependency(ctx, issues_model.DependencyTypeBlocking)
}

func createIssueDependency(ctx *context.APIContext, t issues_model.DependencyType) {
	if !ctx.Repo.Repository.IsDependenciesEnabled(ctx) {
		ctx.NotFound()
		return
	}

	dep, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if dep.IsPull {
		if !ctx.Repo.CanWrite(unit.TypePullRequests) {
			ctx.NotFound()
			return
		}
	} else {
		if !ctx.Repo.CanWrite(unit.TypeIssues) {
			ctx.NotFound()
			return
		}
	}

	form := web.GetForm(ctx).(*api.IssueMeta)
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, form.Owner, form.Name)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.NotFound("IsErrRepoNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRepositoryByOwnerAndName", err)
		}
		return
	}

	issue, err := issues_model.GetIssueByIndex(repo.ID, form.Index)
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if t == issues_model.DependencyTypeBlockedBy {
		perm, err := access_model.GetUserRepoPermission(ctx, ctx.Repo.Repository, ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}
		if issue.IsPull {
			if !perm.CanRead(unit.TypePullRequests) {
				ctx.NotFound()
				return
			}
		} else {
			if !perm.CanRead(unit.TypeIssues) {
				ctx.NotFound()
				return
			}
		}

		err = issues_model.CreateIssueDependency(ctx.Doer, issue, dep)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "CreateIssueDependency", err)
			return
		}
	} else {
		perm, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}
		if issue.IsPull {
			if !perm.CanRead(unit.TypePullRequests) {
				ctx.NotFound()
				return
			}
		} else {
			if !perm.CanRead(unit.TypeIssues) {
				ctx.NotFound()
				return
			}
		}

		err = issues_model.CreateIssueDependency(ctx.Doer, dep, issue)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "CreateIssueDependency", err)
			return
		}
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, dep))
}

func removeIssueDependency(ctx *context.APIContext, t issues_model.DependencyType) {
	if !ctx.Repo.Repository.IsDependenciesEnabled(ctx) {
		ctx.NotFound()
		return
	}

	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if issue.IsPull {
		if !ctx.Repo.CanWrite(unit.TypePullRequests) {
			ctx.NotFound()
			return
		}
	} else {
		if !ctx.Repo.CanWrite(unit.TypeIssues) {
			ctx.NotFound()
			return
		}
	}

	form := web.GetForm(ctx).(*api.IssueMeta)
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, form.Owner, form.Name)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.NotFound("IsErrRepoNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRepositoryByOwnerAndName", err)
		}
		return
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return
	}

	dep, err := issues_model.GetIssueWithAttrsByIndex(repo.ID, form.Index)
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IsErrIssueNotExist", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if issue.IsPull {
		if !perm.CanRead(unit.TypePullRequests) {
			ctx.NotFound("IsErrRepoNotExist", err)
			return
		}
	} else {
		if !perm.CanRead(unit.TypeIssues) {
			ctx.NotFound("IsErrRepoNotExist", err)
			return
		}
	}

	err = issues_model.RemoveIssueDependency(ctx.Doer, issue, dep, t)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateIssueDependency", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIIssue(ctx, dep))
}
