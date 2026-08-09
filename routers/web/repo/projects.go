// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/web/shared/issue"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	project_service "gitea.dev/services/projects"
)

const (
	tplProjects     templates.TplName = "repo/projects/list"
	tplProjectsNew  templates.TplName = "repo/projects/new"
	tplProjectsView templates.TplName = "repo/projects/view"
)

// MustEnableRepoProjects check if repo projects are enabled in settings
func MustEnableRepoProjects(ctx *context.Context) {
	if unit.TypeProjects.UnitGlobalDisabled() {
		ctx.NotFound(nil)
		return
	}

	if ctx.Repo.Repository != nil {
		projectsUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeProjects)
		if !ctx.Repo.Permission.CanRead(unit.TypeProjects) || !projectsUnit.ProjectsConfig().IsProjectsAllowed(repo_model.ProjectsModeRepo) {
			ctx.NotFound(nil)
			return
		}
	}
}

// Projects renders the home page of projects
func Projects(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects")

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"
	keyword := ctx.FormTrim("q")
	repo := ctx.Repo.Repository
	page := max(ctx.FormInt("page"), 1)

	ctx.Data["OpenCount"] = repo.NumOpenProjects
	ctx.Data["ClosedCount"] = repo.NumClosedProjects

	projects, count, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.IssuePagingNum,
			Page:     page,
		},
		RepoID:   repo.ID,
		IsClosed: optional.Some(isShowClosed),
		OrderBy:  project_model.GetSearchOrderByBySortType(sortType),
		Type:     project_model.TypeRepository,
		Title:    keyword,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	if err := project_service.LoadIssueNumbersForProjects(ctx, projects, ctx.Doer); err != nil {
		ctx.ServerError("LoadIssueNumbersForProjects", err)
		return
	}

	for i := range projects {
		rctx := renderhelper.NewRenderContextRepoComment(ctx, repo)
		projects[i].RenderedContent, err = markdown.RenderString(rctx, projects[i].Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
	}

	ctx.Data["Projects"] = projects

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	pager := context.NewPagination(count, setting.UI.IssuePagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeProjects)
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["IsProjectsPage"] = true
	ctx.Data["SortType"] = sortType

	ctx.HTML(http.StatusOK, tplProjects)
}

// RenderNewProject render creating a project page
func RenderNewProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	ctx.Data["TemplateConfigs"] = project_model.GetTemplateConfigs()
	ctx.Data["CardTypes"] = project_model.GetCardConfig()
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeProjects)
	ctx.Data["CancelLink"] = ctx.Repo.Repository.Link() + "/projects"
	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// NewProjectPost creates a new project
func NewProjectPost(ctx *context.Context) {
	form := web.GetForm[*forms.CreateProjectForm](ctx)
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")

	if ctx.HasError() {
		RenderNewProject(ctx)
		return
	}

	if err := project_model.NewProject(ctx, &project_model.Project{
		RepoID:       ctx.Repo.Repository.ID,
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: form.TemplateType,
		CardType:     form.CardType,
		Type:         project_model.TypeRepository,
	}); err != nil {
		ctx.ServerError("NewProject", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/projects")
}

// ChangeProjectStatus updates the status of a project between "open" and "close"
func ChangeProjectStatus(ctx *context.Context) {
	var toClose bool
	switch ctx.PathParam("action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/projects")
		return
	}
	id := ctx.PathParamInt64("id")

	if err := project_model.ChangeProjectStatusByRepoIDAndID(ctx, ctx.Repo.Repository.ID, id, toClose); err != nil {
		ctx.NotFoundOrServerError("ChangeProjectStatusByRepoIDAndID", project_model.IsErrProjectNotExist, err)
		return
	}
	ctx.JSONRedirect(project_model.ProjectLinkForRepo(ctx.Repo.Repository, id))
}

// DeleteProject delete a project
func DeleteProject(ctx *context.Context) {
	p, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}

	if err := project_model.DeleteProjectByID(ctx, p.ID); err != nil {
		ctx.Flash.Error("DeleteProjectByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.projects.deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/projects")
}

// RenderEditProject allows a project to be edited
func RenderEditProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeProjects)
	ctx.Data["CardTypes"] = project_model.GetCardConfig()

	p, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["projectID"] = p.ID
	ctx.Data["title"] = p.Title
	ctx.Data["content"] = p.Description
	ctx.Data["card_type"] = p.CardType
	ctx.Data["redirect"] = ctx.FormString("redirect")
	ctx.Data["CancelLink"] = project_model.ProjectLinkForRepo(ctx.Repo.Repository, p.ID)

	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// EditProjectPost response for editing a project
func EditProjectPost(ctx *context.Context) {
	form := web.GetForm[*forms.CreateProjectForm](ctx)
	projectID := ctx.PathParamInt64("id")

	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeProjects)
	ctx.Data["CardTypes"] = project_model.GetCardConfig()
	ctx.Data["CancelLink"] = project_model.ProjectLinkForRepo(ctx.Repo.Repository, projectID)

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplProjectsNew)
		return
	}

	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}

	p.Title = form.Title
	p.Description = form.Content
	p.CardType = form.CardType
	if err = project_model.UpdateProject(ctx, p); err != nil {
		ctx.ServerError("UpdateProjects", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.edit_success", p.Title))
	if ctx.FormString("redirect") == "project" {
		ctx.Redirect(p.Link(ctx))
	} else {
		ctx.Redirect(ctx.Repo.RepoLink + "/projects")
	}
}

// ViewProject renders the project with board view
func ViewProject(ctx *context.Context) {
	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}

	columns, err := project_model.GetColumns(ctx, project.ID, db.ListOptionsAll)
	if err != nil {
		ctx.ServerError("GetColumns", err)
		return
	}

	preparedLabelFilter := issue.PrepareFilterIssueLabels(ctx, ctx.Repo.Repository.ID, ctx.Repo.Owner)
	if ctx.Written() {
		return
	}

	assigneeID := ctx.FormString("assignee")
	milestoneID := ctx.FormInt64("milestone")

	var milestoneIDs []int64
	if milestoneID > 0 {
		milestoneIDs = []int64{milestoneID}
	} else if milestoneID == db.NoConditionID {
		milestoneIDs = []int64{db.NoConditionID}
	}

	issuesMap, err := project_service.LoadIssuesFromProject(ctx, project, &issues_model.IssuesOptions{
		RepoIDs:      []int64{ctx.Repo.Repository.ID},
		LabelIDs:     preparedLabelFilter.SelectedLabelIDs,
		AssigneeID:   assigneeID,
		MilestoneIDs: milestoneIDs,
	})
	if err != nil {
		ctx.ServerError("LoadIssuesOfColumns", err)
		return
	}
	for _, column := range columns {
		column.NumIssues = int64(len(issuesMap[column.ID]))
	}

	if project.CardType != project_model.CardTypeTextOnly {
		issuesAttachmentMap := make(map[int64][]*repo_model.Attachment)
		for _, issuesList := range issuesMap {
			for _, issue := range issuesList {
				if issueAttachment, err := repo_model.GetAttachmentsByIssueIDImagesLatest(ctx, issue.ID); err == nil {
					issuesAttachmentMap[issue.ID] = issueAttachment
				}
			}
		}
		ctx.Data["issuesAttachmentMap"] = issuesAttachmentMap
	}

	linkedPrsMap := make(map[int64][]*issues_model.Issue)
	for _, issuesList := range issuesMap {
		for _, issue := range issuesList {
			var referencedIDs []int64
			for _, comment := range issue.Comments {
				if comment.RefIssueID != 0 && comment.RefIsPull {
					referencedIDs = append(referencedIDs, comment.RefIssueID)
				}
			}

			if len(referencedIDs) > 0 {
				if linkedPrs, err := issues_model.Issues(ctx, &issues_model.IssuesOptions{
					IssueIDs: referencedIDs,
					IsPull:   optional.Some(true),
				}); err == nil {
					linkedPrsMap[issue.ID] = linkedPrs
				}
			}
		}
	}
	ctx.Data["LinkedPRs"] = linkedPrsMap

	labels, err := issues_model.GetLabelsByRepoID(ctx, project.RepoID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}

	if ctx.Repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, ctx.Repo.Owner.ID, "", db.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return
		}

		labels = append(labels, orgLabels...)
	}

	// Get the exclusive scope for every label ID
	labelExclusiveScopes := make([]string, 0, len(preparedLabelFilter.SelectedLabelIDs))
	for _, labelID := range preparedLabelFilter.SelectedLabelIDs {
		foundExclusiveScope := false
		for _, label := range labels {
			if label.ID == labelID || label.ID == -labelID {
				labelExclusiveScopes = append(labelExclusiveScopes, label.ExclusiveScope())
				foundExclusiveScope = true
				break
			}
		}
		if !foundExclusiveScope {
			labelExclusiveScopes = append(labelExclusiveScopes, "")
		}
	}

	for _, l := range labels {
		l.LoadSelectedLabelsAfterClick(preparedLabelFilter.SelectedLabelIDs, labelExclusiveScopes)
	}
	ctx.Data["Labels"] = labels
	ctx.Data["NumLabels"] = len(labels)

	// Get assignees.
	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = shared_user.MakeSelfOnTop(ctx.Doer, assigneeUsers)
	ctx.Data["AssigneeID"] = assigneeID

	renderMilestones(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["MilestoneID"] = milestoneID

	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository)
	project.RenderedContent, err = markdown.RenderString(rctx, project.Description)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.Data["Title"] = project.Title
	ctx.Data["IsProjectsPage"] = true
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeProjects)
	ctx.Data["Project"] = project
	ctx.Data["IssuesMap"] = issuesMap
	ctx.Data["Columns"] = columns

	ctx.HTML(http.StatusOK, tplProjectsView)
}

// UpdateIssueProject change an issue's project
func UpdateIssueProject(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	if err := issues.LoadProjects(ctx); err != nil {
		ctx.ServerError("LoadProjects", err)
		return
	}
	if _, err := issues.LoadRepositories(ctx); err != nil {
		ctx.ServerError("LoadProjects", err)
		return
	}

	projectIDs := ctx.FormStringInt64s("id")
	var failedIssues []int64
	for _, issue := range issues {
		if err := issues_model.IssueAssignOrRemoveProject(ctx, issue, ctx.Doer, projectIDs); err != nil {
			if errors.Is(err, util.ErrPermissionDenied) {
				failedIssues = append(failedIssues, issue.ID)
				continue
			}
			ctx.ServerError("IssueAssignOrRemoveProject", err)
			return
		}
	}

	if len(failedIssues) > 0 {
		log.Warn("Failed to assign projects to %d issues due to permission denied: %v", len(failedIssues), failedIssues)
	}

	ctx.JSONOK()
}

// UpdateIssueProjectColumn moves an issue to a different column within its project
func UpdateIssueProjectColumn(ctx *context.Context) {
	issue, err := issues_model.GetIssueByRepoID(ctx, ctx.Repo.Repository.ID, ctx.FormInt64("issue_id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByID", issues_model.IsErrIssueNotExist, err)
		return
	}
	column, err := project_model.GetColumn(ctx, ctx.FormInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetColumn", project_model.IsErrProjectColumnNotExist, err)
		return
	}

	if err := issue.LoadProjects(ctx); err != nil {
		ctx.ServerError("LoadProjects", err)
		return
	}

	// it must make sure the requested column is in this issue's projects
	if !slices.ContainsFunc(issue.Projects, func(p *project_model.Project) bool { return p.ID == column.ProjectID }) {
		ctx.NotFound(nil)
		return
	}

	if err := project_service.MoveIssueToColumn(ctx, ctx.Doer, issue, column, optional.None[int64]()); err != nil {
		ctx.ServerError("MoveIssueToColumn", err)
		return
	}

	ctx.JSONOK()
}
