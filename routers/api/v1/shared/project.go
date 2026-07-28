// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"math"
	"net/http"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/routers/common"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	project_service "gitea.dev/services/projects"
)

// ProjectScope identifies whose projects a route operates on. Exactly one of Repo /
// Owner is set. Access rights are checked at the route level.
type ProjectScope struct {
	Type  project_model.Type
	Repo  *repo_model.Repository
	Owner *user_model.User
}

// projectScopeFromContext derives the scope from the route's assignment. Organization
// routes assign the org as the context user, so only the repository case is distinct.
func projectScopeFromContext(ctx *context.APIContext) ProjectScope {
	if ctx.Repo != nil && ctx.Repo.Repository != nil {
		return ProjectScope{Type: project_model.TypeRepository, Repo: ctx.Repo.Repository}
	}
	owner := ctx.ContextUser
	if owner == nil {
		owner = ctx.Doer // "/user/projects" acts on the authenticated user
	}
	projectType := project_model.TypeIndividual
	if owner.IsOrganization() {
		projectType = project_model.TypeOrganization
	}
	return ProjectScope{Type: projectType, Owner: owner}
}

func (s ProjectScope) repoID() int64 {
	if s.Repo != nil {
		return s.Repo.ID
	}
	return 0
}

func (s ProjectScope) ownerID() int64 {
	if s.Owner != nil {
		return s.Owner.ID
	}
	return 0
}

// attach preloads the relation the converter needs for HTMLURL, sparing a lookup per project.
func (s ProjectScope) attach(projects ...*project_model.Project) {
	for _, project := range projects {
		project.Repo, project.Owner = s.Repo, s.Owner
	}
}

// findProject loads the "id" path param, scoped to this owner: another owner's ID reads
// as not found.
func (s ProjectScope) findProject(ctx *context.APIContext) *project_model.Project {
	var project *project_model.Project
	var err error
	if s.Repo != nil {
		project, err = project_model.GetProjectForRepoByID(ctx, s.Repo.ID, ctx.PathParamInt64("id"))
	} else {
		project, err = project_model.GetProjectByIDAndOwner(ctx, ctx.PathParamInt64("id"), s.Owner.ID)
	}
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	s.attach(project)
	return project
}

// columnIn resolves the "column_id" path param inside an already-resolved project.
func columnIn(ctx *context.APIContext, project *project_model.Project) *project_model.Column {
	column, err := project_model.GetColumnByIDAndProjectID(ctx, ctx.PathParamInt64("column_id"), project.ID)
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	return column
}

func (s ProjectScope) findColumn(ctx *context.APIContext) *project_model.Column {
	project := s.findProject(ctx)
	if ctx.Written() {
		return nil
	}
	return columnIn(ctx, project)
}

// findColumnIssue additionally resolves the "issue_id" path param, rejecting closed projects.
func (s ProjectScope) findColumnIssue(ctx *context.APIContext) (*project_model.Column, *issues_model.Issue) {
	_, column := s.findOpenColumn(ctx)
	if ctx.Written() {
		return nil, nil
	}
	return column, s.findIssue(ctx, ctx.PathParamInt64("issue_id"))
}

// findIssue resolves an issue addressable within this scope. Owner-level boards span
// repositories, so the issue is looked up globally and gated on the doer's read access.
func (s ProjectScope) findIssue(ctx *context.APIContext, issueID int64) *issues_model.Issue {
	var issue *issues_model.Issue
	var err error
	if s.Repo != nil {
		issue, err = issues_model.GetIssueByRepoID(ctx, s.Repo.ID, issueID)
	} else {
		issue, err = issues_model.GetIssueByID(ctx, issueID)
	}
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	if s.Repo == nil {
		if err := issue.LoadRepo(ctx); err != nil {
			ctx.APIErrorInternal(err)
			return nil
		}
		perm, err := access_model.GetDoerRepoPermission(ctx, issue.Repo, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return nil
		}
		// hide the issue's existence rather than reporting it as forbidden
		if !perm.CanRead(unit.TypeIssues) {
			ctx.APIErrorNotFound()
			return nil
		}
	}
	return issue
}

// findOpenProject is findProject for mutating endpoints: a closed project is read-only.
func (s ProjectScope) findOpenProject(ctx *context.APIContext) *project_model.Project {
	project := s.findProject(ctx)
	if ctx.Written() {
		return nil
	}
	if project.IsClosed {
		ctx.APIError(http.StatusForbidden, "project is closed")
		return nil
	}
	return project
}

// findOpenColumn is findColumn with the same closed-project rejection.
func (s ProjectScope) findOpenColumn(ctx *context.APIContext) (*project_model.Project, *project_model.Column) {
	project := s.findOpenProject(ctx)
	if ctx.Written() {
		return nil, nil
	}
	return project, columnIn(ctx, project)
}

func ListProjects(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects repository repoListProjects
	// ---
	// summary: List a repository's projects
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: state
	//   in: query
	//   description: State of the project (open, closed, all)
	//   type: string
	//   enum: [open, closed, all]
	//   default: open
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /orgs/{org}/projects organization orgListProjects
	// ---
	// summary: List an organization's projects
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: state
	//   in: query
	//   description: State of the project (open, closed, all)
	//   type: string
	//   enum: [open, closed, all]
	//   default: open
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /user/projects user userListProjects
	// ---
	// summary: List your projects
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: state
	//   in: query
	//   description: State of the project (open, closed, all)
	//   type: string
	//   enum: [open, closed, all]
	//   default: open
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /users/{username}/projects user userListUserProjects
	// ---
	// summary: List a user's projects
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: state
	//   in: query
	//   description: State of the project (open, closed, all)
	//   type: string
	//   enum: [open, closed, all]
	//   default: open
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	listOptions := utils.GetListOptions(ctx)
	projects, count, err := db.FindAndCount[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: listOptions,
		RepoID:      scope.repoID(),
		OwnerID:     scope.ownerID(),
		IsClosed:    common.ParseIssueFilterStateIsClosed(ctx.FormTrim("state")),
		Type:        scope.Type,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// attach first, else LoadOwner re-queries the owner this scope already holds, per project
	scope.attach(projects...)
	if err := project_service.LoadIssueNumbersForProjects(ctx, projects, ctx.Doer); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, convert.ToProjectList(ctx, projects, ctx.Doer))
}

func GetProject(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id} repository repoGetProject
	// ---
	// summary: Get a project
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /orgs/{org}/projects/{id} organization orgGetProject
	// ---
	// summary: Get a project
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /user/projects/{id} user userGetProject
	// ---
	// summary: Get a project
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project := scope.findProject(ctx)
	if ctx.Written() {
		return
	}
	if err := project_service.LoadIssueNumbersForProject(ctx, project, ctx.Doer); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProject(ctx, project, ctx.Doer))
}

func CreateProject(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects repository repoCreateProject
	// ---
	// summary: Create a project owned by a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /orgs/{org}/projects organization orgCreateProject
	// ---
	// summary: Create a project owned by an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /user/projects user userCreateProject
	// ---
	// summary: Create a project owned by the authenticated user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Project"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	form := web.GetForm(ctx).(*api.CreateProjectOption)

	templateType, err := convert.ProjectTemplateTypeFromString(form.TemplateType)
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		return
	}
	cardType, err := convert.ProjectCardTypeFromString(form.CardType)
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		return
	}

	project := &project_model.Project{
		RepoID:       scope.repoID(),
		OwnerID:      scope.ownerID(),
		Title:        form.Title,
		Description:  form.Description,
		CreatorID:    ctx.Doer.ID,
		TemplateType: templateType,
		CardType:     cardType,
		Type:         scope.Type,
	}
	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	scope.attach(project)
	ctx.JSON(http.StatusCreated, convert.ToProject(ctx, project, ctx.Doer))
}

func EditProject(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{id} repository repoEditProject
	// ---
	// summary: Edit a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation PATCH /orgs/{org}/projects/{id} organization orgEditProject
	// ---
	// summary: Edit a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation PATCH /user/projects/{id} user userEditProject
	// ---
	// summary: Edit a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Project"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project := scope.findProject(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectOption)
	opts := project_service.UpdateProjectOptions{
		Title:       optional.FromPtr(form.Title),
		Description: optional.FromPtr(form.Description),
	}
	if form.CardType != nil {
		cardType, err := convert.ProjectCardTypeFromString(*form.CardType)
		if err != nil {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
			return
		}
		opts.CardType = optional.Some(cardType)
	}
	if form.State != nil {
		switch *form.State {
		case api.StateOpen:
			opts.IsClosed = optional.Some(false)
		case api.StateClosed:
			opts.IsClosed = optional.Some(true)
		default:
			ctx.APIError(http.StatusUnprocessableEntity, "state must be 'open' or 'closed'")
			return
		}
	}
	if err := project_service.UpdateProject(ctx, project, opts); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := project_service.LoadIssueNumbersForProject(ctx, project, ctx.Doer); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProject(ctx, project, ctx.Doer))
}

func DeleteProject(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id} repository repoDeleteProject
	// ---
	// summary: Delete a project
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation DELETE /orgs/{org}/projects/{id} organization orgDeleteProject
	// ---
	// summary: Delete a project
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation DELETE /user/projects/{id} user userDeleteProject
	// ---
	// summary: Delete a project
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project := scope.findProject(ctx)
	if ctx.Written() {
		return
	}
	if err := project_model.DeleteProjectByID(ctx, project.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func ListProjectColumns(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id}/columns repository repoListProjectColumns
	// ---
	// summary: List a project's columns
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/ProjectColumnList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /orgs/{org}/projects/{id}/columns organization orgListProjectColumns
	// ---
	// summary: List a project's columns
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/ProjectColumnList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /user/projects/{id}/columns user userListProjectColumns
	// ---
	// summary: List a project's columns
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/ProjectColumnList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project := scope.findProject(ctx)
	if ctx.Written() {
		return
	}

	listOptions := utils.GetListOptions(ctx)
	columns, total, err := db.FindAndCount[project_model.Column](ctx, project_model.SearchColumnOptions{
		ListOptions: listOptions,
		ProjectID:   project.ID,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, convert.ToProjectColumnList(ctx, columns, ctx.Doer))
}

func CreateProjectColumn(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/columns repository repoCreateProjectColumn
	// ---
	// summary: Create a column in a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectColumnOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectColumn"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /orgs/{org}/projects/{id}/columns organization orgCreateProjectColumn
	// ---
	// summary: Create a column in a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectColumnOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectColumn"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /user/projects/{id}/columns user userCreateProjectColumn
	// ---
	// summary: Create a column in a project
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateProjectColumnOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectColumn"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project := scope.findOpenProject(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.CreateProjectColumnOption)
	column := &project_model.Column{
		Title:     form.Title,
		Color:     form.Color,
		ProjectID: project.ID,
		CreatorID: ctx.Doer.ID,
	}
	if err := project_model.NewColumn(ctx, column); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToProjectColumn(ctx, column, ctx.Doer))
}

func GetProjectColumn(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id}/columns/{column_id} repository repoGetProjectColumn
	// ---
	// summary: Get a project column
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumn"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /orgs/{org}/projects/{id}/columns/{column_id} organization orgGetProjectColumn
	// ---
	// summary: Get a project column
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumn"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /user/projects/{id}/columns/{column_id} user userGetProjectColumn
	// ---
	// summary: Get a project column
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumn"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	column := scope.findColumn(ctx)
	if ctx.Written() {
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProjectColumn(ctx, column, ctx.Doer))
}

func EditProjectColumn(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/projects/{id}/columns/{column_id} repository repoEditProjectColumn
	// ---
	// summary: Edit a project column
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectColumnOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumn"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation PATCH /orgs/{org}/projects/{id}/columns/{column_id} organization orgEditProjectColumn
	// ---
	// summary: Edit a project column
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectColumnOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumn"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation PATCH /user/projects/{id}/columns/{column_id} user userEditProjectColumn
	// ---
	// summary: Edit a project column
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditProjectColumnOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectColumn"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	_, column := scope.findOpenColumn(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.EditProjectColumnOption)
	if form.Title != nil {
		column.Title = *form.Title
	}
	if form.Color != nil {
		column.Color = *form.Color
	}
	if form.Sorting != nil {
		if *form.Sorting < math.MinInt8 || *form.Sorting > math.MaxInt8 {
			ctx.APIError(http.StatusUnprocessableEntity, "sorting must be between -128 and 127")
			return
		}
		column.Sorting = int8(*form.Sorting)
	}

	if err := project_model.UpdateColumn(ctx, column); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToProjectColumn(ctx, column, ctx.Doer))
}

// DeleteProjectColumn removes a column, moving its issues to the default column.

func DeleteProjectColumn(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id}/columns/{column_id} repository repoDeleteProjectColumn
	// ---
	// summary: Delete a project column
	// description: The default column cannot be deleted while it is still the column new issues land in.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation DELETE /orgs/{org}/projects/{id}/columns/{column_id} organization orgDeleteProjectColumn
	// ---
	// summary: Delete a project column
	// description: The default column cannot be deleted while it is still the column new issues land in.
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation DELETE /user/projects/{id}/columns/{column_id} user userDeleteProjectColumn
	// ---
	// summary: Delete a project column
	// description: The default column cannot be deleted while it is still the column new issues land in.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	_, column := scope.findOpenColumn(ctx)
	if ctx.Written() {
		return
	}
	if err := project_model.DeleteColumnByID(ctx, column.ID); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// SetDefaultProjectColumn marks a column as the one new issues land in.

func SetDefaultProjectColumn(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/columns/{column_id}/default repository repoSetDefaultProjectColumn
	// ---
	// summary: Set a project's default column
	// description: The default column is where newly assigned issues land.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /orgs/{org}/projects/{id}/columns/{column_id}/default organization orgSetDefaultProjectColumn
	// ---
	// summary: Set a project's default column
	// description: The default column is where newly assigned issues land.
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /user/projects/{id}/columns/{column_id}/default user userSetDefaultProjectColumn
	// ---
	// summary: Set a project's default column
	// description: The default column is where newly assigned issues land.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project, column := scope.findOpenColumn(ctx)
	if ctx.Written() {
		return
	}
	if err := project_model.SetDefaultColumn(ctx, project.ID, column.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// MoveProjectColumns reorders every column at once; the body lists all IDs in their new order.

func MoveProjectColumns(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/columns/move repository repoMoveProjectColumns
	// ---
	// summary: Reorder a project's columns
	// description: Reorders every column of the project at once; the body lists all column IDs in their new order.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MoveProjectColumnsOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /orgs/{org}/projects/{id}/columns/move organization orgMoveProjectColumns
	// ---
	// summary: Reorder a project's columns
	// description: Reorders every column of the project at once; the body lists all column IDs in their new order.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MoveProjectColumnsOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /user/projects/{id}/columns/move user userMoveProjectColumns
	// ---
	// summary: Reorder a project's columns
	// description: Reorders every column of the project at once; the body lists all column IDs in their new order.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MoveProjectColumnsOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project := scope.findOpenProject(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.MoveProjectColumnsOption)
	columns, err := project_model.GetProjectColumns(ctx, project.ID, db.ListOptionsAll)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if len(form.ColumnIDs) != len(columns) {
		ctx.APIError(http.StatusUnprocessableEntity, "column_ids must list every column of the project exactly once")
		return
	}

	sortedColumnIDs := make(map[int64]int64, len(form.ColumnIDs))
	known := make(container.Set[int64], len(columns))
	for _, column := range columns {
		known.Add(column.ID)
	}
	for position, columnID := range form.ColumnIDs {
		if !known.Remove(columnID) {
			ctx.APIError(http.StatusUnprocessableEntity, "column_ids must list every column of the project exactly once")
			return
		}
		sortedColumnIDs[int64(position)] = columnID
	}

	if err := project_model.MoveColumnsOnProject(ctx, project, sortedColumnIDs); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListProjectColumnIssues lists the issues placed in a column, filtered to what the doer may see.

func ListProjectColumnIssues(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/projects/{id}/columns/{column_id}/issues repository repoListProjectColumnIssues
	// ---
	// summary: List the issues in a project column
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /orgs/{org}/projects/{id}/columns/{column_id}/issues organization orgListProjectColumnIssues
	// ---
	// summary: List the issues in a project column
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation GET /user/projects/{id}/columns/{column_id}/issues user userListProjectColumnIssues
	// ---
	// summary: List the issues in a project column
	// description: Projects track issues and pull requests; standalone note cards are not supported.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	column := scope.findColumn(ctx)
	if ctx.Written() {
		return
	}

	listOptions := utils.GetListOptions(ctx)
	issuesOpts := &issues_model.IssuesOptions{
		Paginator:        &listOptions,
		ProjectIDs:       []int64{column.ProjectID},
		ProjectColumnIDs: project_model.ColumnIssueIDs(column),
		SortType:         issues_model.SortTypeProjectColumnSorting,
	}
	if scope.Repo != nil {
		// the route already established repo read access, and every issue here is that repo's
		issuesOpts.RepoIDs = []int64{scope.Repo.ID}
	} else {
		// an owner-level board spans repositories, so filter to what this doer may see
		issuesOpts.Owner = scope.Owner
		issuesOpts.Doer = ctx.Doer
		issuesOpts.AllPublic = ctx.Doer == nil
	}

	count, err := issues_model.CountIssues(ctx, issuesOpts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	issues, err := issues_model.Issues(ctx, issuesOpts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetLinkHeader(count, listOptions.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, ctx.Doer, issues))
}

// AddIssueToProjectColumn assigns an issue to the project and places it in the column.

func AddIssueToProjectColumn(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/columns/{column_id}/issues/{issue_id} repository repoAddIssueToProjectColumn
	// ---
	// summary: Add an issue to a project column
	// description: Assigns the issue to the project if it is not a member yet, then places it in the column.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /orgs/{org}/projects/{id}/columns/{column_id}/issues/{issue_id} organization orgAddIssueToProjectColumn
	// ---
	// summary: Add an issue to a project column
	// description: Assigns the issue to the project if it is not a member yet, then places it in the column.
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /user/projects/{id}/columns/{column_id}/issues/{issue_id} user userAddIssueToProjectColumn
	// ---
	// summary: Add an issue to a project column
	// description: Assigns the issue to the project if it is not a member yet, then places it in the column.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	column, issue := scope.findColumnIssue(ctx)
	if ctx.Written() {
		return
	}
	if err := project_service.AddIssueToColumn(ctx, ctx.Doer, issue, column); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusCreated)
}

// RemoveIssueFromProjectColumn detaches an issue from the column's project.

func RemoveIssueFromProjectColumn(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/projects/{id}/columns/{column_id}/issues/{issue_id} repository repoRemoveIssueFromProjectColumn
	// ---
	// summary: Remove an issue from a project column
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation DELETE /orgs/{org}/projects/{id}/columns/{column_id}/issues/{issue_id} organization orgRemoveIssueFromProjectColumn
	// ---
	// summary: Remove an issue from a project column
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation DELETE /user/projects/{id}/columns/{column_id}/issues/{issue_id} user userRemoveIssueFromProjectColumn
	// ---
	// summary: Remove an issue from a project column
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: column_id
	//   in: path
	//   description: id of the column
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	column, issue := scope.findColumnIssue(ctx)
	if ctx.Written() {
		return
	}
	if err := project_service.RemoveIssueFromColumn(ctx, ctx.Doer, issue, column); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// MoveProjectIssue moves an issue that is already in the project into another column.

func MoveProjectIssue(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/projects/{id}/issues/{issue_id}/move repository repoMoveProjectIssue
	// ---
	// summary: Move an issue between a project's columns
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MoveProjectIssueOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /orgs/{org}/projects/{id}/issues/{issue_id}/move organization orgMoveProjectIssue
	// ---
	// summary: Move an issue between a project's columns
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MoveProjectIssueOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// swagger:operation POST /user/projects/{id}/issues/{issue_id}/move user userMoveProjectIssue
	// ---
	// summary: Move an issue between a project's columns
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: integer
	//   format: int64
	//   required: true
	// - name: issue_id
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MoveProjectIssueOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	scope := projectScopeFromContext(ctx)
	project := scope.findOpenProject(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*api.MoveProjectIssueOption)
	column, err := project_model.GetColumnByIDAndProjectID(ctx, form.ColumnID, project.ID)
	if err != nil {
		if project_model.IsErrProjectColumnNotExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, "target column does not belong to this project")
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	issue := scope.findIssue(ctx, ctx.PathParamInt64("issue_id"))
	if ctx.Written() {
		return
	}

	if err := project_service.MoveIssueToColumn(ctx, ctx.Doer, issue, column, optional.FromPtr(form.Sorting)); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
