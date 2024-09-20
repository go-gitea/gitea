// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// GetProjects returns a list of projects for a given user and repository.
func GetProjects(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{reponame}/projects project repoGetProjects
	// ---
	// summary: Get a list of projects
	// description: Returns a list of projects for a given user and repository.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the project
	//   required: true
	//   type: string
	// - name: reponame
	//   in: path
	//   description: repository name.
	//   required: true
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	listOptions := utils.GetListOptions(ctx)
	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"

	searchOptions := project_model.SearchOptions{
		ListOptions: listOptions,
		IsClosed:    optional.Some(isShowClosed),
		OrderBy:     project_model.GetSearchOrderByBySortType(sortType),
		RepoID:      ctx.Repo.Repository.ID,
		Type:        project_model.TypeRepository,
	}

	projects, maxResults, err := db.FindAndCount[project_model.Project](ctx, &searchOptions)

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "db.FindAndCount[project_model.Project]", err)
		return
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, convert.ToProjects(ctx, projects))
}

// CreateProject creates a new project
func CreateProject(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{reponame}/projects project repoCreateProject
	// ---
	// summary: Create a new project
	// description: Creates a new project for a given user and repository.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the project
	//   required: true
	//   type: string
	// - name: reponame
	//   in: path
	//   description: repository name.
	//   required: true
	//   type: string
	// - name: body
	//   in: body
	//   description: Project data
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateProjectOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateProjectOption)

	project := &project_model.Project{
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.TemplateType(form.TemplateType),
		CardType:     project_model.CardType(form.CardType),
		Type:         project_model.TypeRepository,
		RepoID:       ctx.Repo.Repository.ID,
	}

	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewProject", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToProject(ctx, project))
}

// UpdateIssueProject moves issues from a project to another in a repository
func UpdateIssueProject(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{reponame}/projects/{type} project repoUpdateIssueProject
	// ---
	// summary: Moves issues from a project to another in a repository
	// consumes:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the project
	//   required: true
	//   type: string
	// - name: reponame
	//   in: path
	//   description: repository name.
	//   required: true
	//   type: string
	// - name: type
	//   in: path
	//   description: issue type (issues or pulls)
	//   required: true
	//   type: string
	//   enum:
	//   - issues
	//   - pulls
	// - name: body
	//   in: body
	//   description: issues data
	//   required: true
	//   schema:
	//    "$ref": "#/definitions/UpdateIssuesOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := &api.UpdateIssuesOption{}

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
		ctx.ServerError("LoadRepositories", err)
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

	ctx.Status(http.StatusNoContent)
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
