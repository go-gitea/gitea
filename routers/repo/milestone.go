// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
	"github.com/Unknwon/paginater"
)

const (
	tplMilestoneIssues base.TplName = "repo/issue/milestone_issues"
)

// MilestoneIssuesAndPulls lists all the issues and pull requests of the milestone
func MilestoneIssuesAndPulls(ctx *context.Context) {
	milestoneID := ctx.ParamsInt64(":id")
	milestone, err := models.GetMilestoneByID(milestoneID)
	if err != nil {
		ctx.ServerError("GetMilestoneByID", err)
		return
	}

	ctx.Data["Title"] = milestone.Name
	ctx.Data["Milestone"] = milestone

	viewType := ctx.Query("type")
	sortType := ctx.Query("sort")
	types := []string{"all", "your_repositories", "assigned", "created_by", "mentioned"}
	if !com.IsSliceContainsStr(types, viewType) {
		viewType = "all"
	}

	var (
		assigneeID  = ctx.QueryInt64("assignee")
		posterID    int64
		mentionedID int64
		forceEmpty  bool
	)

	if ctx.IsSigned {
		switch viewType {
		case "created_by":
			posterID = ctx.User.ID
		case "mentioned":
			mentionedID = ctx.User.ID
		}
	}

	repo := ctx.Repo.Repository
	selectLabels := ctx.Query("labels")

	isShowClosed := ctx.Query("state") == "closed"

	keyword := strings.Trim(ctx.Query("q"), " ")
	if bytes.Contains([]byte(keyword), []byte{0x00}) {
		keyword = ""
	}

	var issueIDs []int64
	if len(keyword) > 0 {
		issueIDs, err = indexer.SearchIssuesByKeyword(repo.ID, keyword)
		if len(issueIDs) == 0 {
			forceEmpty = true
		}
	}

	var issueStats *models.IssueStats
	if forceEmpty {
		issueStats = &models.IssueStats{}
	} else {
		issueStats, err = models.GetIssueStats(&models.IssueStatsOptions{
			RepoID:      repo.ID,
			Labels:      selectLabels,
			MilestoneID: milestoneID,
			AssigneeID:  assigneeID,
			MentionedID: mentionedID,
			PosterID:    posterID,
			IssueIDs:    issueIDs,
		})
		if err != nil {
			ctx.ServerError("GetIssueStats", err)
			return
		}
	}
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	if !isShowClosed {
		total = int(issueStats.OpenCount)
	} else {
		total = int(issueStats.ClosedCount)
	}
	pager := paginater.New(total, setting.UI.IssuePagingNum, page, 5)
	ctx.Data["Page"] = pager

	var issues []*models.Issue
	if forceEmpty {
		issues = []*models.Issue{}
	} else {
		issues, err = models.Issues(&models.IssuesOptions{
			RepoIDs:     []int64{repo.ID},
			AssigneeID:  assigneeID,
			PosterID:    posterID,
			MentionedID: mentionedID,
			MilestoneID: milestoneID,
			Page:        pager.Current(),
			PageSize:    setting.UI.IssuePagingNum,
			IsClosed:    util.OptionalBoolOf(isShowClosed),
			Labels:      selectLabels,
			SortType:    sortType,
			IssueIDs:    issueIDs,
		})
		if err != nil {
			ctx.ServerError("Issues", err)
			return
		}
	}

	// Get posters.
	for i := range issues {
		// Check read status
		if !ctx.IsSigned {
			issues[i].IsRead = true
		} else if err = issues[i].GetIsRead(ctx.User.ID); err != nil {
			ctx.ServerError("GetIsRead", err)
			return
		}
	}
	ctx.Data["Issues"] = issues

	// Get assignees.
	ctx.Data["Assignees"], err = repo.GetAssignees()
	if err != nil {
		ctx.ServerError("GetAssignees", err)
		return
	}

	if ctx.QueryInt64("assignee") == 0 {
		assigneeID = 0 // Reset ID to prevent unexpected selection of assignee.
	}

	ctx.Data["IssueStats"] = issueStats
	ctx.Data["SelectLabels"] = com.StrTo(selectLabels).MustInt64()
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["Keyword"] = keyword
	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	ctx.HTML(200, tplMilestoneIssues)
}
