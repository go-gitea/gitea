// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/issue"

	"xorm.io/builder"
)

const (
	tplMilestone       templates.TplName = "repo/issue/milestones"
	tplMilestoneNew    templates.TplName = "repo/issue/milestone_new"
	tplMilestoneIssues templates.TplName = "repo/issue/milestone_issues"
)

// Milestones render milestones page
func Milestones(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsMilestones"] = true

	isShowClosed := ctx.FormString("state") == "closed"
	sortType := ctx.FormString("sort")
	keyword := ctx.FormTrim("q")
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	miles, total, err := db.FindAndCount[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.IssuePagingNum,
		},
		RepoID:   ctx.Repo.Repository.ID,
		IsClosed: optional.Some(isShowClosed),
		SortType: sortType,
		Name:     keyword,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}

	stats, err := issues_model.GetMilestonesStatsByRepoCondAndKw(ctx, builder.And(builder.Eq{"id": ctx.Repo.Repository.ID}), keyword)
	if err != nil {
		ctx.ServerError("GetMilestoneStats", err)
		return
	}
	ctx.Data["OpenCount"] = stats.OpenCount
	ctx.Data["ClosedCount"] = stats.ClosedCount
	if ctx.Repo.Repository.IsTimetrackerEnabled(ctx) {
		if err := issues_model.MilestoneList(miles).LoadTotalTrackedTimes(ctx); err != nil {
			ctx.ServerError("LoadTotalTrackedTimes", err)
			return
		}
	}
	for _, m := range miles {
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository)
		m.RenderedContent, err = markdown.RenderString(rctx, m.Content)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
	}
	ctx.Data["Milestones"] = miles

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	ctx.Data["SortType"] = sortType
	ctx.Data["Keyword"] = keyword
	ctx.Data["IsShowClosed"] = isShowClosed

	pager := context.NewPagination(int(total), setting.UI.IssuePagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplMilestone)
}

// NewMilestone render creating milestone page
func NewMilestone(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsMilestones"] = true
	ctx.HTML(http.StatusOK, tplMilestoneNew)
}

// NewMilestonePost response for creating milestone
func NewMilestonePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateMilestoneForm)
	ctx.Data["Title"] = ctx.Tr("repo.milestones.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsMilestones"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplMilestoneNew)
		return
	}

	deadlineUnix, err := common.ParseDeadlineDateToEndOfDay(form.Deadline)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestoneNew, &form)
		return
	}

	if err := issues_model.NewMilestone(ctx, &issues_model.Milestone{
		RepoID:       ctx.Repo.Repository.ID,
		Name:         form.Title,
		Content:      form.Content,
		DeadlineUnix: deadlineUnix,
	}); err != nil {
		ctx.ServerError("NewMilestone", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.milestones.create_success", form.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/milestones")
}

// EditMilestone render edting milestone page
func EditMilestone(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones.edit")
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["PageIsEditMilestone"] = true

	m, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetMilestoneByRepoID", err)
		}
		return
	}
	ctx.Data["title"] = m.Name
	ctx.Data["content"] = m.Content
	if len(m.DeadlineString) > 0 {
		ctx.Data["deadline"] = m.DeadlineString
	}
	ctx.HTML(http.StatusOK, tplMilestoneNew)
}

// EditMilestonePost response for edting milestone
func EditMilestonePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateMilestoneForm)
	ctx.Data["Title"] = ctx.Tr("repo.milestones.edit")
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["PageIsEditMilestone"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplMilestoneNew)
		return
	}

	deadlineUnix, err := common.ParseDeadlineDateToEndOfDay(form.Deadline)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestoneNew, &form)
		return
	}

	m, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetMilestoneByRepoID", err)
		}
		return
	}
	m.Name = form.Title
	m.Content = form.Content
	m.DeadlineUnix = deadlineUnix
	if err = issues_model.UpdateMilestone(ctx, m, m.IsClosed); err != nil {
		ctx.ServerError("UpdateMilestone", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.milestones.edit_success", m.Name))
	ctx.Redirect(ctx.Repo.RepoLink + "/milestones")
}

// ChangeMilestoneStatus response for change a milestone's status
func ChangeMilestoneStatus(ctx *context.Context) {
	var toClose bool
	switch ctx.PathParam("action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/milestones")
		return
	}
	id := ctx.PathParamInt64("id")

	if err := issues_model.ChangeMilestoneStatusByRepoIDAndID(ctx, ctx.Repo.Repository.ID, id, toClose); err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("ChangeMilestoneStatusByIDAndRepoID", err)
		}
		return
	}
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/milestones?state=" + url.QueryEscape(ctx.PathParam("action")))
}

// DeleteMilestone delete a milestone
func DeleteMilestone(ctx *context.Context) {
	if err := issues_model.DeleteMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteMilestoneByRepoID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.milestones.deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/milestones")
}

// MilestoneIssuesAndPulls lists all the issues and pull requests of the milestone
func MilestoneIssuesAndPulls(ctx *context.Context) {
	milestoneID := ctx.PathParamInt64("id")
	projectID := ctx.FormInt64("project")
	milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
	if err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound(err)
			return
		}

		ctx.ServerError("GetMilestoneByID", err)
		return
	}

	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository)
	milestone.RenderedContent, err = markdown.RenderString(rctx, milestone.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.Data["Title"] = milestone.Name
	ctx.Data["Milestone"] = milestone

	issues(ctx, milestoneID, projectID, optional.None[bool]())

	ret := issue.ParseTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["NewIssueChooseTemplate"] = len(ret.IssueTemplates) > 0

	ctx.Data["CanWriteIssues"] = ctx.Repo.CanWriteIssuesOrPulls(false)
	ctx.Data["CanWritePulls"] = ctx.Repo.CanWriteIssuesOrPulls(true)

	ctx.HTML(http.StatusOK, tplMilestoneIssues)
}
