// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

var errInvalidModelType = errors.New("invalid model type")

func checkModelType(model string) error {
	if model != "repo" && model != "org" {
		return errInvalidModelType
	}
	return nil
}

// ProjectHandler is a handler for project actions
func ProjectHandler(model string, fn func(ctx *context.APIContext, model string)) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		fn(ctx, model)
	}
}

// CreateProject creates a new project
func CreateProject(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateProject", err)
		return
	}

	form := web.GetForm(ctx).(*api.CreateProjectOption)

	project := &project_model.Project{
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.TemplateType(form.TemplateType),
		CardType:     project_model.CardType(form.CardType),
	}

	if model == "repo" {
		project.Type = project_model.TypeRepository
		project.RepoID = ctx.Repo.Repository.ID
	} else {
		if ctx.ContextUser.IsOrganization() {
			project.Type = project_model.TypeOrganization
		} else {
			project.Type = project_model.TypeIndividual
		}
		project.OwnerID = ctx.ContextUser.ID
	}

	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewProject", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToProject(ctx, project))
}

// GetProjects returns a list of projects
func GetProjects(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProjects", err)
		return
	}

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"

	searchOptions := project_model.SearchOptions{
		IsClosed: optional.Some(isShowClosed),
		OrderBy:  project_model.GetSearchOrderByBySortType(sortType),
	}

	if model == "repo" {
		repo := ctx.Repo.Repository
		searchOptions.RepoID = repo.ID
		searchOptions.Type = project_model.TypeRepository
	} else {
		searchOptions.OwnerID = ctx.ContextUser.ID

		if ctx.ContextUser.IsOrganization() {
			searchOptions.Type = project_model.TypeOrganization
		} else {
			searchOptions.Type = project_model.TypeIndividual
		}
	}

	projects, err := db.Find[project_model.Project](ctx, &searchOptions)

	if err != nil {
		ctx.ServerError("FindProjects", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProjects(ctx, projects))
}

// GetProject returns a project
func GetProject(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProject", err)
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
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

	issues := issues_model.IssueList{}

	for _, column := range columns {
		if empty := issuesMap[column.ID]; len(empty) == 0 {
			continue
		}
		issues = append(issues, issuesMap[column.ID]...)
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"project": convert.ToProject(ctx, project),
		"columns": convert.ToColumns(ctx, columns),
		"issues":  convert.ToAPIIssueList(ctx, ctx.Doer, issues),
	})
}

// EditProject edits a project
func EditProject(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "EditProject", err)
		return
	}

	form := web.GetForm(ctx).(*api.CreateProjectOption)
	projectID := ctx.PathParamInt64(":id")

	project, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	project.Title = form.Title
	project.Description = form.Content
	project.CardType = project_model.CardType(form.CardType)

	if err = project_model.UpdateProject(ctx, project); err != nil {
		ctx.ServerError("UpdateProjects", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToProject(ctx, project))
}

// DeleteProject deletes a project
func DeleteProject(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteProject", err)
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}

	err = project_model.DeleteProjectByID(ctx, project.ID)

	if err != nil {
		ctx.ServerError("DeleteProjectByID", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{"message": "project deleted successfully"})
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

func AddColumnToProject(ctx *context.APIContext, model string) {
	var err error
	err = checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "AddColumnToProject", err)
		return
	}

	if model == "repo" {
		if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
			ctx.JSON(http.StatusForbidden, map[string]string{
				"message": "Only authorized users are allowed to perform this action.",
			})
			return
		}
	}

	var project *project_model.Project
	if model == "repo" {
		project, err = project_model.GetProjectForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":id"))
	} else {
		project, err = project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	}

	if err != nil {
		ctx.NotFoundOrServerError("GetProjectForRepoByID", project_model.IsErrProjectNotExist, err)
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectColumnOption)
	column := &project_model.Column{
		ProjectID: project.ID,
		Title:     form.Title,
		Sorting:   form.Sorting,
		Color:     form.Color,
		CreatorID: ctx.Doer.ID,
	}
	if err := project_model.NewColumn(ctx, column); err != nil {
		ctx.ServerError("NewProjectColumn", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToColumn(ctx, column))
}

// checkProjectColumnChangePermissions check permission
func checkProjectColumnChangePermissions(ctx *context.APIContext, model string) (*project_model.Project, *project_model.Column) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return nil, nil
	}

	if model == "repo" {
		if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
			ctx.JSON(http.StatusForbidden, map[string]string{
				"message": "Only authorized users are allowed to perform this action.",
			})
			return nil, nil
		}
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

	if model == "repo" {
		if project.RepoID != ctx.Repo.Repository.ID {
			ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
				"message": fmt.Sprintf("ProjectColumn[%d] is not in Repository[%d] as expected", column.ID, project.ID),
			})
			return nil, nil
		}
	} else {
		if project.OwnerID != ctx.ContextUser.ID {
			ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
				"message": fmt.Sprintf("ProjectColumn[%d] is not in Repository[%d] as expected", column.ID, project.ID),
			})
			return nil, nil
		}
	}
	return project, column
}

// EditProjectColumn allows a project column's to be updated
func EditProjectColumn(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "EditProjectColumn", err)
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectColumnOption)
	_, column := checkProjectColumnChangePermissions(ctx, model)
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

	ctx.JSON(http.StatusOK, convert.ToColumn(ctx, column))
}

// DeleteProjectColumn allows for the deletion of a project column
func DeleteProjectColumn(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteProjectColumn", err)
		return
	}

	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	if model == "repo" {
		if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
			ctx.JSON(http.StatusForbidden, map[string]string{
				"message": "Only authorized users are allowed to perform this action.",
			})
			return
		}
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

	if model == "repo" {
		if project.RepoID != ctx.Repo.Repository.ID {
			ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
				"message": fmt.Sprintf("ProjectColumn[%d] is not in Owner[%d] as expected", pb.ID, ctx.ContextUser.ID),
			})
			return
		}
	} else {
		if project.OwnerID != ctx.ContextUser.ID {
			ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
				"message": fmt.Sprintf("ProjectColumn[%d] is not in Owner[%d] as expected", pb.ID, ctx.ContextUser.ID),
			})
			return
		}
	}

	if err := project_model.DeleteColumnByID(ctx, ctx.PathParamInt64(":columnID")); err != nil {
		ctx.ServerError("DeleteProjectColumnByID", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"message": "column deleted successfully"})
}

// SetDefaultProjectColumn set default column for uncategorized issues/pulls
func SetDefaultProjectColumn(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SetDefaultProjectColumn", err)
		return
	}

	project, column := checkProjectColumnChangePermissions(ctx, model)
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
func MoveIssues(ctx *context.APIContext, model string) {
	err := checkModelType(model)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "MoveIssues", err)
		return
	}

	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	if model == "repo" {
		if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeProjects) {
			ctx.JSON(http.StatusForbidden, map[string]string{
				"message": "Only authorized users are allowed to perform this action.",
			})
			return
		}
	}

	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
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

func getActionIssues(ctx *context.APIContext, issuesIDs []int64) issues_model.IssueList {

	if len(issuesIDs) == 0 {
		return nil
	}

	issues, err := issues_model.GetIssuesByIDs(ctx, issuesIDs)
	if err != nil {
		ctx.ServerError("GetIssuesByIDs", err)
		return nil
	}

	issueUnitEnabled := ctx.Repo.CanRead(unit.TypeIssues)
	prUnitEnabled := ctx.Repo.CanRead(unit.TypePullRequests)
	for _, issue := range issues {
		if issue.RepoID != ctx.Repo.Repository.ID {
			ctx.NotFound("some issue's RepoID is incorrect", errors.New("some issue's RepoID is incorrect"))
			return nil
		}
		if issue.IsPull && !prUnitEnabled || !issue.IsPull && !issueUnitEnabled {
			ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
			return nil
		}
		if err = issue.LoadAttributes(ctx); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return nil
		}
	}
	return issues
}

// UpdateIssueProject change an issue's project
func UpdateIssueProject(ctx *context.APIContext) {
	type updateIssuesForm struct {
		ProjectID int64   `json:"project_id"`
		Issues    []int64 `json:"issues"`
	}

	form := &updateIssuesForm{}

	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedIssuesForm", err)
		return
	}

	issues := getActionIssues(ctx, form.Issues)
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

	projectID := form.ProjectID
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

	ctx.JSON(http.StatusOK, map[string]string{"message": "issues moved successfully"})
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
