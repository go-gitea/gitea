// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	milestone_model "code.gitea.io/gitea/models/milestone"
	org_model "code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	repo "code.gitea.io/gitea/routers/web/repo"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/forms"
	pull_service "code.gitea.io/gitea/services/pull"
)

const (
	tplMilestones     base.TplName = "org/milestones/list"
	tplMilestonesNew  base.TplName = "org/milestones/new"
	tplMilestonesView base.TplName = "org/milestones/view"
)

// Milestones renders the home page of milestones
func Milestones(ctx *context.Context) {
	shared_user.PrepareContextForProfileBigAvatar(ctx)
	canWriteMilestone(ctx)
	ctx.Data["Title"] = ctx.Tr("Milestones")

	sortType := ctx.FormTrim("sort")

	isShowClosed := strings.ToLower(ctx.FormTrim("state")) == "closed"
	keyword := ctx.FormTrim("q")
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	milestones, err := db.Find[milestone_model.Milestone](ctx, milestone_model.FindMilestoneOptions{
		OrgID: ctx.Org.Organization.ID,
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.IssuePagingNum,
		},
		IsClosed: util.OptionalBoolOf(isShowClosed),
		SortType: sortType,
		Name:     keyword,
		Type:     milestone_model.MilestoneTypeOrganization,
	})
	if err != nil {
		ctx.ServerError("FindMilestones", err)
		return
	}

	total, err := db.Count[milestone_model.Milestone](ctx, milestone_model.FindMilestoneOptions{
		OrgID:    ctx.Org.Organization.ID,
		IsClosed: util.OptionalBoolOf(isShowClosed),
		Type:     milestone_model.MilestoneTypeOrganization,
	})
	if err != nil {
		ctx.ServerError("CountMilestones", err)
		return
	}
	opTotal, err := db.Count[milestone_model.Milestone](ctx, milestone_model.FindMilestoneOptions{
		OrgID:    ctx.Org.Organization.ID,
		IsClosed: util.OptionalBoolOf(!isShowClosed),
		Type:     milestone_model.MilestoneTypeOrganization,
	})
	if err != nil {
		ctx.ServerError("CountMilestones", err)
		return
	}

	if isShowClosed {
		ctx.Data["OpenCount"] = opTotal
		ctx.Data["ClosedCount"] = total

	} else {
		ctx.Data["OpenCount"] = total
		ctx.Data["ClosedCount"] = opTotal
	}
	linkStr := "%s/milestones?state=%s&q=%s&sort=%s"
	ctx.Data["OpenLink"] = fmt.Sprintf(linkStr, ctx.Org.Organization.HomeLink()+"/-", "open",
		url.QueryEscape(keyword), url.QueryEscape(sortType))
	ctx.Data["ClosedLink"] = fmt.Sprintf(linkStr, ctx.Org.Organization.HomeLink()+"/-", "closed",
		url.QueryEscape(keyword), url.QueryEscape(sortType))

	ctx.Data["Milestones"] = milestones
	shared_user.RenderUserHeader(ctx)

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	for _, milestone := range milestones {
		milestone.RenderedContent = milestone.Name
	}

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	numPages := 0
	if total > 0 {
		numPages = (int(total) - 1/setting.UI.IssuePagingNum)
	}

	pager := context.NewPagination(int(total), setting.UI.IssuePagingNum, page, numPages)
	pager.AddParam(ctx, "state", "State")
	ctx.Data["Page"] = pager

	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["PageIsViewMilestones"] = true
	ctx.Data["SortType"] = sortType

	ctx.HTML(http.StatusOK, tplMilestones)
}

// RenderNewMilestone render creating a milestone page
func RenderNewMilestone(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("milestones.new")
	ctx.Data["PageIsViewMilestones"] = true
	ctx.Data["HomeLink"] = ctx.ContextUser.HomeLink()
	ctx.Data["CancelLink"] = ctx.ContextUser.HomeLink() + "/-/milestones"
	shared_user.RenderUserHeader(ctx)

	err := shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.HTML(http.StatusOK, tplMilestonesNew)
}

// NewMilestonePost creates a new milestone
func NewMilestonePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateMilestoneForm)
	ctx.Data["Title"] = ctx.Tr("milestones.new")
	shared_user.RenderUserHeader(ctx)

	if ctx.HasError() {
		RenderNewMilestone(ctx)
		return
	}

	if len(form.Deadline) == 0 {
		form.Deadline = "9999-12-31"
	}
	deadline, err := time.ParseInLocation("2006-01-02", form.Deadline, time.Local)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestonesNew, &form)
		return
	}

	deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 23, 59, 59, 0, deadline.Location())

	newMilestone := milestone_model.Milestone{
		OrgID:        ctx.Org.Organization.ID,
		Name:         form.Title,
		Content:      form.Content,
		DeadlineUnix: timeutil.TimeStamp(deadline.Unix()),
		Type:         milestone_model.MilestoneTypeOrganization,
	}

	if err := milestone_model.NewMilestone(ctx, &newMilestone); err != nil {
		ctx.ServerError("NewMilestone", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.milestones.create_success", form.Title))
	ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/milestones")
}

// ChangeMilestoneStatus updates the status of a milestone between "open" and "close"
func ChangeMilestoneStatus(ctx *context.Context) {
	toClose := false
	switch ctx.Params(":action") {
	case "open":
		toClose = false
	case "close":
		toClose = true
	default:
		ctx.Redirect(ctx.Org.Organization.HomeLink() + "/-/milestones")
	}
	id := ctx.ParamsInt64(":id")

	milestone, err := milestone_model.GetMilestoneByID(ctx, id)
	if err != nil {
		ctx.ServerError("GetMilestoneByID", err)
		return
	}

	if err := milestone_model.ChangeMilestoneStatusByOrgIDAndID(ctx, milestone.OrgID, id, toClose); err != nil {
		if milestone_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", err)
		} else {
			ctx.ServerError("ChangeMilestoneStatusByOrgIDAndID", err)
		}
		return
	}

	ctx.JSONRedirect(ctx.Org.Organization.HomeLink() + "/-/milestones?state=" + url.QueryEscape(ctx.Params(":action")))
}

// DeleteMilestone delete a milestone
func DeleteMilestone(ctx *context.Context) {
	milestone, err := milestone_model.GetMilestoneByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if milestone_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetMilestoneByID", err)
		}
		return
	}

	if err := milestone_model.DeleteMilestoneByOrgID(ctx, milestone.OrgID, milestone.ID); err != nil {
		ctx.Flash.Error("DeleteMilestoneByOrgID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.milestone.deletion_success"))
	}

	ctx.JSONRedirect(ctx.Org.Organization.HomeLink() + "/-/milestones")
}

// EditMilestone allows a milestone to be edited
func EditMilestone(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.projects.edit")
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["PageIsEditMilestone"] = true
	canWriteMilestone(ctx)
	shared_user.RenderUserHeader(ctx)

	milestone, err := milestone_model.GetMilestoneByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if milestone_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetMilestoneByID", err)
		}
		return
	}

	ctx.Data["title"] = milestone.Name
	ctx.Data["content"] = milestone.Content
	if len(milestone.DeadlineString) > 0 {
		ctx.Data["deadline"] = milestone.DeadlineString
	}
	ctx.HTML(http.StatusOK, tplMilestonesNew)
}

// EditMilestonePost response for editing a milestone
func EditMilestonePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateMilestoneForm)
	ctx.Data["Title"] = ctx.Tr("repo.milestones.edit")
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["PageIsEditMilestone"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplMilestonesNew)
		return
	}

	if len(form.Deadline) == 0 {
		form.Deadline = "9999-12-31"
	}
	deadline, err := time.ParseInLocation("2006-01-02", form.Deadline, time.Local)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestonesNew, &form)
		return
	}

	deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 23, 59, 59, 0, deadline.Location())
	m, err := milestone_model.GetMilestoneByOrgID(ctx, ctx.Org.Organization.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if milestone_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetMilestoneByOrgID", err)
		}
		return
	}
	m.Name = form.Title
	m.Content = form.Content
	m.DeadlineUnix = timeutil.TimeStamp(deadline.Unix())
	if err = milestone_model.UpdateMilestone(ctx, m, m.IsClosed); err != nil {
		ctx.ServerError("UpdateMilestone", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.milestones.edit_success", m.Name))
	ctx.Redirect(ctx.Org.Organization.HomeLink() + "/-/milestones")
}

// ViewMilestone renders the issues for a milestone
func ViewMilestone(ctx *context.Context) {
	shared_user.PrepareContextForProfileBigAvatar(ctx)
	canWriteMilestone(ctx)
	milestoneID := ctx.ParamsInt64(":id")

	milestone, err := milestone_model.GetMilestoneByOrgID(ctx, ctx.Org.Organization.ID, milestoneID)
	if err != nil {
		if milestone_model.IsErrMilestoneNotExist(err) {
			ctx.NotFound("", nil)
		} else {
			ctx.ServerError("GetMilestoneByOrgID", err)
		}
		return
	}

	metas := map[string]string{
		"org":     ctx.Org.Organization.FullName,
		"orgPath": ctx.Org.OrgLink,
	}

	milestone.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
		Links: markup.Links{
			Base: ctx.Org.OrgLink,
		},
		Metas: metas,
		Ctx:   ctx,
	}, milestone.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.Data["Title"] = milestone.Name
	ctx.Data["Milestone"] = milestone

	issues(ctx, milestoneID, util.OptionalBoolNone)

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.Data["CanWriteIssues"] = ctx.Repo.CanWriteIssuesOrPulls(false)
	ctx.Data["CanWritePulls"] = ctx.Repo.CanWriteIssuesOrPulls(true)
	ctx.Data["PageIsOrgMilestone"] = true

	ctx.HTML(http.StatusOK, tplMilestonesView)
}

func canWriteMilestone(ctx *context.Context) {
	org := ctx.Org.Organization
	canCreateMilestone, err := org.CanCreateOrgRepo(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("CanCreateOrgRepo", err)
		return
	}
	ctx.Data["CanWriteMilestone"] = canCreateMilestone
}

func issues(ctx *context.Context, milestoneID int64, isPullOption util.OptionalBool) {
	var err error
	viewType := ctx.FormString("type")
	sortType := ctx.FormString("sort")
	types := []string{"all", "your_repositories", "assigned", "created_by", "mentioned", "review_requested", "reviewed_by"}
	if !util.SliceContainsString(types, viewType, true) {
		viewType = "all"
	}

	var (
		assigneeID        = ctx.FormInt64("assignee")
		posterID          = ctx.FormInt64("poster")
		projectID         = ctx.FormInt64("project")
		mentionedID       int64
		reviewRequestedID int64
		reviewedID        int64
	)

	if ctx.IsSigned {
		switch viewType {
		case "created_by":
			posterID = ctx.Doer.ID
		case "mentioned":
			mentionedID = ctx.Doer.ID
		case "assigned":
			assigneeID = ctx.Doer.ID
		case "review_requested":
			reviewRequestedID = ctx.Doer.ID
		case "reviewed_by":
			reviewedID = ctx.Doer.ID
		}
	}

	// org := ctx.Org.Organization.ID
	var labelIDs []int64
	// 1,-2 means including label 1 and excluding label 2
	// 0 means issues with no label
	// blank means labels will not be filtered for issues
	selectLabels := ctx.FormString("labels")
	if selectLabels == "" {
		ctx.Data["AllLabels"] = true
	} else if selectLabels == "0" {
		ctx.Data["NoLabel"] = true
	}
	if len(selectLabels) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(selectLabels, ","))
		if err != nil {
			ctx.ServerError("StringsToInt64s", err)
			return
		}
	}

	keyword := strings.Trim(ctx.FormString("q"), " ")
	if bytes.Contains([]byte(keyword), []byte{0x00}) {
		keyword = ""
	}

	var mileIDs []int64
	if milestoneID > 0 || milestoneID == db.NoConditionID { // -1 to get those issues which have no any milestone assigned
		mileIDs = []int64{milestoneID}
	}

	var issueStats *issues_model.IssueStats
	statsOpts := &issues_model.IssuesOptions{
		Org:               ctx.Org.Organization,
		LabelIDs:          labelIDs,
		MilestoneIDs:      mileIDs,
		ProjectID:         projectID,
		AssigneeID:        assigneeID,
		MentionedID:       mentionedID,
		PosterID:          posterID,
		ReviewRequestedID: reviewRequestedID,
		ReviewedID:        reviewedID,
		IsPull:            isPullOption,
		IssueIDs:          nil,
	}
	if keyword != "" {
		allIssueIDs, err := issueIDsFromSearch(ctx, keyword, statsOpts)
		if err != nil {
			if issue_indexer.IsAvailable(ctx) {
				ctx.ServerError("issueIDsFromSearch", err)
				return
			}
			ctx.Data["IssueIndexerUnavailable"] = true
			return
		}
		statsOpts.IssueIDs = allIssueIDs
	}
	if keyword != "" && len(statsOpts.IssueIDs) == 0 {
		// So it did search with the keyword, but no issue found.
		// Just set issueStats to empty.
		issueStats = &issues_model.IssueStats{}
	} else {
		// So it did search with the keyword, and found some issues. It needs to get issueStats of these issues.
		// Or the keyword is empty, so it doesn't need issueIDs as filter, just get issueStats with statsOpts.
		issueStats, err = issues_model.GetIssueStats(ctx, statsOpts)
		if err != nil {
			ctx.ServerError("GetIssueStats", err)
			return
		}
	}

	var isShowClosed util.OptionalBool
	switch ctx.FormString("state") {
	case "closed":
		isShowClosed = util.OptionalBoolTrue
	case "all":
		isShowClosed = util.OptionalBoolNone
	default:
		isShowClosed = util.OptionalBoolFalse
	}
	// if there are closed issues and no open issues, default to showing all issues
	if len(ctx.FormString("state")) == 0 && issueStats.OpenCount == 0 && issueStats.ClosedCount != 0 {
		isShowClosed = util.OptionalBoolNone
	}

	archived := ctx.FormBool("archived")

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	switch isShowClosed {
	case util.OptionalBoolTrue:
		total = int(issueStats.ClosedCount)
	case util.OptionalBoolNone:
		total = int(issueStats.OpenCount + issueStats.ClosedCount)
	default:
		total = int(issueStats.OpenCount)
	}
	pager := context.NewPagination(total, setting.UI.IssuePagingNum, page, 5)

	var issues issues_model.IssueList
	{
		ids, err := issueIDsFromSearch(ctx, keyword, &issues_model.IssuesOptions{
			Paginator: &db.ListOptions{
				Page:     pager.Paginater.Current(),
				PageSize: setting.UI.IssuePagingNum,
			},
			Org:               ctx.Org.Organization,
			AssigneeID:        assigneeID,
			PosterID:          posterID,
			MentionedID:       mentionedID,
			ReviewRequestedID: reviewRequestedID,
			ReviewedID:        reviewedID,
			MilestoneIDs:      mileIDs,
			ProjectID:         projectID,
			IsClosed:          isShowClosed,
			IsPull:            isPullOption,
			LabelIDs:          labelIDs,
			SortType:          sortType,
		})
		if err != nil {
			if issue_indexer.IsAvailable(ctx) {
				ctx.ServerError("issueIDsFromSearch", err)
				return
			}
			ctx.Data["IssueIndexerUnavailable"] = true
			return
		}
		issues, err = issues_model.GetIssuesByIDs(ctx, ids, true)
		if err != nil {
			ctx.ServerError("GetIssuesByIDs", err)
			return
		}
	}

	approvalCounts, err := issues.GetApprovalCounts(ctx)
	if err != nil {
		ctx.ServerError("ApprovalCounts", err)
		return
	}

	// Get posters.
	for i := range issues {
		// Check read status
		if !ctx.IsSigned {
			issues[i].IsRead = true
		} else if err = issues[i].GetIsRead(ctx, ctx.Doer.ID); err != nil {
			ctx.ServerError("GetIsRead", err)
			return
		}
	}

	commitStatuses, lastStatus, err := pull_service.GetIssuesAllCommitStatus(ctx, issues)
	if err != nil {
		ctx.ServerError("GetIssuesAllCommitStatus", err)
		return
	}

	if err := issues.LoadAttributes(ctx); err != nil {
		ctx.ServerError("issues.LoadAttributes", err)
		return
	}

	ctx.Data["Issues"] = issues
	ctx.Data["CommitLastStatus"] = lastStatus
	ctx.Data["CommitStatuses"] = commitStatuses

	// Get assignees.
	orgUsers, err := org_model.GetOrgUsersByOrgID(ctx, &org_model.FindOrgMembersOpts{
		ListOptions: db.ListOptions{},
		OrgID:       ctx.Org.Organization.ID,
	})
	if err != nil {
		ctx.ServerError("GetOrgUsersByOrgID", err)
		return
	}

	var assigneeIDs []int64

	for _, id := range orgUsers {
		assigneeIDs = append(assigneeIDs, id.UID)
	}

	assigneeUsers, err := user_model.GetUsersByIDs(ctx, assigneeIDs)

	if err != nil {
		ctx.ServerError("GetUsersByIDs", err)
		return
	}

	ctx.Data["Assignees"] = repo.MakeSelfOnTop(ctx.Doer, assigneeUsers)

	labels, err := issues_model.GetLabelsByOrgID(ctx, ctx.Org.Organization.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByOrgID", err)
		return
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

	if ctx.FormInt64("assignee") == 0 {
		assigneeID = 0 // Reset ID to prevent unexpected selection of assignee.
	}

	ctx.Data["ApprovalCounts"] = func(issueID int64, typ string) int64 {
		counts, ok := approvalCounts[issueID]
		if !ok || len(counts) == 0 {
			return 0
		}
		reviewTyp := issues_model.ReviewTypeApprove
		if typ == "reject" {
			reviewTyp = issues_model.ReviewTypeReject
		} else if typ == "waiting" {
			reviewTyp = issues_model.ReviewTypeRequest
		}
		for _, count := range counts {
			if count.Type == reviewTyp {
				return count.Count
			}
		}
		return 0
	}

	retrieveProjects(ctx, ctx.Org.Organization)
	if ctx.Written() {
		return
	}

	ctx.Data["IssueStats"] = issueStats
	ctx.Data["OpenCount"] = issueStats.OpenCount
	ctx.Data["ClosedCount"] = issueStats.ClosedCount
	linkStr := "%s?q=%s&type=%s&ssort=%s&state=%s&labels=%s&milestone=%d&project=%d&assignee=%d&poster=%d&archived=%t"
	ctx.Data["AllStatesLink"] = fmt.Sprintf(linkStr, ctx.Link,
		url.QueryEscape(keyword), url.QueryEscape(viewType), url.QueryEscape(sortType), "all", url.QueryEscape(selectLabels),
		mentionedID, projectID, assigneeID, posterID, archived)
	ctx.Data["OpenLink"] = fmt.Sprintf(linkStr, ctx.Link,
		url.QueryEscape(keyword), url.QueryEscape(viewType), url.QueryEscape(sortType), "open", url.QueryEscape(selectLabels),
		mentionedID, projectID, assigneeID, posterID, archived)
	ctx.Data["ClosedLink"] = fmt.Sprintf(linkStr, ctx.Link,
		url.QueryEscape(keyword), url.QueryEscape(viewType), url.QueryEscape(sortType), "closed", url.QueryEscape(selectLabels),
		mentionedID, projectID, assigneeID, posterID, archived)
	ctx.Data["SelLabelIDs"] = labelIDs
	ctx.Data["SelectLabels"] = selectLabels
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["ProjectID"] = projectID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["PosterID"] = posterID
	ctx.Data["Keyword"] = keyword
	switch isShowClosed {
	case util.OptionalBoolTrue:
		ctx.Data["State"] = "closed"
	case util.OptionalBoolNone:
		ctx.Data["State"] = "all"
	default:
		ctx.Data["State"] = "open"
	}
	ctx.Data["ShowArchivedLabels"] = archived

	pager.AddParam(ctx, "q", "Keyword")
	pager.AddParam(ctx, "type", "ViewType")
	pager.AddParam(ctx, "sort", "SortType")
	pager.AddParam(ctx, "state", "State")
	pager.AddParam(ctx, "labels", "SelectLabels")
	pager.AddParam(ctx, "milestone", "MilestoneID")
	pager.AddParam(ctx, "project", "ProjectID")
	pager.AddParam(ctx, "assignee", "AssigneeID")
	pager.AddParam(ctx, "poster", "PosterID")
	pager.AddParam(ctx, "archived", "ShowArchivedLabels")

	ctx.Data["Page"] = pager
}

func issueIDsFromSearch(ctx *context.Context, keyword string, opts *issues_model.IssuesOptions) ([]int64, error) {
	ids, _, err := issue_indexer.SearchIssues(ctx, issue_indexer.ToSearchOptions(keyword, opts))
	if err != nil {
		return nil, fmt.Errorf("SearchIssues: %w", err)
	}
	return ids, nil
}

func MilestonePoster(ctx *context.Context) {
	org := ctx.Org.Organization
	search := strings.TrimSpace(ctx.FormString("q"))
	posters, err := org_model.GetOrgPostersWithSearch(ctx, org, search, setting.UI.DefaultShowFullName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	if search == "" && ctx.Doer != nil {
		// the returned posters slice only contains limited number of users,
		// to make the current user (doer) can quickly filter their own issues, always add doer to the posters slice
		if !slices.ContainsFunc(posters, func(user *user_model.User) bool { return user.ID == ctx.Doer.ID }) {
			posters = append(posters, ctx.Doer)
		}
	}

	posters = repo.MakeSelfOnTop(ctx.Doer, posters)

	type userSearchInfo struct {
		UserID     int64  `json:"user_id"`
		UserName   string `json:"username"`
		AvatarLink string `json:"avatar_link"`
		FullName   string `json:"full_name"`
	}

	type userSearchResponse struct {
		Results []*userSearchInfo `json:"results"`
	}

	resp := &userSearchResponse{}
	resp.Results = make([]*userSearchInfo, len(posters))
	for i, user := range posters {
		resp.Results[i] = &userSearchInfo{UserID: user.ID, UserName: user.Name, AvatarLink: user.AvatarLink(ctx)}
		if setting.UI.DefaultShowFullName {
			resp.Results[i].FullName = user.FullName
		}
	}
	ctx.JSON(http.StatusOK, resp)
}

func retrieveProjects(ctx *context.Context, org *org_model.Organization) {
	var err error
	projects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptionsAll,
		OwnerID:     org.ID,
		IsClosed:    util.OptionalBoolFalse,
		Type:        project_model.TypeOrganization,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	ctx.Data["OpenProjects"] = projects

	projects, err = db.Find[project_model.Project](ctx, project_model.SearchOptions{
		ListOptions: db.ListOptionsAll,
		OwnerID:     org.ID,
		IsClosed:    util.OptionalBoolTrue,
		Type:        project_model.TypeOrganization,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	ctx.Data["ClosedProjects"] = projects
}
