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
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if !ctx.Repo.Permission.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	excludedIDs, err := issues_model.GetLinkedDependencyIssueIDs(ctx, issue.ID)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	excludedIDs = append(excludedIDs, issue.ID)

	limit := convert.ToCorrectPageSize(ctx.FormInt("limit"))
	searchOpt := &issue_indexer.SearchOptions{
		Paginator: &db.ListOptions{PageSize: min(limit+len(excludedIDs), setting.API.MaxResponseItems)},
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
			ctx.ServerErrorAuto(err)
			return
		}
	}

	perms := map[int64]access_model.Permission{ctx.Repo.Repository.ID: ctx.Repo.Permission}
	candidates := make(issues_model.IssueList, 0, limit)
	// the indexers can't leave out the linked issues or the ones the doer can't read, so search on until the page is full
	for page := 1; page <= 10 && len(candidates) < limit; page++ {
		searchOpt.Paginator.Page = page
		ids, _, err := issue_indexer.SearchIssues(ctx, searchOpt)
		if err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		hasMore := len(ids) == searchOpt.Paginator.PageSize
		ids = slices.DeleteFunc(ids, func(id int64) bool { return slices.Contains(excludedIDs, id) })
		issues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
		if err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		if _, err = issues.LoadRepositories(ctx); err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		for _, candidate := range issues {
			if len(candidates) == limit {
				break
			}
			perm, ok := perms[candidate.RepoID]
			if !ok {
				if perm, err = access_model.GetDoerRepoPermission(ctx, candidate.Repo, ctx.Doer); err != nil {
					ctx.ServerErrorAuto(err)
					return
				}
				perms[candidate.RepoID] = perm
			}
			if perm.CanReadIssuesOrPulls(candidate.IsPull) {
				candidates = append(candidates, candidate)
			}
		}
		if !hasMore {
			break
		}
	}

	ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, ctx.Doer, candidates))
}

// AddDependency adds new dependencies
func AddDependency(ctx *context.Context) {
	issueIndex := ctx.PathParamInt64("index")
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
		return
	}

	depID := ctx.FormInt64("newDependency")

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerErrorAuto(err)
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
			ctx.ServerErrorAuto(err)
			return
		}
		// Can ctx.Doer read issues in the dep repo?
		depRepoPerm, err := access_model.GetDoerRepoPermission(ctx, dep.Repo, ctx.Doer)
		if err != nil {
			ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.JSONOK()
}

// RemoveDependency removes the dependency
func RemoveDependency(ctx *context.Context) {
	issueIndex := ctx.PathParamInt64("index")
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull) {
		ctx.JSONError(ctx.Locale.TrString("error.permission_denied"))
		return
	}

	depID := ctx.FormInt64("removeDependencyID")

	if err = issue.LoadRepo(ctx); err != nil {
		ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
		return
	}

	// Existing cross-repo dependencies must remain removable even when
	// AllowCrossRepositoryDependencies is disabled, so only enforce that the
	// doer can read the dependency's repository.
	if issue.RepoID != dep.RepoID {
		if err := dep.LoadRepo(ctx); err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		depRepoPerm, err := access_model.GetDoerRepoPermission(ctx, dep.Repo, ctx.Doer)
		if err != nil {
			ctx.ServerErrorAuto(err)
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
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.JSONOK()
}
