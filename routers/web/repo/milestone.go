// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/issue"

	"xorm.io/builder"
)

const (
	tplMilestone       base.TplName = "repo/issue/milestones"
	tplMilestoneNew    base.TplName = "repo/issue/milestone_new"
	tplMilestoneIssues base.TplName = "repo/issue/milestone_issues"
)

// Milestones render milestones page
func Milestones(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsMilestones"] = true

	isShowClosed := ctx.FormString("state") == "closed"
	stats, err := issues_model.GetMilestonesStatsByRepoCond(builder.And(builder.Eq{"id": ctx.Repo.Repository.ID}))
	if err != nil {
		ctx.ServerError("MilestoneStats", err)
		return
	}
	ctx.Data["OpenCount"] = stats.OpenCount
	ctx.Data["ClosedCount"] = stats.ClosedCount

	sortType := ctx.FormString("sort")

	keyword := ctx.FormTrim("q")

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	state := structs.StateOpen
	if isShowClosed {
		state = structs.StateClosed
	}

	miles, total, err := issues_model.GetMilestones(issues_model.GetMilestonesOption{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.IssuePagingNum,
		},
		RepoID:   ctx.Repo.Repository.ID,
		State:    state,
		SortType: sortType,
		Name:     keyword,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
	if ctx.Repo.Repository.IsTimetrackerEnabled(ctx) {
		if err := miles.LoadTotalTrackedTimes(); err != nil {
			ctx.ServerError("LoadTotalTrackedTimes", err)
			return
		}
	}
	for _, m := range miles {
		m.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     ctx.Repo.Repository.ComposeMetas(),
			GitRepo:   ctx.Repo.GitRepo,
			Ctx:       ctx,
		}, m.Content)
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
	pager.AddParam(ctx, "state", "State")
	pager.AddParam(ctx, "q", "Keyword")
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

	if len(form.Deadline) == 0 {
		form.Deadline = "9999-12-31"
	}
	deadline, err := time.ParseInLocation("2006-01-02", form.Deadline, time.Local)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestoneNew, &form)
		return
	}

	deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 23, 59, 59, 0, deadline.Location())
	if err = issues_model.NewMilestone(&issues_model.Milestone{
		RepoID:       ctx.Repo.Repository.ID,
		Name:         form.Title,
		Content:      form.Content,
		DeadlineUnix: timeutil.TimeStamp(deadline.Unix()),
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

	m, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", nil)
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

	if len(form.Deadline) == 0 {
		form.Deadline = "9999-12-31"
	}
	deadline, err := time.ParseInLocation("2006-01-02", form.Deadline, time.Local)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestoneNew, &form)
		return
	}

	deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 23, 59, 59, 0, deadline.Location())
	m, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetMilestoneByRepoID", err)
		}
		return
	}
	m.Name = form.Title
	m.Content = form.Content
	m.DeadlineUnix = timeutil.TimeStamp(deadline.Unix())
	if err = issues_model.UpdateMilestone(m, m.IsClosed); err != nil {
		ctx.ServerError("UpdateMilestone", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.milestones.edit_success", m.Name))
	ctx.Redirect(ctx.Repo.RepoLink + "/milestones")
}

// ChangeMilestoneStatus response for change a milestone's status
func ChangeMilestoneStatus(ctx *context.Context) {
	toClose := false
	switch ctx.Params(":action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.Redirect(ctx.Repo.RepoLink + "/milestones")
	}
	id := ctx.ParamsInt64(":id")

	if err := issues_model.ChangeMilestoneStatusByRepoIDAndID(ctx.Repo.Repository.ID, id, toClose); err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", err)
		} else {
			ctx.ServerError("ChangeMilestoneStatusByIDAndRepoID", err)
		}
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/milestones?state=" + url.QueryEscape(ctx.Params(":action")))
}

// DeleteMilestone delete a milestone
func DeleteMilestone(ctx *context.Context) {
	if err := issues_model.DeleteMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteMilestoneByRepoID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.milestones.deletion_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/milestones",
	})
}

// MilestoneIssuesAndPulls lists all the issues and pull requests of the milestone
func MilestoneIssuesAndPulls(ctx *context.Context) {
	milestoneID := ctx.ParamsInt64(":id")
	milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
	if err != nil {
		if issues_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("GetMilestoneByID", err)
			return
		}

		ctx.ServerError("GetMilestoneByID", err)
		return
	}

	milestone.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.Repo.RepoLink,
		Metas:     ctx.Repo.Repository.ComposeMetas(),
		GitRepo:   ctx.Repo.GitRepo,
		Ctx:       ctx,
	}, milestone.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.Data["Title"] = milestone.Name
	ctx.Data["Milestone"] = milestone

	issues(ctx, milestoneID, 0, util.OptionalBoolNone)

	ret, _ := issue.GetTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["NewIssueChooseTemplate"] = len(ret) > 0

	ctx.Data["CanWriteIssues"] = ctx.Repo.CanWriteIssuesOrPulls(false)
	ctx.Data["CanWritePulls"] = ctx.Repo.CanWriteIssuesOrPulls(true)

	ctx.HTML(http.StatusOK, tplMilestoneIssues)
}
