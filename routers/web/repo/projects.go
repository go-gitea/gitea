// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/shared/issue"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	project_service "code.gitea.io/gitea/services/projects"
)

const (
	tplProjects     base.TplName = "repo/projects/list"
	tplProjectsNew  base.TplName = "repo/projects/new"
	tplProjectsView base.TplName = "repo/projects/view"
)

// MustEnableRepoProjects check if repo projects are enabled in settings
func MustEnableRepoProjects(ctx *context.Context) {
	if unit.TypeProjects.UnitGlobalDisabled() {
		ctx.NotFound("EnableRepoProjects", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		projectsUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeProjects)
		if !ctx.Repo.CanRead(unit.TypeProjects) || !projectsUnit.ProjectsConfig().IsProjectsAllowed(repo_model.ProjectsModeRepo) {
			ctx.NotFound("MustEnableRepoProjects", nil)
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
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	ctx.Data["OpenCount"] = repo.NumOpenProjects
	ctx.Data["ClosedCount"] = repo.NumClosedProjects

	var total int
	if !isShowClosed {
		total = repo.NumOpenProjects
	} else {
		total = repo.NumClosedProjects
	}

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

	numPages := 0
	if count > 0 {
		numPages = (int(count) - 1/setting.UI.IssuePagingNum)
	}

	pager := context.NewPagination(total, setting.UI.IssuePagingNum, page, numPages)
	pager.AddParamString("state", fmt.Sprint(ctx.Data["State"]))
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
	form := web.GetForm(ctx).(*forms.CreateProjectForm)
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
	switch ctx.PathParam(":action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/projects")
		return
	}
	id := ctx.PathParamInt64(":id")

	if err := project_model.ChangeProjectStatusByRepoIDAndID(ctx, ctx.Repo.Repository.ID, id, toClose); err != nil {
		ctx.NotFoundOrServerError("ChangeProjectStatusByRepoIDAndID", project_model.IsErrProjectNotExist, err)
		return
	}
	ctx.JSONRedirect(project_model.ProjectLinkForRepo(ctx.Repo.Repository, id))
}

// DeleteProject delete a project
func DeleteProject(ctx *context.Context) {
	p, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
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

	p, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
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
	form := web.GetForm(ctx).(*forms.CreateProjectForm)
	projectID := ctx.PathParamInt64(":id")

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
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
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
	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
		return
	}

	columns, err := project.GetColumns(ctx)
	if err != nil {
		ctx.ServerError("GetProjectColumns", err)
		return
	}

	labelIDs := issue.PrepareFilterIssueLabels(ctx, ctx.Repo.Repository.ID, ctx.Repo.Owner)

	assigneeID := ctx.FormInt64("assignee") // TODO: use "optional" but not 0 in the future

	issuesMap, err := project_service.LoadIssuesFromProject(ctx, project, &issues_model.IssuesOptions{
		RepoIDs:    []int64{ctx.Repo.Repository.ID},
		LabelIDs:   labelIDs,
		AssigneeID: optional.Some(assigneeID),
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
	labelExclusiveScopes := make([]string, 0, len(labelIDs))
	for _, labelID := range labelIDs {
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
		l.LoadSelectedLabelsAfterClick(labelIDs, labelExclusiveScopes)
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

	projectID := ctx.FormInt64("id")
	for _, issue := range issues {
		if issue.Project != nil && issue.Project.ID == projectID {
			continue
		}
		if err := issues_model.IssueAssignOrRemoveProject(ctx, issue, ctx.Doer, projectID, 0); err != nil {
			if errors.Is(err, util.ErrPermissionDenied) {
				continue
			}
			ctx.ServerError("IssueAssignOrRemoveProject", err)
			return
		}
	}

	ctx.JSONOK()
}

// DeleteProjectColumn allows for the deletion of a project column
func DeleteProjectColumn(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}

	pb, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":columnID"))
	if err != nil {
		ctx.ServerError("GetProjectColumn", err)
		return
	}
	if pb.ProjectID != ctx.PathParamInt64(":id") {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectColumn[%d] is not in Project[%d] as expected", pb.ID, project.ID),
		})
		return
	}

	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectColumn[%d] is not in Repository[%d] as expected", pb.ID, ctx.Repo.Repository.ID),
		})
		return
	}

	if err := project_model.DeleteColumnByID(ctx, ctx.PathParamInt64(":columnID")); err != nil {
		ctx.ServerError("DeleteProjectColumnByID", err)
		return
	}

	ctx.JSONOK()
}

// AddColumnToProjectPost allows a new column to be added to a project.
func AddColumnToProjectPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditProjectColumnForm)
	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
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

func checkProjectColumnChangePermissions(ctx *context.Context) (*project_model.Project, *project_model.Column) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return nil, nil
	}

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return nil, nil
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return nil, nil
	}

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":columnID"))
	if err != nil {
		ctx.ServerError("GetProjectColumn", err)
		return nil, nil
	}
	if column.ProjectID != ctx.PathParamInt64(":id") {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectColumn[%d] is not in Project[%d] as expected", column.ID, project.ID),
		})
		return nil, nil
	}

	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectColumn[%d] is not in Repository[%d] as expected", column.ID, ctx.Repo.Repository.ID),
		})
		return nil, nil
	}
	return project, column
}

// EditProjectColumn allows a project column's to be updated
func EditProjectColumn(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditProjectColumnForm)
	_, column := checkProjectColumnChangePermissions(ctx)
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
	project, column := checkProjectColumnChangePermissions(ctx)
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

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("ProjectNotExist", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("InvalidRepoID", nil)
		return
	}

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":columnID"))
	if err != nil {
		if project_model.IsErrProjectColumnNotExist(err) {
			ctx.NotFound("ProjectColumnNotExist", nil)
		} else {
			ctx.ServerError("GetProjectColumn", err)
		}
		return
	}

	if column.ProjectID != project.ID {
		ctx.NotFound("ColumnNotInProject", nil)
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
	}

	issueIDs := make([]int64, 0, len(form.Issues))
	sortedIssueIDs := make(map[int64]int64)
	for _, issue := range form.Issues {
		issueIDs = append(issueIDs, issue.IssueID)
		sortedIssueIDs[issue.Sorting] = issue.IssueID
	}
	movedIssues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("IssueNotExisting", nil)
		} else {
			ctx.ServerError("GetIssueByID", err)
		}
		return
	}

	if len(movedIssues) != len(form.Issues) {
		ctx.ServerError("some issues do not exist", errors.New("some issues do not exist"))
		return
	}

	for _, issue := range movedIssues {
		if issue.RepoID != project.RepoID {
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
