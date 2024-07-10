package org

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	attachment_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

// CreateProject creates a new project
func CreateProject(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.CreateProjectOption)

	project := &project_model.Project{
		OwnerID:      ctx.ContextUser.ID,
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.TemplateType(form.TemplateType),
		CardType:     project_model.CardType(form.CardType),
	}

	if ctx.ContextUser.IsOrganization() {
		project.Type = project_model.TypeOrganization
	} else {
		project.Type = project_model.TypeIndividual
	}

	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewProject", err)
		return
	}

	ctx.JSON(http.StatusCreated, project)
}

// ChangeProjectStatus updates the status of a project between "open" and "close"
func ChangeProjectStatus(ctx *context.APIContext) {
	var toClose bool
	switch ctx.PathParam(":action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.NotFound("ChangeProjectStatus", nil)
		return
	}
	id := ctx.PathParamInt64(":id")

	if err := project_model.ChangeProjectStatusByRepoIDAndID(ctx, 0, id, toClose); err != nil {
		ctx.NotFoundOrServerError("ChangeProjectStatusByRepoIDAndID", project_model.IsErrProjectNotExist, err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"message": "project status updated successfully"})
}

// Projects renders the home page of projects
func GetProjects(ctx *context.APIContext) {
	ctx.Data["Title"] = ctx.Tr("repo.projects")

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"
	keyword := ctx.FormTrim("q")
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	var projectType project_model.Type
	if ctx.ContextUser.IsOrganization() {
		projectType = project_model.TypeOrganization
	} else {
		projectType = project_model.TypeIndividual
	}
	projects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
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

	ctx.JSON(http.StatusOK, projects)
}

// TODO: Send issues as well
// GetProject returns a project by ID
func GetProject(ctx *context.APIContext) {
	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if project.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("", nil)
		return
	}

	columns, err := project.GetColumns(ctx)
	if err != nil {
		ctx.ServerError("GetProjectColumns", err)
		return
	}

	issuesMap, err := issues_model.LoadIssuesFromColumnList(ctx, columns)
	if err != nil {
		ctx.ServerError("LoadIssuesOfColumns", err)
		return
	}

	if project.CardType != project_model.CardTypeTextOnly {
		issuesAttachmentMap := make(map[int64][]*attachment_model.Attachment)
		for _, issuesList := range issuesMap {
			for _, issue := range issuesList {
				if issueAttachment, err := attachment_model.GetAttachmentsByIssueIDImagesLatest(ctx, issue.ID); err == nil {
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

	issues := make(map[int64][]*issues_model.Issue)

	for _, column := range columns {
		if empty := issuesMap[column.ID]; len(empty) == 0 {
			continue
		}
		issues[column.ID] = issuesMap[column.ID]

	}

	data := map[string]any{
		"project": project,
		"columns": columns,
	}

	ctx.JSON(http.StatusOK, data)
}

// AddColumnToProject adds a new column to a project
func AddColumnToProject(ctx *context.APIContext) {
	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if project.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("", nil)
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectColumnOption)
	column := &project_model.Column{
		ProjectID: project.ID,
		Title:     form.Title,
		Sorting:   form.Sorting,
		Color:     form.Color,
	}
	if err := project_model.NewColumn(ctx, column); err != nil {
		ctx.ServerError("NewProjectColumn", err)
		return
	}

	ctx.JSON(http.StatusCreated, column)
}

// DeleteProject delete a project
func DeleteProject(ctx *context.APIContext) {
	p, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("", nil)
		return
	}

	err = project_model.DeleteProjectByID(ctx, p.ID)

	if err != nil {
		ctx.ServerError("DeleteProjectByID", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{"message": "project deleted successfully"})
}

// EditProject updates a project
func EditProject(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.CreateProjectOption)
	projectID := ctx.PathParamInt64(":id")

	ctx.Data["CancelLink"] = fmt.Sprintf("%s/-/projects/%d", ctx.ContextUser.HomeLink(), projectID)

	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("", nil)
		return
	}

	p.Title = form.Title
	p.Description = form.Content
	p.CardType = project_model.CardType(form.CardType)

	if err = project_model.UpdateProject(ctx, p); err != nil {
		ctx.ServerError("UpdateProjects", err)
		return
	}

	ctx.JSON(http.StatusOK, p)
}

// MoveColumns moves or keeps columns in a project and sorts them inside that project
func MoveColumns(ctx *context.APIContext) {
	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if !project.CanBeAccessedByOwnerRepo(ctx.ContextUser.ID, ctx.Repo.Repository) {
		ctx.NotFound("CanBeAccessedByOwnerRepo", nil)
		return
	}

	type movedColumnsForm struct {
		Columns []struct {
			ColumnID int64 `json:"columnID"`
			Sorting  int64 `json:"sorting"`
		} `json:"columns"`
	}

	form := &movedColumnsForm{}
	if err = json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedColumnsForm", err)
		return
	}

	sortedColumnIDs := make(map[int64]int64)
	for _, column := range form.Columns {
		sortedColumnIDs[column.Sorting] = column.ColumnID
	}

	if err = project_model.MoveColumnsOnProject(ctx, project, sortedColumnIDs); err != nil {
		ctx.ServerError("MoveColumnsOnProject", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"message": "columns moved successfully"})
}

// CheckProjectColumnChangePermissions check permission
func CheckProjectColumnChangePermissions(ctx *context.APIContext) (*project_model.Project, *project_model.Column) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return nil, nil
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
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

	if project.OwnerID != ctx.ContextUser.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectColumn[%d] is not in Repository[%d] as expected", column.ID, project.ID),
		})
		return nil, nil
	}
	return project, column
}

// EditProjectColumn allows a project column's to be updated
func EditProjectColumn(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.EditProjectColumnOption)
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

	ctx.JSON(http.StatusOK, column)
}

// DeleteProjectColumn allows for the deletion of a project column
func DeleteProjectColumn(ctx *context.APIContext) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
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

	if project.OwnerID != ctx.ContextUser.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectColumn[%d] is not in Owner[%d] as expected", pb.ID, ctx.ContextUser.ID),
		})
		return
	}

	if err := project_model.DeleteColumnByID(ctx, ctx.PathParamInt64(":columnID")); err != nil {
		ctx.ServerError("DeleteProjectColumnByID", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"message": "column deleted successfully"})
}

// SetDefaultProjectColumn set default column for uncategorized issues/pulls
func SetDefaultProjectColumn(ctx *context.APIContext) {
	project, column := CheckProjectColumnChangePermissions(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.SetDefaultColumn(ctx, project.ID, column.ID); err != nil {
		ctx.ServerError("SetDefaultColumn", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"message": "default column set successfully"})
}

// MoveIssues moves or keeps issues in a column and sorts them inside that column
func MoveIssues(ctx *context.APIContext) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if project.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("InvalidRepoID", nil)
		return
	}

	column, err := project_model.GetColumn(ctx, ctx.PathParamInt64(":columnID"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectColumn", project_model.IsErrProjectColumnNotExist, err)
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

	if err = project_model.MoveIssuesOnProjectColumn(ctx, column, sortedIssueIDs); err != nil {
		ctx.ServerError("MoveIssuesOnProjectColumn", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"message": "issues moved successfully"})
}
