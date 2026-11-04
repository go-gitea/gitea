// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/optional"
	"gitea.dev/services/context"
	issue_service "gitea.dev/services/issue"
)

// redirectMovedIssue redirects to the new location of an issue moved away from the current repository,
// if the doer can read it there
func redirectMovedIssue(ctx *context.Context, index int64) bool {
	redirect, err := issues_model.GetIssueRedirect(ctx, ctx.Repo.Repository.ID, index)
	if err != nil {
		return false
	}
	issue, err := issues_model.GetIssueByID(ctx, redirect.IssueID)
	if err != nil {
		return false
	}
	if err := issue.LoadRepo(ctx); err != nil {
		return false
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, issue.Repo, ctx.Doer)
	if err != nil || !perm.CanRead(unit.TypeIssues) {
		return false
	}
	ctx.Redirect(issue.Link())
	return true
}

func getMoveIssueTargetRepo(ctx *context.Context) *repo_model.Repository {
	ownerName, repoName, ok := strings.Cut(ctx.FormString("target_repo"), "/")
	if !ok || ownerName == "" || repoName == "" {
		ctx.JSONError(ctx.Tr("repo.issues.move_copy.target_invalid"))
		return nil
	}
	targetRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
	if repo_model.IsErrRepoNotExist(err) {
		ctx.JSONError(ctx.Tr("repo.issues.move_copy.target_invalid"))
		return nil
	} else if err != nil {
		ctx.ServerError("GetRepositoryByOwnerAndName", err)
		return nil
	}
	return targetRepo
}

func getMoveIssueOptions(ctx *context.Context) issue_service.MoveIssueOptions {
	return issue_service.MoveIssueOptions{
		KeepLabels:    ctx.FormBool("keep_labels"),
		KeepMilestone: ctx.FormBool("keep_milestone"),
		KeepProjects:  ctx.FormBool("keep_projects"),
	}
}

// MoveIssue moves an issue to another repository
func MoveIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}
	targetRepo := getMoveIssueTargetRepo(ctx)
	if ctx.Written() {
		return
	}
	if err := issue_service.MoveIssue(ctx, ctx.Doer, issue, targetRepo, getMoveIssueOptions(ctx)); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSONRedirect(issue.Link())
}

// CopyIssue copies an issue to another repository
func CopyIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}
	targetRepo := getMoveIssueTargetRepo(ctx)
	if ctx.Written() {
		return
	}
	newIssue, err := issue_service.CopyIssue(ctx, ctx.Doer, issue, targetRepo, issue_service.CopyIssueOptions{
		MoveIssueOptions: getMoveIssueOptions(ctx),
		CopyAttachments:  ctx.FormBool("copy_attachments"),
		CopyAssignees:    ctx.FormBool("copy_assignees"),
		Dependency:       issue_service.CopyIssueDependency(ctx.FormString("dependency")),
	})
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSONRedirect(newIssue.Link())
}

// MoveIssueTargetRepos returns the repositories in which the doer can write issues, ordered by:
// the current owner's repositories, the doer's organizations' repositories, the doer's own repositories, then the others.
func MoveIssueTargetRepos(ctx *context.Context) {
	repos, _, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 50},
		Actor:       ctx.Doer,
		OwnerID:     ctx.Doer.ID,
		Private:     true,
		Keyword:     ctx.FormTrim("q"),
		Archived:    optional.Some(false),
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}
	orgs, err := organization.GetUserOrgsList(ctx, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserOrgsList", err)
		return
	}
	orgIDs := make(map[int64]bool, len(orgs))
	for _, org := range orgs {
		orgIDs[org.ID] = true
	}

	type targetRepo struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	}
	groups := make([][]targetRepo, 4)
	for _, repo := range repos {
		if repo.ID == ctx.Repo.Repository.ID {
			continue
		}
		perm, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetDoerRepoPermission", err)
			return
		}
		if !perm.CanWrite(unit.TypeIssues) {
			continue
		}
		group := 3
		switch {
		case repo.OwnerID == ctx.Repo.Repository.OwnerID:
			group = 0
		case orgIDs[repo.OwnerID]:
			group = 1
		case repo.OwnerID == ctx.Doer.ID:
			group = 2
		}
		groups[group] = append(groups[group], targetRepo{ID: repo.ID, FullName: repo.FullName()})
	}
	result := make([]targetRepo, 0, len(repos))
	for _, group := range groups {
		result = append(result, group...)
	}
	ctx.JSON(http.StatusOK, result)
}
