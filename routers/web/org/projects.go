// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	attachment_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplProjects           base.TplName = "org/projects/list"
	tplProjectsNew        base.TplName = "org/projects/new"
	tplProjectsView       base.TplName = "org/projects/view"
	tplGenericProjectsNew base.TplName = "user/project"
)

// MustEnableProjects check if projects are enabled in settings
func MustEnableProjects(ctx *context.Context) {
	if unit.TypeProjects.UnitGlobalDisabled() {
		ctx.NotFound("EnableKanbanBoard", nil)
		return
	}
}

// Projects renders the home page of projects
func Projects(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.project_board")

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"
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
	projects, total, err := project_model.FindProjects(ctx, project_model.SearchOptions{
		OwnerID:  ctx.ContextUser.ID,
		Page:     page,
		IsClosed: util.OptionalBoolOf(isShowClosed),
		SortType: sortType,
		Type:     projectType,
	})
	if err != nil {
		ctx.ServerError("FindProjects", err)
		return
	}

	opTotal, err := project_model.CountProjects(ctx, project_model.SearchOptions{
		OwnerID:  ctx.ContextUser.ID,
		IsClosed: util.OptionalBoolOf(!isShowClosed),
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
	shared_user.RenderUserHeader(ctx)

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	for _, project := range projects {
		project.RenderedContent = project.Description
	}

	numPages := 0
	if total > 0 {
		numPages = (int(total) - 1/setting.UI.IssuePagingNum)
	}

	pager := context.NewPagination(int(total), setting.UI.IssuePagingNum, page, numPages)
	pager.AddParam(ctx, "state", "State")
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

// NewProject render creating a project page
func NewProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	ctx.Data["BoardTypes"] = project_model.GetBoardConfig()
	ctx.Data["CardTypes"] = project_model.GetCardConfig()
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["HomeLink"] = ctx.ContextUser.HomeLink()
	shared_user.RenderUserHeader(ctx)
	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// NewProjectPost creates a new project
func NewProjectPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateProjectForm)
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	shared_user.RenderUserHeader(ctx)

	if ctx.HasError() {
		ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
		ctx.Data["PageIsViewProjects"] = true
		ctx.Data["BoardTypes"] = project_model.GetBoardConfig()
		ctx.Data["CardTypes"] = project_model.GetCardConfig()
		ctx.HTML(http.StatusOK, tplProjectsNew)
		return
	}

	newProject := project_model.Project{
		OwnerID:     ctx.ContextUser.ID,
		Title:       form.Title,
		Description: form.Content,
		CreatorID:   ctx.Doer.ID,
		BoardType:   form.BoardType,
		CardType:    form.CardType,
	}

	if ctx.ContextUser.IsOrganization() {
		newProject.Type = project_model.TypeOrganization
	} else {
		newProject.Type = project_model.TypeIndividual
	}

	if err := project_model.NewProject(&newProject); err != nil {
		ctx.ServerError("NewProject", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/projects")
}

// ChangeProjectStatus updates the status of a project between "open" and "close"
func ChangeProjectStatus(ctx *context.Context) {
	toClose := false
	switch ctx.Params(":action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/projects")
	}
	id := ctx.ParamsInt64(":id")

	if err := project_model.ChangeProjectStatusByRepoIDAndID(0, id, toClose); err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", err)
		} else {
			ctx.ServerError("ChangeProjectStatusByRepoIDAndID", err)
		}
		return
	}
	ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/projects?state=" + url.QueryEscape(ctx.Params(":action")))
}

// DeleteProject delete a project
func DeleteProject(ctx *context.Context) {
	p, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("", nil)
		return
	}

	if err := project_model.DeleteProjectByID(ctx, p.ID); err != nil {
		ctx.Flash.Error("DeleteProjectByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.projects.deletion_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.ContextUser.HomeLink() + "/-/projects",
	})
}

// EditProject allows a project to be edited
func EditProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["CardTypes"] = project_model.GetCardConfig()

	shared_user.RenderUserHeader(ctx)

	p, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("", nil)
		return
	}

	ctx.Data["projectID"] = p.ID
	ctx.Data["title"] = p.Title
	ctx.Data["content"] = p.Description
	ctx.Data["redirect"] = ctx.FormString("redirect")
	ctx.Data["HomeLink"] = ctx.ContextUser.HomeLink()
	ctx.Data["card_type"] = p.CardType

	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// EditProjectPost response for editing a project
func EditProjectPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateProjectForm)
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["CardTypes"] = project_model.GetCardConfig()

	shared_user.RenderUserHeader(ctx)

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplProjectsNew)
		return
	}

	p, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.OwnerID != ctx.ContextUser.ID {
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
		ctx.Redirect(p.Link())
	} else {
		ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/projects")
	}
}

// ViewProject renders the project board for a project
func ViewProject(ctx *context.Context) {
	project, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if project.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("", nil)
		return
	}

	boards, err := project.GetBoards(ctx)
	if err != nil {
		ctx.ServerError("GetProjectBoards", err)
		return
	}

	if boards[0].ID == 0 {
		boards[0].Title = ctx.Tr("repo.projects.type.uncategorized")
	}

	issuesMap, err := issues_model.LoadIssuesFromBoardList(ctx, boards)
	if err != nil {
		ctx.ServerError("LoadIssuesOfBoards", err)
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
			var referencedIds []int64
			for _, comment := range issue.Comments {
				if comment.RefIssueID != 0 && comment.RefIsPull {
					referencedIds = append(referencedIds, comment.RefIssueID)
				}
			}

			if len(referencedIds) > 0 {
				if linkedPrs, err := issues_model.Issues(ctx, &issues_model.IssuesOptions{
					IssueIDs: referencedIds,
					IsPull:   util.OptionalBoolTrue,
				}); err == nil {
					linkedPrsMap[issue.ID] = linkedPrs
				}
			}
		}
	}

	project.RenderedContent = project.Description
	ctx.Data["LinkedPRs"] = linkedPrsMap
	ctx.Data["PageIsViewProjects"] = true
	ctx.Data["CanWriteProjects"] = canWriteProjects(ctx)
	ctx.Data["Project"] = project
	ctx.Data["IssuesMap"] = issuesMap
	ctx.Data["Boards"] = boards
	shared_user.RenderUserHeader(ctx)

	ctx.HTML(http.StatusOK, tplProjectsView)
}

func getActionIssues(ctx *context.Context) []*issues_model.Issue {
	commaSeparatedIssueIDs := ctx.FormString("issue_ids")
	if len(commaSeparatedIssueIDs) == 0 {
		return nil
	}
	issueIDs := make([]int64, 0, 10)
	for _, stringIssueID := range strings.Split(commaSeparatedIssueIDs, ",") {
		issueID, err := strconv.ParseInt(stringIssueID, 10, 64)
		if err != nil {
			ctx.ServerError("ParseInt", err)
			return nil
		}
		issueIDs = append(issueIDs, issueID)
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		ctx.ServerError("GetIssuesByIDs", err)
		return nil
	}
	// Check access rights for all issues
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
func UpdateIssueProject(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	projectID := ctx.FormInt64("id")
	for _, issue := range issues {
		oldProjectID := issue.ProjectID()
		if oldProjectID == projectID {
			continue
		}

		if err := issues_model.ChangeProjectAssign(issue, ctx.Doer, projectID); err != nil {
			ctx.ServerError("ChangeProjectAssign", err)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// DeleteProjectBoard allows for the deletion of a project board
func DeleteProjectBoard(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}

	pb, err := project_model.GetBoard(ctx, ctx.ParamsInt64(":boardID"))
	if err != nil {
		ctx.ServerError("GetProjectBoard", err)
		return
	}
	if pb.ProjectID != ctx.ParamsInt64(":id") {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectBoard[%d] is not in Project[%d] as expected", pb.ID, project.ID),
		})
		return
	}

	if project.OwnerID != ctx.ContextUser.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectBoard[%d] is not in Owner[%d] as expected", pb.ID, ctx.ContextUser.ID),
		})
		return
	}

	if err := project_model.DeleteBoardByID(ctx.ParamsInt64(":boardID")); err != nil {
		ctx.ServerError("DeleteProjectBoardByID", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// AddBoardToProjectPost allows a new board to be added to a project.
func AddBoardToProjectPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditProjectBoardForm)

	project, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}

	if err := project_model.NewBoard(&project_model.Board{
		ProjectID: project.ID,
		Title:     form.Title,
		Color:     form.Color,
		CreatorID: ctx.Doer.ID,
	}); err != nil {
		ctx.ServerError("NewProjectBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// CheckProjectBoardChangePermissions check permission
func CheckProjectBoardChangePermissions(ctx *context.Context) (*project_model.Project, *project_model.Board) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return nil, nil
	}

	project, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return nil, nil
	}

	board, err := project_model.GetBoard(ctx, ctx.ParamsInt64(":boardID"))
	if err != nil {
		ctx.ServerError("GetProjectBoard", err)
		return nil, nil
	}
	if board.ProjectID != ctx.ParamsInt64(":id") {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectBoard[%d] is not in Project[%d] as expected", board.ID, project.ID),
		})
		return nil, nil
	}

	if project.OwnerID != ctx.ContextUser.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectBoard[%d] is not in Repository[%d] as expected", board.ID, project.ID),
		})
		return nil, nil
	}
	return project, board
}

// EditProjectBoard allows a project board's to be updated
func EditProjectBoard(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditProjectBoardForm)
	_, board := CheckProjectBoardChangePermissions(ctx)
	if ctx.Written() {
		return
	}

	if form.Title != "" {
		board.Title = form.Title
	}

	board.Color = form.Color

	if form.Sorting != 0 {
		board.Sorting = form.Sorting
	}

	if err := project_model.UpdateBoard(ctx, board); err != nil {
		ctx.ServerError("UpdateProjectBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// SetDefaultProjectBoard set default board for uncategorized issues/pulls
func SetDefaultProjectBoard(ctx *context.Context) {
	project, board := CheckProjectBoardChangePermissions(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.SetDefaultBoard(project.ID, board.ID); err != nil {
		ctx.ServerError("SetDefaultBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// UnsetDefaultProjectBoard unset default board for uncategorized issues/pulls
func UnsetDefaultProjectBoard(ctx *context.Context) {
	project, _ := CheckProjectBoardChangePermissions(ctx)
	if ctx.Written() {
		return
	}

	if err := project_model.SetDefaultBoard(project.ID, 0); err != nil {
		ctx.ServerError("SetDefaultBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// MoveIssues moves or keeps issues in a column and sorts them inside that column
func MoveIssues(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	project, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound("ProjectNotExist", nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if project.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("InvalidRepoID", nil)
		return
	}

	var board *project_model.Board

	if ctx.ParamsInt64(":boardID") == 0 {
		board = &project_model.Board{
			ID:        0,
			ProjectID: project.ID,
			Title:     ctx.Tr("repo.projects.type.uncategorized"),
		}
	} else {
		board, err = project_model.GetBoard(ctx, ctx.ParamsInt64(":boardID"))
		if err != nil {
			if project_model.IsErrProjectBoardNotExist(err) {
				ctx.NotFound("ProjectBoardNotExist", nil)
			} else {
				ctx.ServerError("GetProjectBoard", err)
			}
			return
		}
		if board.ProjectID != project.ID {
			ctx.NotFound("BoardNotInProject", nil)
			return
		}
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

	if err = project_model.MoveIssuesOnProjectBoard(board, sortedIssueIDs); err != nil {
		ctx.ServerError("MoveIssuesOnProjectBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}
