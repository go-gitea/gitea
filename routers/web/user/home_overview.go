// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/url"
	"strconv"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/routers/web/shared/issue"
	"gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"

	"xorm.io/builder"
)

const dashboardOverviewListSize = 5

type dashboardOverviewList struct {
	Title  string
	Link   string
	Issues issues_model.IssueList
	Total  int64
}

func dashboardTab(ctx *context.Context) (string, error) {
	if (unit.TypeIssues.UnitGlobalDisabled() && unit.TypePullRequests.UnitGlobalDisabled()) || ctx.FormString("date") != "" || ctx.FormString("page") != "" { // Overview needs issues or pulls, feed filters and pagination only exist on Activity
		return user_model.DashboardTabActivity, nil
	}
	tab, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyDashboardTab)
	return util.Iif(tab == user_model.DashboardTabActivity, tab, user_model.DashboardTabOverview), err
}

func dashboardOverviewRepoCond(ctx *context.Context, ctxUser *user_model.User, unitType unit.Type) builder.Cond {
	repoOpts := repo_model.SearchRepoOptions{
		OwnerID:   ctxUser.ID,
		Private:   true,
		AllPublic: !ctxUser.IsOrganization(),
		UnitType:  unitType,
		Archived:  optional.Some(false),
	}
	if ctxUser.IsOrganization() {
		repoOpts.Collaborate = optional.Some(false)
	}
	if ctx.Org.Team != nil {
		repoOpts.TeamID = ctx.Org.Team.ID
	}
	cond := repo_model.SearchRepositoryCondition(repoOpts).And(
		builder.In("repository.id", builder.Select("repo_id").From("repo_unit").Where(builder.Eq{"type": unitType})),
	)
	if !user_model.DoerIsSiteAdminFullAccess(ctx.Doer) {
		cond = cond.And(repo_model.AccessibleRepositoryCondition(ctx.Doer, unitType))
	}
	return builder.In("issue.repo_id", builder.Select("id").From("repository").Where(cond))
}

func issueFilterMode(viewType string) (string, int) {
	switch viewType {
	case "assigned":
		return viewType, issues_model.FilterModeAssign
	case "created_by":
		return viewType, issues_model.FilterModeCreate
	case "mentioned":
		return viewType, issues_model.FilterModeMention
	case "review_requested":
		return viewType, issues_model.FilterModeReviewRequested
	case "reviewed_by":
		return viewType, issues_model.FilterModeReviewed
	}
	return "your_repositories", issues_model.FilterModeYourRepositories
}

func applyIssueFilterMode(opts *issues_model.IssuesOptions, filterMode int, doerID int64) {
	switch filterMode {
	case issues_model.FilterModeAssign:
		opts.AssigneeID = strconv.FormatInt(doerID, 10)
	case issues_model.FilterModeCreate:
		opts.PosterID = strconv.FormatInt(doerID, 10)
	case issues_model.FilterModeMention:
		opts.MentionedID = doerID
	case issues_model.FilterModeReviewRequested:
		opts.ReviewRequestedID = doerID
	case issues_model.FilterModeReviewed:
		opts.ReviewedID = doerID
	}
}

func prepareDashboardOverview(ctx *context.Context, ctxUser *user_model.User) {
	type listSpec struct {
		unitType unit.Type
		viewType string
		title    string
	}
	specs := []listSpec{
		{unit.TypePullRequests, "review_requested", "home.overview.review_requested"},
		{unit.TypePullRequests, "created_by", "home.overview.your_pull_requests"},
		{unit.TypeIssues, "assigned", "home.overview.assigned_issues"},
	}

	linkPrefix := setting.AppSubURL
	linkSuffix := ""
	if ctxUser.IsOrganization() {
		linkPrefix = ctxUser.OrganisationLink()
		if ctx.Org.Team != nil {
			linkSuffix = "/" + url.PathEscape(ctx.Org.Team.Name)
		}
	}

	var lists []*dashboardOverviewList
	var all issues_model.IssueList
	for _, spec := range specs {
		if spec.unitType.UnitGlobalDisabled() {
			continue
		}
		isPull := spec.unitType == unit.TypePullRequests
		opts := &issues_model.IssuesOptions{
			IsPull:    optional.Some(isPull),
			IsClosed:  optional.Some(false),
			SortType:  "recentupdate",
			RepoCond:  dashboardOverviewRepoCond(ctx, ctxUser, spec.unitType),
			Paginator: &db.ListOptions{Page: 1, PageSize: dashboardOverviewListSize},
		}
		_, filterMode := issueFilterMode(spec.viewType)
		applyIssueFilterMode(opts, filterMode, ctx.Doer.ID)
		issueIDs, total, err := issues_model.IssueIDs(ctx, opts)
		if err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs, true)
		if err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		all = append(all, issues...)
		lists = append(lists, &dashboardOverviewList{
			Title:  spec.title,
			Link:   linkPrefix + util.Iif(isPull, "/pulls", "/issues") + linkSuffix + "?type=" + spec.viewType,
			Issues: issues,
			Total:  total,
		})
	}
	if err := all.LoadPosters(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	commitStatuses, lastStatus, err := pull_service.GetIssuesAllCommitStatus(ctx, ctx.Doer, all) // also loads the repositories
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	approvalCounts, err := all.GetApprovalCounts(ctx)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["OverviewLists"] = lists
	ctx.Data["CommitStatuses"] = commitStatuses
	ctx.Data["CommitLastStatus"] = lastStatus
	ctx.Data["ApprovalCounts"] = issue.ApprovalCountsFunc(approvalCounts)
}
