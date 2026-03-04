// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/shared/issue"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	project_service "code.gitea.io/gitea/services/projects"

	"xorm.io/builder"
)

const (
	tplProjects     templates.TplName = "org/projects/list"
	tplProjectsNew  templates.TplName = "org/projects/new"
	tplProjectsView templates.TplName = "org/projects/view"
)

// MustEnableProjects check if projects are enabled in settings
func MustEnableProjects(ctx *context.Context) {
	if unit.TypeProjects.UnitGlobalDisabled() {
		ctx.NotFound(nil)
		return
	}
}

// Projects renders the home page of projects
func Projects(ctx *context.Context) {
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("repo.projects")

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"
	keyword := ctx.FormTrim("q")
	page := max(ctx.FormInt("page"), 1)

	var projectType project_model.Type
	if ctx.ContextUser.IsOrganization() {
		projectType = project_model.TypeOrganization
	} else {
		projectType = project_model.TypeIndividual
	}
	projects, total, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.IssuePagingNum,
		},
		OwnerID:  ctx.ContextUser.ID,
		IsClosed: optional.Some(isShowClosed),
		OrderBy:  project_model.GetSearchOrderByBySortType(sortType),
		Type:     projectType,
		Title:    keyword,
	})
	if err != nil {
		ctx.ServerError("FindProjects", err)
		return
	}

	if err := project_service.LoadIssueNumbersForProjects(ctx, projects, ctx.Doer); err != nil {
		ctx.ServerError("LoadIssueNumbersForProjects", err)
		return
	}

	opTotal, err := db.Count[project_model.Project](ctx, project_model.SearchOptions{
		OwnerID:  ctx.ContextUser.ID,
		IsClosed: optional.Some(!isShowClosed),
		Type:     projectType,
	})
	if err != nil {
		ctx.ServerError("CountProjects", err)
		return
	}

	if isShowClosed {
		ctx.Data["OpenCount"] = opTotal
		ctx.Data["ClosedCount"] = total
	} else {
		ctx.Data["OpenCount"] = total
		ctx.Data["ClosedCount"] = opTotal
	}

	ctx.Data["Projects"] = projects

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}
	renderUtils := templates.NewRenderUtils(ctx)
	for _, project := range projects {
		project.RenderedContent = renderUtils.MarkdownToHtml(project.Description)
	}

	numPages := 0
	if total > 0 {
		numPages = (int(total) - 1/setting.UI.IssuePagingNum)
	}

	pager := context.NewPagination(int(total), setting.UI.IssuePagingNum, page, numPages)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["SortType"] = sortType

	ctx.HTML(http.StatusOK, tplProjects)
}

func canWriteProjects(ctx *context.Context) bool {
	if ctx.ContextUser.IsOrganization() {
		return ctx.Org.CanWriteUnit(ctx, unit.TypeProjects)
	}
	return ctx.Doer != nil && ctx.ContextUser.ID == ctx.Doer.ID
}

// RenderNewProject render creating a project page
func RenderNewProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	ctx.Data["TemplateConfigs"] = project_model.GetTemplateConfigs()
	ctx.Data["CardTypes"] = project_model.GetCardConfig()
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["HomeLink"] = ctx.ContextUser.HomeLink()
	ctx.Data["CancelLink"] = ctx.ContextUser.HomeLink() + "/-/projects"
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// NewProjectPost creates a new project
func NewProjectPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateProjectForm)
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	if ctx.HasError() {
		RenderNewProject(ctx)
		return
	}

	newProject := project_model.Project{
		OwnerID:      ctx.ContextUser.ID,
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: form.TemplateType,
		CardType:     form.CardType,
	}

	if ctx.ContextUser.IsOrganization() {
		newProject.Type = project_model.TypeOrganization
	} else {
		newProject.Type = project_model.TypeIndividual
	}

	if err := project_model.NewProject(ctx, &newProject); err != nil {
		ctx.ServerError("NewProject", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/projects")
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
		ctx.JSONRedirect(ctx.ContextUser.HomeLink() + "/-/projects")
		return
	}
	id := ctx.PathParamInt64("id")

	project, err := project_model.GetProjectByIDAndOwner(ctx, id, ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	if err := project_model.ChangeProjectStatusByRepoIDAndID(ctx, 0, project.ID, toClose); err != nil {
		ctx.NotFoundOrServerError("ChangeProjectStatusByRepoIDAndID", project_model.IsErrProjectNotExist, err)
		return
	}
	ctx.JSONRedirect(project_model.ProjectLinkForOrg(ctx.ContextUser, project.ID))
}

// DeleteProject delete a project
func DeleteProject(ctx *context.Context) {
	p, err := project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	if err := project_model.DeleteProjectByID(ctx, p.ID); err != nil {
		ctx.Flash.Error("DeleteProjectByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.projects.deletion_success"))
	}

	ctx.JSONRedirect(ctx.ContextUser.HomeLink() + "/-/projects")
}

// RenderEditProject allows a project to be edited
func RenderEditProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["CardTypes"] = project_model.GetCardConfig()

	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	p, err := project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	ctx.Data["projectID"] = p.ID
	ctx.Data["title"] = p.Title
	ctx.Data["content"] = p.Description
	ctx.Data["redirect"] = ctx.FormString("redirect")
	ctx.Data["HomeLink"] = ctx.ContextUser.HomeLink()
	ctx.Data["card_type"] = p.CardType
	ctx.Data["CancelLink"] = project_model.ProjectLinkForOrg(ctx.ContextUser, p.ID)

	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// EditProjectPost response for editing a project
func EditProjectPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateProjectForm)
	projectID := ctx.PathParamInt64("id")
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["CardTypes"] = project_model.GetCardConfig()
	ctx.Data["CancelLink"] = project_model.ProjectLinkForOrg(ctx.ContextUser, projectID)

	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplProjectsNew)
		return
	}

	p, err := project_model.GetProjectByIDAndOwner(ctx, projectID, ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
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
		ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/projects")
	}
}

// ViewProject renders the project with board view for a project
func ViewProject(ctx *context.Context) {
	project, err := project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	if err := project.LoadOwner(ctx); err != nil {
		ctx.ServerError("LoadOwner", err)
		return
	}

	columns, err := project.GetColumns(ctx)
	if err != nil {
		ctx.ServerError("GetProjectColumns", err)
		return
	}

	preparedLabelFilter := issue.PrepareFilterIssueLabels(ctx, project.RepoID, project.Owner)
	if ctx.Written() {
		return
	}
	assigneeID := ctx.FormString("assignee")
	milestoneID := ctx.FormInt64("milestone")

	// Prepare milestone IDs for filtering
	var milestoneIDs []int64
	if milestoneID > 0 {
		milestoneIDs = []int64{milestoneID}
	} else if milestoneID == db.NoConditionID {
		milestoneIDs = []int64{db.NoConditionID}
	}

	opts := issues_model.IssuesOptions{
		LabelIDs:     preparedLabelFilter.SelectedLabelIDs,
		AssigneeID:   assigneeID,
		MilestoneIDs: milestoneIDs,
		Owner:        project.Owner,
	}
	if ctx.Doer != nil {
		opts.Doer = ctx.Doer
	} else {
		opts.AllPublic = true
	}

	issuesMap, err := project_service.LoadIssuesFromProject(ctx, project, &opts)
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

	// TODO: Add option to filter also by repository specific labels
	labels, err := issues_model.GetLabelsByOrgID(ctx, project.OwnerID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByOrgID", err)
		return
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

	// Get milestones for filtering
	// For organization projects, we need to get milestones from all repos the user has access to
	var milestones issues_model.MilestoneList
	if project.RepoID > 0 {
		// Repo-specific project
		milestones, err = db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
			RepoID: project.RepoID,
		})
		if err != nil {
			ctx.ServerError("GetRepoMilestones", err)
			return
		}
	} else {
		// Organization-wide project - get milestones from all organization repos
		// but only from repositories the current user can access.
		// Use RepoCond with a subquery to avoid materializing all repo IDs in memory
		// which can hit SQL parameter limits for orgs with many repos.
		accessCond := repo_model.AccessibleRepositoryCondition(ctx.Doer, unit.TypeIssues)
		repoCond := builder.And(
			builder.Eq{"owner_id": project.OwnerID},
			accessCond,
		)
		milestones, err = db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
			RepoCond: repoCond,
		})
		if err != nil {
			ctx.ServerError("GetOrgMilestones", err)
			return
		}
	}

	openMilestones, closedMilestones := milestones.SplitByOpenClosed()
	ctx.Data["OpenMilestones"] = openMilestones
	ctx.Data["ClosedMilestones"] = closedMilestones
	ctx.Data["MilestoneID"] = milestoneID

	// Get assignees.
	assigneeUsers, err := org_model.GetOrgAssignees(ctx, project.OwnerID)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = shared_user.MakeSelfOnTop(ctx.Doer, assigneeUsers)
	ctx.Data["AssigneeID"] = assigneeID

	project.RenderedContent = templates.NewRenderUtils(ctx).MarkdownToHtml(project.Description)
	ctx.Data["LinkedPRs"] = linkedPrsMap
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["Project"] = project
	ctx.Data["IssuesMap"] = issuesMap
	ctx.Data["Columns"] = columns
	ctx.Data["Title"] = fmt.Sprintf("%s - %s", project.Title, ctx.ContextUser.DisplayName())

	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	ctx.HTML(http.StatusOK, tplProjectsView)
}

// DeleteProjectColumn allows for the deletion of a project column
func DeleteProjectColumn(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	_, err = project_model.GetColumnByIDAndProjectID(ctx, ctx.PathParamInt64("columnID"), project.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetColumnByIDAndProjectID", project_model.IsErrProjectColumnNotExist, err)
		return
	}

	if err := project_model.DeleteColumnByID(ctx, ctx.PathParamInt64("columnID")); err != nil {
		ctx.ServerError("DeleteProjectColumnByID", err)
		return
	}

	ctx.JSONOK()
}

// AddColumnToProjectPost allows a new column to be added to a project.
func AddColumnToProjectPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditProjectColumnForm)

	project, err := project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	if err := project_model.NewColumn(ctx, &project_model.Column{
		ProjectID: project.ID,
		Title:     form.Title,
		Color:     form.Color,
		CreatorID: ctx.Doer.ID,
	}); err != nil {
		ctx.ServerError("NewProjectColumn", err)
		return
	}

	ctx.JSONOK()
}

// CheckProjectColumnChangePermissions check permission
func CheckProjectColumnChangePermissions(ctx *context.Context) (*project_model.Project, *project_model.Column) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return nil, nil
	}

	project, err := project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return nil, nil
	}

	column, err := project_model.GetColumnByIDAndProjectID(ctx, ctx.PathParamInt64("columnID"), project.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetColumnByIDAndProjectID", project_model.IsErrProjectColumnNotExist, err)
		return nil, nil
	}

	return project, column
}

// EditProjectColumn allows a project column's to be updated
func EditProjectColumn(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditProjectColumnForm)
	_, column := CheckProjectColumnChangePermissions(ctx)
	if ctx.Written() {
		return
	}

	if form.Title != "" {
		column.Title = form.Title
	}
	column.Color = form.Color
	if form.Sorting != 0 {
		column.Sorting = form.Sorting
	}

	if err := project_model.UpdateColumn(ctx, column); err != nil {
		ctx.ServerError("UpdateProjectColumn", err)
		return
	}

	ctx.JSONOK()
}

// SetDefaultProjectColumn set default column for uncategorized issues/pulls
func SetDefaultProjectColumn(ctx *context.Context) {
	project, column := CheckProjectColumnChangePermissions(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.SetDefaultColumn(ctx, project.ID, column.ID); err != nil {
		ctx.ServerError("SetDefaultColumn", err)
		return
	}

	ctx.JSONOK()
}

// MoveIssues moves or keeps issues in a column and sorts them inside that column
func MoveIssues(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), ctx.ContextUser.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	column, err := project_model.GetColumnByIDAndProjectID(ctx, ctx.PathParamInt64("columnID"), project.ID)
	if err != nil {
		ctx.NotFoundOrServerError("GetColumnByIDAndProjectID", project_model.IsErrProjectColumnNotExist, err)
		return
	}

	type movedIssuesForm struct {
		Issues []struct {
			IssueID int64 `json:"issueID"`
			Sorting int64 `json:"sorting"`
		} `json:"issues"`
	}

	form := &movedIssuesForm{}
	if err = json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedIssuesForm", err)
		return
	}

	issueIDs := make([]int64, 0, len(form.Issues))
	sortedIssueIDs := make(map[int64]int64)
	for _, issue := range form.Issues {
		issueIDs = append(issueIDs, issue.IssueID)
		sortedIssueIDs[issue.Sorting] = issue.IssueID
	}
	movedIssues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByID", issues_model.IsErrIssueNotExist, err)
		return
	}

	if len(movedIssues) != len(form.Issues) {
		ctx.ServerError("some issues do not exist", errors.New("some issues do not exist"))
		return
	}

	if _, err = movedIssues.LoadRepositories(ctx); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}

	for _, issue := range movedIssues {
		if issue.RepoID != project.RepoID && issue.Repo.OwnerID != project.OwnerID {
			ctx.ServerError("Some issue's repoID is not equal to project's repoID", errors.New("Some issue's repoID is not equal to project's repoID"))
			return
		}
	}

	if err = project_service.MoveIssuesOnProjectColumn(ctx, ctx.Doer, column, sortedIssueIDs); err != nil {
		ctx.ServerError("MoveIssuesOnProjectColumn", err)
		return
	}

	ctx.JSONOK()
}
