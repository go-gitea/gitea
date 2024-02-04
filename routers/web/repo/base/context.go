// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"slices"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"
)

// RetrieveRepoMetas find all the meta information of a repository
func RetrieveRepoMetas(ctx *context.Context, repo *repo_model.Repository, isPull bool) []*issues_model.Label {
	if !ctx.Repo.CanWriteIssuesOrPulls(isPull) {
		return nil
	}

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return nil
	}
	ctx.Data["Labels"] = labels
	if repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{})
		if err != nil {
			return nil
		}

		ctx.Data["OrgLabels"] = orgLabels
		labels = append(labels, orgLabels...)
	}

	RetrieveRepoMilestonesAndAssignees(ctx, repo)
	if ctx.Written() {
		return nil
	}

	RetrieveProjects(ctx, repo)
	if ctx.Written() {
		return nil
	}

	PrepareBranchList(ctx)
	if ctx.Written() {
		return nil
	}

	// Contains true if the user can create issue dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, isPull)

	return labels
}

func PrepareBranchList(ctx *context.Context) {
	branchOpts := git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		IsDeletedBranch: util.OptionalBoolFalse,
		ListOptions: db.ListOptions{
			ListAll: true,
		},
	}
	brs, err := git_model.FindBranchNames(ctx, branchOpts)
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return
	}
	// always put default branch on the top if it exists
	if slices.Contains(brs, ctx.Repo.Repository.DefaultBranch) {
		brs = util.SliceRemoveAll(brs, ctx.Repo.Repository.DefaultBranch)
		brs = append([]string{ctx.Repo.Repository.DefaultBranch}, brs...)
	}
	ctx.Data["Branches"] = brs
}

// RetrieveRepoMilestonesAndAssignees find all the milestones and assignees of a repository
func RetrieveRepoMilestonesAndAssignees(ctx *context.Context, repo *repo_model.Repository) {
	var err error
	ctx.Data["OpenMilestones"], err = db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: util.OptionalBoolFalse,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
	ctx.Data["ClosedMilestones"], err = db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: util.OptionalBoolTrue,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}

	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, repo)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = MakeSelfOnTop(ctx.Doer, assigneeUsers)

	HandleTeamMentions(ctx)
}

// HandleTeamMentions get all teams that current user can mention
func HandleTeamMentions(ctx *context.Context) {
	if ctx.Doer == nil || !ctx.Repo.Owner.IsOrganization() {
		return
	}

	var isAdmin bool
	var err error
	var teams []*organization.Team
	org := organization.OrgFromUser(ctx.Repo.Owner)
	// Admin has super access.
	if ctx.Doer.IsAdmin {
		isAdmin = true
	} else {
		isAdmin, err = org.IsOwnedBy(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("IsOwnedBy", err)
			return
		}
	}

	if isAdmin {
		teams, err = org.LoadTeams(ctx)
		if err != nil {
			ctx.ServerError("LoadTeams", err)
			return
		}
	} else {
		teams, err = org.GetUserTeams(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("GetUserTeams", err)
			return
		}
	}

	ctx.Data["MentionableTeams"] = teams
	ctx.Data["MentionableTeamsOrg"] = ctx.Repo.Owner.Name
	ctx.Data["MentionableTeamsOrgAvatar"] = ctx.Repo.Owner.AvatarLink(ctx)
}

func RetrieveProjects(ctx *context.Context, repo *repo_model.Repository) {
	// Distinguish whether the owner of the repository
	// is an individual or an organization
	repoOwnerType := project_model.TypeIndividual
	if repo.Owner.IsOrganization() {
		repoOwnerType = project_model.TypeOrganization
	}
	var err error
	projects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptionsAll,
		RepoID:      repo.ID,
		IsClosed:    util.OptionalBoolFalse,
		Type:        project_model.TypeRepository,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}
	projects2, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptionsAll,
		OwnerID:     repo.OwnerID,
		IsClosed:    util.OptionalBoolFalse,
		Type:        repoOwnerType,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	ctx.Data["OpenProjects"] = append(projects, projects2...)

	projects, err = db.Find[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptionsAll,
		RepoID:      repo.ID,
		IsClosed:    util.OptionalBoolTrue,
		Type:        project_model.TypeRepository,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}
	projects2, err = db.Find[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptionsAll,
		OwnerID:     repo.OwnerID,
		IsClosed:    util.OptionalBoolTrue,
		Type:        repoOwnerType,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	ctx.Data["ClosedProjects"] = append(projects, projects2...)
}
