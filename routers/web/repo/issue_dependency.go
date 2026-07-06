// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

var dependencyRefPattern = regexp.MustCompile(`^\s*(?:(?P<owner>[0-9a-zA-Z_.-]+)/(?P<repo>[0-9a-zA-Z_.-]+))?(?P<type>[#!])(?P<index>[0-9]+)\s*$`)

func parseDependencyRef(ref string) (owner, repo string, index int64, isPull, ok bool) {
	match := dependencyRefPattern.FindStringSubmatch(ref)
	if match == nil {
		return "", "", 0, false, false
	}

	for i, name := range dependencyRefPattern.SubexpNames() {
		switch name {
		case "owner":
			owner = match[i]
		case "repo":
			repo = match[i]
		case "type":
			isPull = match[i] == "!"
		case "index":
			var err error
			index, err = strconv.ParseInt(match[i], 10, 64)
			if err != nil {
				return "", "", 0, false, false
			}
		}
	}

	return owner, repo, index, isPull, true
}

func dependencySearchTypeAllows(searchType string, isPull bool) bool {
	switch searchType {
	case "pulls":
		return isPull
	case "issues":
		return !isPull
	case "", "all":
		return true
	default:
		return false
	}
}

func getDependencyByRef(ctx *context.Context, currentIssueID int64, ref, searchType string) (*issues_model.Issue, error) {
	owner, repoName, index, isPull, ok := parseDependencyRef(ref)
	if !ok || !dependencySearchTypeAllows(searchType, isPull) {
		return nil, util.ErrNotExist
	}

	depRepo := ctx.Repo.Repository
	if owner != "" || repoName != "" {
		var err error
		depRepo, err = repo_model.GetRepositoryByOwnerAndName(ctx, owner, repoName)
		if err != nil {
			return nil, err
		}
		if depRepo.ID != ctx.Repo.Repository.ID && !setting.Service.AllowCrossRepositoryDependencies {
			return nil, util.ErrNotExist
		}
	}

	dep, err := issues_model.GetIssueByIndex(ctx, depRepo.ID, index)
	if err != nil {
		return nil, err
	}
	if dep.IsPull != isPull || dep.ID == currentIssueID {
		return nil, util.ErrNotExist
	}
	dep.Repo = depRepo

	depRepoPerm := ctx.Repo.Permission
	if depRepo.ID != ctx.Repo.Repository.ID {
		depRepoPerm, err = access_model.GetDoerRepoPermission(ctx, depRepo, ctx.Doer)
		if err != nil {
			return nil, err
		}
	}
	if !depRepoPerm.CanReadIssuesOrPulls(dep.IsPull) {
		return nil, util.ErrNotExist
	}

	return dep, nil
}

// SearchDependencyByRef resolves exact dependency references for the dependency dropdown.
func SearchDependencyByRef(ctx *context.Context) {
	dep, err := getDependencyByRef(ctx, ctx.FormInt64("issue_id"), ctx.FormTrim("ref"), ctx.FormString("type"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) || issues_model.IsErrIssueNotExist(err) || repo_model.IsErrRepoNotExist(err) {
			ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, ctx.Doer, issues_model.IssueList{}))
			return
		}
		ctx.ServerError("getDependencyByRef", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, ctx.Doer, issues_model.IssueList{dep}))
}

// AddDependency adds new dependencies
func AddDependency(ctx *context.Context) {
	issueIndex := ctx.PathParamInt64("index")
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.HTTPError(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Redirect
	defer ctx.Redirect(issue.Link())

	depRef := ctx.FormTrim("newDependency")
	var dep *issues_model.Issue
	if depID, err := strconv.ParseInt(depRef, 10, 64); err == nil {
		dep, err = issues_model.GetIssueByID(ctx, depID)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_issue_not_exist"))
			return
		}
	} else {
		dep, err = getDependencyByRef(ctx, issue.ID, depRef, "all")
		if err != nil {
			if errors.Is(err, util.ErrNotExist) || issues_model.IsErrIssueNotExist(err) || repo_model.IsErrRepoNotExist(err) {
				ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_issue_not_exist"))
				return
			}
			ctx.ServerError("getDependencyByRef", err)
			return
		}
	}

	// Check if both issues are in the same repo if cross repository dependencies is not enabled
	if issue.RepoID != dep.RepoID {
		if !setting.Service.AllowCrossRepositoryDependencies {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_same_repo"))
			return
		}
		if err := dep.LoadRepo(ctx); err != nil {
			ctx.ServerError("loadRepo", err)
			return
		}
		// Can ctx.Doer read issues in the dep repo?
		depRepoPerm, err := access_model.GetDoerRepoPermission(ctx, dep.Repo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetDoerRepoPermission", err)
			return
		}
		if !depRepoPerm.CanReadIssuesOrPulls(dep.IsPull) {
			// you can't see this dependency
			return
		}
	}

	// Check if issue and dependency is the same
	if dep.ID == issue.ID {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_same_issue"))
		return
	}

	err = issues_model.CreateIssueDependency(ctx, ctx.Doer, issue, dep)
	if err != nil {
		if issues_model.IsErrDependencyExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_exists"))
			return
		} else if issues_model.IsErrCircularDependency(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_cannot_create_circular"))
			return
		}
		ctx.ServerError("CreateOrUpdateIssueDependency", err)
		return
	}
}

// RemoveDependency removes the dependency
func RemoveDependency(ctx *context.Context) {
	issueIndex := ctx.PathParamInt64("index")
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.HTTPError(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	depID := ctx.FormInt64("removeDependencyID")

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Dependency Type
	depTypeStr := ctx.Req.PostFormValue("dependencyType")

	var depType issues_model.DependencyType

	switch depTypeStr {
	case "blockedBy":
		depType = issues_model.DependencyTypeBlockedBy
	case "blocking":
		depType = issues_model.DependencyTypeBlocking
	default:
		ctx.HTTPError(http.StatusBadRequest, "GetDependencyType")
		return
	}

	// Dependency
	dep, err := issues_model.GetIssueByID(ctx, depID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	if err = issues_model.RemoveIssueDependency(ctx, ctx.Doer, issue, dep, depType); err != nil {
		if issues_model.IsErrDependencyNotExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_exist"))
			return
		}
		ctx.ServerError("RemoveIssueDependency", err)
		return
	}

	// Redirect
	ctx.Redirect(issue.Link())
}
