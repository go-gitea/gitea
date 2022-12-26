// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	board_model "code.gitea.io/gitea/models/board"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplProjects           base.TplName = "repo/projects/list"
	tplProjectsNew        base.TplName = "repo/projects/new"
	tplProjectsView       base.TplName = "repo/projects/view"
	tplGenericProjectsNew base.TplName = "user/project"
)

// MustEnableProjects check if projects are enabled in settings
func MustEnableProjects(ctx *context.Context) {
	if unit.TypeBoards.UnitGlobalDisabled() {
		ctx.NotFound("EnableKanbanBoard", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.CanRead(unit.TypeBoards) {
			ctx.NotFound("MustEnableProjects", nil)
			return
		}
	}
}

// Boards renders the home page of boards
func Boards(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.project_board")

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"
	repo := ctx.Repo.Repository
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	ctx.Data["OpenCount"] = repo.NumOpenBoards
	ctx.Data["ClosedCount"] = repo.NumClosedBoards

	var total int
	if !isShowClosed {
		total = repo.NumOpenBoards
	} else {
		total = repo.NumClosedBoards
	}

	projects, count, err := board_model.FindBoards(ctx, board_model.SearchOptions{
		RepoID:   repo.ID,
		Page:     page,
		IsClosed: util.OptionalBoolOf(isShowClosed),
		SortType: sortType,
		Type:     board_model.TypeRepository,
	})
	if err != nil {
		ctx.ServerError("FindBoards", err)
		return
	}

	for i := range projects {
		projects[i].RenderedContent, err = markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     ctx.Repo.Repository.ComposeMetas(),
			GitRepo:   ctx.Repo.GitRepo,
			Ctx:       ctx,
		}, projects[i].Description)
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
	pager.AddParam(ctx, "state", "State")
	ctx.Data["Page"] = pager

	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["IsProjectsPage"] = true
	ctx.Data["SortType"] = sortType

	ctx.HTML(http.StatusOK, tplProjects)
}

// NewBoard render creating a project page
func NewBoard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	ctx.Data["ProjectTypes"] = board_model.GetBoardsConfig()
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)
	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// NewBoardPost creates a new project
func NewBoardPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateBoardForm)
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")

	if ctx.HasError() {
		ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)
		ctx.Data["ProjectTypes"] = board_model.GetBoardsConfig()
		ctx.HTML(http.StatusOK, tplProjectsNew)
		return
	}

	if err := board_model.NewBoard(&board_model.Board{
		RepoID:      ctx.Repo.Repository.ID,
		Title:       form.Title,
		Description: form.Content,
		CreatorID:   ctx.Doer.ID,
		ColumnType:  form.ColumnType,
		Type:        board_model.TypeRepository,
	}); err != nil {
		ctx.ServerError("NewBoard", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/projects")
}

// ChangeBoardStatus updates the status of a board between "open" and "close"
func ChangeBoardStatus(ctx *context.Context) {
	toClose := false
	switch ctx.Params(":action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.Redirect(ctx.Repo.RepoLink + "/projects")
	}
	id := ctx.ParamsInt64(":id")

	if err := board_model.ChangeBoardStatusByRepoIDAndID(ctx.Repo.Repository.ID, id, toClose); err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", err)
		} else {
			ctx.ServerError("ChangeProjectStatusByIDAndRepoID", err)
		}
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/projects?state=" + url.QueryEscape(ctx.Params(":action")))
}

// DeleteBoard delete a board
func DeleteBoard(ctx *context.Context) {
	p, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
		return
	}

	if err := board_model.DeleteBoardByID(ctx, p.ID); err != nil {
		ctx.Flash.Error("DeleteBoardByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.projects.deletion_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/projects",
	})
}

// EditBoard allows a board to be edited
func EditBoard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)

	p, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
		return
	}

	ctx.Data["title"] = p.Title
	ctx.Data["content"] = p.Description

	ctx.HTML(http.StatusOK, tplProjectsNew)
}

// EditBoardPost response for editing a board
func EditBoardPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateBoardForm)
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsEditProjects"] = true
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplProjectsNew)
		return
	}

	p, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
		return
	}

	p.Title = form.Title
	p.Description = form.Content
	if err = board_model.UpdateBoard(ctx, p); err != nil {
		ctx.ServerError("UpdateProjects", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.edit_success", p.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/projects")
}

// ViewBoard renders the columns for a board
func ViewBoard(ctx *context.Context) {
	project, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return
	}
	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("", nil)
		return
	}

	boards, err := board_model.FindColumns(ctx, project.ID)
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
	ctx.Data["LinkedPRs"] = linkedPrsMap

	project.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.Repo.RepoLink,
		Metas:     ctx.Repo.Repository.ComposeMetas(),
		GitRepo:   ctx.Repo.GitRepo,
		Ctx:       ctx,
	}, project.Description)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.Data["IsProjectsPage"] = true
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)
	ctx.Data["Project"] = project
	ctx.Data["IssuesMap"] = issuesMap
	ctx.Data["Boards"] = boards

	ctx.HTML(http.StatusOK, tplProjectsView)
}

// UpdateIssueProject change an issue's project
func UpdateIssueProject(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	projectID := ctx.FormInt64("id")
	for _, issue := range issues {
		oldProjectID := issue.BoardID()
		if oldProjectID == projectID {
			continue
		}

		if err := issues_model.ChangeBoardAssign(issue, ctx.Doer, projectID); err != nil {
			ctx.ServerError("ChangeProjectAssign", err)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// DeleteBoardColumn allows for the deletion of a board column
func DeleteBoardColumn(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return
	}

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeBoards) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return
	}

	project, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return
	}

	pb, err := board_model.GetColumn(ctx, ctx.ParamsInt64(":boardID"))
	if err != nil {
		ctx.ServerError("GetProjectBoard", err)
		return
	}
	if pb.BoardID != ctx.ParamsInt64(":id") {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectBoard[%d] is not in Project[%d] as expected", pb.ID, project.ID),
		})
		return
	}

	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("ProjectBoard[%d] is not in Repository[%d] as expected", pb.ID, ctx.Repo.Repository.ID),
		})
		return
	}

	if err := board_model.DeleteBoardByID(ctx, ctx.ParamsInt64(":boardID")); err != nil {
		ctx.ServerError("DeleteProjectBoardByID", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// AddBoardColumnPost allows a new column to be added to a board.
func AddBoardColumnPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditBoardColumnForm)
	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeBoards) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return
	}

	board, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return
	}

	if err := board_model.NewColumn(&board_model.Column{
		BoardID:   board.ID,
		Title:     form.Title,
		Color:     form.Color,
		CreatorID: ctx.Doer.ID,
	}); err != nil {
		ctx.ServerError("NewBoardBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

func checkBoardColumnChangePermissions(ctx *context.Context) (*board_model.Board, *board_model.Column) {
	if ctx.Doer == nil {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only signed in users are allowed to perform this action.",
		})
		return nil, nil
	}

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeBoards) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return nil, nil
	}

	board, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return nil, nil
	}

	column, err := board_model.GetColumn(ctx, ctx.ParamsInt64(":boardID"))
	if err != nil {
		ctx.ServerError("GetProjectBoard", err)
		return nil, nil
	}
	if column.BoardID != ctx.ParamsInt64(":id") {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("BoardColumn[%d] is not in Board[%d] as expected", column.ID, board.ID),
		})
		return nil, nil
	}

	if board.RepoID != ctx.Repo.Repository.ID {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
			"message": fmt.Sprintf("BoardColumn[%d] is not in Repository[%d] as expected", column.ID, ctx.Repo.Repository.ID),
		})
		return nil, nil
	}
	return board, column
}

// EditBoardColumn allows a board column's to be updated
func EditBoardColumn(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditBoardColumnForm)
	_, column := checkBoardColumnChangePermissions(ctx)
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

	if err := board_model.UpdateColumn(ctx, column); err != nil {
		ctx.ServerError("UpdateProjectBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// SetDefaultBoardColumn set default board column for uncategorized issues/pulls
func SetDefaultBoardColumn(ctx *context.Context) {
	board, column := checkBoardColumnChangePermissions(ctx)
	if ctx.Written() {
		return
	}

	if err := board_model.SetDefaultColumn(board.ID, column.ID); err != nil {
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

	if !ctx.Repo.IsOwner() && !ctx.Repo.IsAdmin() && !ctx.Repo.CanAccess(perm.AccessModeWrite, unit.TypeBoards) {
		ctx.JSON(http.StatusForbidden, map[string]string{
			"message": "Only authorized users are allowed to perform this action.",
		})
		return
	}

	project, err := board_model.GetBoardByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if board_model.IsErrBoardNotExist(err) {
			ctx.NotFound("ProjectNotExist", nil)
		} else {
			ctx.ServerError("GetBoardByID", err)
		}
		return
	}
	if project.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("InvalidRepoID", nil)
		return
	}

	var column *board_model.Column

	if ctx.ParamsInt64(":boardID") == 0 {
		column = &board_model.Column{
			ID:      0,
			BoardID: project.ID,
			Title:   ctx.Tr("repo.projects.type.uncategorized"),
		}
	} else {
		column, err = board_model.GetColumn(ctx, ctx.ParamsInt64(":boardID"))
		if err != nil {
			if board_model.IsErrColumnNotExist(err) {
				ctx.NotFound("ProjectBoardNotExist", nil)
			} else {
				ctx.ServerError("GetProjectBoard", err)
			}
			return
		}
		if column.BoardID != project.ID {
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

	for _, issue := range movedIssues {
		if issue.RepoID != project.RepoID {
			ctx.ServerError("Some issue's repoID is not equal to project's repoID", errors.New("Some issue's repoID is not equal to project's repoID"))
			return
		}
	}

	if err = board_model.MoveIssuesOnBoardColumn(column, sortedIssueIDs); err != nil {
		ctx.ServerError("MoveIssuesOnProjectBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// CreateProject renders the generic project creation page
func CreateProject(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.new")
	ctx.Data["ProjectTypes"] = board_model.GetBoardsConfig()
	ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)

	ctx.HTML(http.StatusOK, tplGenericProjectsNew)
}

// CreateProjectPost creates an individual and/or organization project
func CreateProjectPost(ctx *context.Context, form forms.UserCreateProjectForm) {
	user := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}

	ctx.Data["ContextUser"] = user

	if ctx.HasError() {
		ctx.Data["CanWriteProjects"] = ctx.Repo.Permission.CanWrite(unit.TypeBoards)
		ctx.HTML(http.StatusOK, tplGenericProjectsNew)
		return
	}

	projectType := board_model.TypeIndividual
	if user.IsOrganization() {
		projectType = board_model.TypeOrganization
	}

	if err := board_model.NewBoard(&board_model.Board{
		Title:       form.Title,
		Description: form.Content,
		CreatorID:   user.ID,
		ColumnType:  form.ColumnType,
		Type:        projectType,
	}); err != nil {
		ctx.ServerError("NewBoard", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.projects.create_success", form.Title))
	ctx.Redirect(setting.AppSubURL + "/")
}
