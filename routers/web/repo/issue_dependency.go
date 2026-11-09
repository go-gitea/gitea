// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"slices"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/unit"
	issue_indexer "gitea.dev/modules/indexer/issues"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/routers/common"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// IssueDependencyCandidates searches the open issues and pull requests that can be added as a dependency of the issue.
// It is used by the "add dependency" dropdown, so it leaves out the issue itself and the issues already linked to it.
func IssueDependencyCandidates(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	excludedIDs, err := issues_model.GetLinkedDependencyIssueIDs(ctx, issue.ID)
	if err != nil {
		ctx.ServerError("GetLinkedDependencyIssueIDs", err)
		return
	}
	excludedIDs = append(excludedIDs, issue.ID)

	// The indexers can't leave out a list of issue IDs. Asking for one more result per excluded issue is enough:
	// whichever of them are among the results, the first "limit" results left are the ones the dropdown should show.
	limit := convert.ToCorrectPageSize(ctx.FormInt("limit"))
	searchOpt := &issue_indexer.SearchOptions{
		Paginator: &db.ListOptions{Page: 1, PageSize: limit + len(excludedIDs)},
		Keyword:   ctx.FormTrim("q"),
		RepoIDs:   []int64{ctx.Repo.Repository.ID},
		IsClosed:  optional.Some(false),
		SortBy:    issue_indexer.SortByCreatedDesc,
	}
	if !ctx.Repo.Permission.CanRead(unit.TypeIssues) {
		searchOpt.IsPull = optional.Some(true)
	} else if !ctx.Repo.Permission.CanRead(unit.TypePullRequests) {
		searchOpt.IsPull = optional.Some(false)
	}
	if setting.Service.AllowCrossRepositoryDependencies {
		searchOpt.RepoIDs, searchOpt.AllPublic, err = common.SearchIssuesRepoIDs(ctx, common.SearchIssuesRepoIDsOptions{Doer: ctx.Doer})
		if err != nil {
			ctx.ServerError("SearchIssuesRepoIDs", err)
			return
		}
	}

	ids, _, err := issue_indexer.SearchIssues(ctx, searchOpt)
	if err != nil {
		ctx.ServerError("SearchIssues", err)
		return
	}
	ids = slices.DeleteFunc(ids, func(id int64) bool { return slices.Contains(excludedIDs, id) })
	issues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
	if err != nil {
		ctx.ServerError("GetIssuesByIDs", err)
		return
	}
	if _, err = issues.LoadRepositories(ctx); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}

	// A repository can be accessible through a team that doesn't have its issues or pull requests unit.
	// AddDependency refuses the issues the doer can't read, so they are no candidates either.
	perms := map[int64]access_model.Permission{ctx.Repo.Repository.ID: ctx.Repo.Permission}
	candidates := make(issues_model.IssueList, 0, limit)
	for _, candidate := range issues {
		if len(candidates) == limit {
			break
		}
		perm, ok := perms[candidate.RepoID]
		if !ok {
			if perm, err = access_model.GetDoerRepoPermission(ctx, candidate.Repo, ctx.Doer); err != nil {
				ctx.ServerError("GetDoerRepoPermission", err)
				return
			}
			perms[candidate.RepoID] = perm
		}
		if perm.CanReadIssuesOrPulls(candidate.IsPull) {
			candidates = append(candidates, candidate)
		}
	}

	ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, ctx.Doer, candidates))
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
		ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
		return
	}

	depID := ctx.FormInt64("newDependency")

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerError("LoadRepo", err)
		return
	}

	// Dependency
	dep, err := issues_model.GetIssueByID(ctx, depID)
	if err != nil {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_issue_not_exist"))
		return
	}

	// Check if both issues are in the same repo if cross repository dependencies is not enabled
	if issue.RepoID != dep.RepoID {
		if !setting.Service.AllowCrossRepositoryDependencies {
			ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_not_same_repo"))
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
			ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
			return
		}
	}

	// Check if issue and dependency is the same
	if dep.ID == issue.ID {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_same_issue"))
		return
	}

	err = issues_model.CreateIssueDependency(ctx, ctx.Doer, issue, dep)
	if issues_model.IsErrDependencyExists(err) {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_exists"))
		return
	} else if issues_model.IsErrCircularDependency(err) {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_cannot_create_circular"))
		return
	} else if err != nil {
		ctx.ServerError("CreateOrUpdateIssueDependency", err)
		return
	}
	ctx.JSONOK()
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
		ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
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
		ctx.JSONError("invalid dependency type")
		return
	}

	// Dependency
	dep, err := issues_model.GetIssueByID(ctx, depID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	// Existing cross-repo dependencies must remain removable even when
	// AllowCrossRepositoryDependencies is disabled, so only enforce that the
	// doer can read the dependency's repository.
	if issue.RepoID != dep.RepoID {
		if err := dep.LoadRepo(ctx); err != nil {
			ctx.ServerError("loadRepo", err)
			return
		}
		depRepoPerm, err := access_model.GetDoerRepoPermission(ctx, dep.Repo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetDoerRepoPermission", err)
			return
		}
		if !depRepoPerm.CanReadIssuesOrPulls(dep.IsPull) {
			ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
			return
		}
	}

	err = issues_model.RemoveIssueDependency(ctx, ctx.Doer, issue, dep, depType)
	if issues_model.IsErrDependencyNotExists(err) {
		ctx.JSONError(ctx.Tr("repo.issues.dependency.add_error_dep_not_exist"))
		return
	} else if err != nil {
		ctx.ServerError("RemoveIssueDependency", err)
		return
	}
	ctx.JSONOK()
}
