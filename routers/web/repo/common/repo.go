// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/forms"
)

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
		isAdmin, err = org.IsOwnedBy(ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("IsOwnedBy", err)
			return
		}
	}

	if isAdmin {
		teams, err = org.LoadTeams()
		if err != nil {
			ctx.ServerError("LoadTeams", err)
			return
		}
	} else {
		teams, err = org.GetUserTeams(ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("GetUserTeams", err)
			return
		}
	}

	ctx.Data["MentionableTeams"] = teams
	ctx.Data["MentionableTeamsOrg"] = ctx.Repo.Owner.Name
	ctx.Data["MentionableTeamsOrgAvatar"] = ctx.Repo.Owner.AvatarLink()
}

// RetrieveRepoMilestonesAndAssignees find all the milestones and assignees of a repository
func RetrieveRepoMilestonesAndAssignees(ctx *context.Context, repo *repo_model.Repository) {
	var err error
	ctx.Data["OpenMilestones"], _, err = issues_model.GetMilestones(issues_model.GetMilestonesOption{
		RepoID: repo.ID,
		State:  api.StateOpen,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
	ctx.Data["ClosedMilestones"], _, err = issues_model.GetMilestones(issues_model.GetMilestonesOption{
		RepoID: repo.ID,
		State:  api.StateClosed,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}

	ctx.Data["Assignees"], err = repo_model.GetRepoAssignees(ctx, repo)
	if err != nil {
		ctx.ServerError("GetAssignees", err)
		return
	}

	HandleTeamMentions(ctx)
}

// RetrieveProjects get projects open/close informations
func RetrieveProjects(ctx *context.Context, repo *repo_model.Repository) {
	var err error

	ctx.Data["OpenProjects"], _, err = project_model.GetProjects(ctx, project_model.SearchOptions{
		RepoID:   repo.ID,
		Page:     -1,
		IsClosed: util.OptionalBoolFalse,
		Type:     project_model.TypeRepository,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	ctx.Data["ClosedProjects"], _, err = project_model.GetProjects(ctx, project_model.SearchOptions{
		RepoID:   repo.ID,
		Page:     -1,
		IsClosed: util.OptionalBoolTrue,
		Type:     project_model.TypeRepository,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}
}

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

	brs, _, err := ctx.Repo.GitRepo.GetBranchNames(0, 0)
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return nil
	}
	ctx.Data["Branches"] = brs

	// Contains true if the user can create issue dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx.Doer, isPull)

	return labels
}

// ValidateRepoMetas check and returns repository's meta information
func ValidateRepoMetas(ctx *context.Context, form forms.CreateIssueForm, isPull bool) ([]int64, []int64, int64, int64) {
	var (
		repo = ctx.Repo.Repository
		err  error
	)

	labels := RetrieveRepoMetas(ctx, ctx.Repo.Repository, isPull)
	if ctx.Written() {
		return nil, nil, 0, 0
	}

	var labelIDs []int64
	hasSelected := false
	// Check labels.
	if len(form.LabelIDs) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
		if err != nil {
			return nil, nil, 0, 0
		}
		labelIDMark := make(container.Set[int64])
		labelIDMark.AddMultiple(labelIDs...)

		for i := range labels {
			if labelIDMark.Contains(labels[i].ID) {
				labels[i].IsChecked = true
				hasSelected = true
			}
		}
	}

	ctx.Data["Labels"] = labels
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["label_ids"] = form.LabelIDs

	// Check milestone.
	milestoneID := form.MilestoneID
	if milestoneID > 0 {
		milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
		if err != nil {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0, 0
		}
		if milestone.RepoID != repo.ID {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0, 0
		}
		ctx.Data["Milestone"] = milestone
		ctx.Data["milestone_id"] = milestoneID
	}

	if form.ProjectID > 0 {
		p, err := project_model.GetProjectByID(ctx, form.ProjectID)
		if err != nil {
			ctx.ServerError("GetProjectByID", err)
			return nil, nil, 0, 0
		}
		if p.RepoID != ctx.Repo.Repository.ID {
			ctx.NotFound("", nil)
			return nil, nil, 0, 0
		}

		ctx.Data["Project"] = p
		ctx.Data["project_id"] = form.ProjectID
	}

	// Check assignees
	var assigneeIDs []int64
	if len(form.AssigneeIDs) > 0 {
		assigneeIDs, err = base.StringsToInt64s(strings.Split(form.AssigneeIDs, ","))
		if err != nil {
			return nil, nil, 0, 0
		}

		// Check if the passed assignees actually exists and is assignable
		for _, aID := range assigneeIDs {
			assignee, err := user_model.GetUserByID(aID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return nil, nil, 0, 0
			}

			valid, err := access_model.CanBeAssigned(ctx, assignee, repo, isPull)
			if err != nil {
				ctx.ServerError("CanBeAssigned", err)
				return nil, nil, 0, 0
			}

			if !valid {
				ctx.ServerError("canBeAssigned", repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: repo.Name})
				return nil, nil, 0, 0
			}
		}
	}

	// Keep the old assignee id thingy for compatibility reasons
	if form.AssigneeID > 0 {
		assigneeIDs = append(assigneeIDs, form.AssigneeID)
	}

	return labelIDs, assigneeIDs, milestoneID, form.ProjectID
}
