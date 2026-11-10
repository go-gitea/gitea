// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/url"
	"strconv"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	issue_indexer "gitea.dev/modules/indexer/issues"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"
)

const dashboardOverviewListSize = 5

type dashboardOverviewList struct {
	Title  string // locale key
	Icon   string
	Link   string
	Issues issues_model.IssueList
	Total  int64
}

func dashboardTab(ctx *context.Context) (string, error) {
	if tab := ctx.FormString("tab"); setting.IsValidDashboardTab(tab) {
		return tab, nil
	}
	if ctx.FormString("date") != "" { // heatmap clicks filter the activity feed
		return setting.DashboardTabActivity, nil
	}
	return user_model.GetUserDashboardTab(ctx, ctx.Doer.ID)
}

// dashboardRepoIDs returns the repos of the dashboard's user, org or team that have the unit enabled
func dashboardRepoIDs(ctx *context.Context, ctxUser *user_model.User, unitType unit.Type) ([]int64, error) {
	repoOpts := repo_model.SearchRepoOptions{
		Actor:       ctx.Doer,
		OwnerID:     ctxUser.ID,
		Private:     true,
		Collaborate: optional.None[bool](),
		UnitType:    unitType,
		Archived:    optional.Some(false),
	}
	if ctx.Org.Team != nil {
		repoOpts.TeamID = ctx.Org.Team.ID
	}
	ids, _, err := repo_model.SearchRepositoryIDs(ctx, repoOpts)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		ids = []int64{0} // don't let the indexer return all repos
	}
	return ids, nil
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

func makeApprovalCountsFunc(approvalCounts map[int64][]*issues_model.ReviewCount) func(issueID int64, typ string) int64 {
	return func(issueID int64, typ string) int64 {
		reviewTyp := issues_model.ReviewTypeApprove
		switch typ {
		case "reject":
			reviewTyp = issues_model.ReviewTypeReject
		case "waiting":
			reviewTyp = issues_model.ReviewTypeRequest
		}
		for _, count := range approvalCounts[issueID] {
			if count.Type == reviewTyp {
				return count.Count
			}
		}
		return 0
	}
}

func prepareDashboardOverview(ctx *context.Context, ctxUser *user_model.User) {
	type listSpec struct {
		unitType   unit.Type
		filterMode int
		viewType   string
		title      string
		icon       string
	}
	specs := []listSpec{
		{unit.TypePullRequests, issues_model.FilterModeReviewRequested, "review_requested", "home.overview.review_requested", "octicon-eye"},
		{unit.TypePullRequests, issues_model.FilterModeCreate, "created_by", "home.overview.your_pull_requests", "octicon-git-pull-request"},
		{unit.TypeIssues, issues_model.FilterModeAssign, "assigned", "home.overview.assigned_issues", "octicon-issue-opened"},
	}

	linkPrefix := setting.AppSubURL
	linkSuffix := ""
	if ctxUser.IsOrganization() {
		linkPrefix = ctxUser.OrganisationLink()
		if ctx.Org.Team != nil {
			linkSuffix = "/" + url.PathEscape(ctx.Org.Team.Name)
		}
	}

	repoIDsByUnit := map[unit.Type][]int64{}
	var lists []*dashboardOverviewList
	var all, pulls issues_model.IssueList
	for _, spec := range specs {
		if spec.unitType.UnitGlobalDisabled() {
			continue
		}
		repoIDs, ok := repoIDsByUnit[spec.unitType]
		if !ok {
			var err error
			if repoIDs, err = dashboardRepoIDs(ctx, ctxUser, spec.unitType); err != nil {
				ctx.ServerError("dashboardRepoIDs", err)
				return
			}
			repoIDsByUnit[spec.unitType] = repoIDs
		}

		isPull := spec.unitType == unit.TypePullRequests
		opts := &issues_model.IssuesOptions{
			IsPull:     optional.Some(isPull),
			IsClosed:   optional.Some(false),
			IsArchived: optional.Some(false),
			SortType:   "recentupdate",
			Doer:       ctx.Doer,
			RepoIDs:    repoIDs,
			AllPublic:  ctx.Doer.ID == ctxUser.ID, // same scope as the issue list's filters
			Paginator:  &db.ListOptions{Page: 1, PageSize: dashboardOverviewListSize},
		}
		applyIssueFilterMode(opts, spec.filterMode, ctx.Doer.ID)
		issueIDs, total, err := issue_indexer.SearchIssues(ctx, issue_indexer.ToSearchOptions("", opts))
		if err != nil {
			ctx.ServerError("SearchIssues", err)
			return
		}
		issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs, true)
		if err != nil {
			ctx.ServerError("GetIssuesByIDs", err)
			return
		}
		all = append(all, issues...)
		if isPull {
			pulls = append(pulls, issues...)
		}
		page := util.Iif(isPull, "/pulls", "/issues")
		lists = append(lists, &dashboardOverviewList{
			Title:  spec.title,
			Icon:   spec.icon,
			Link:   linkPrefix + page + linkSuffix + "?type=" + spec.viewType,
			Issues: issues,
			Total:  total,
		})
	}
	if _, err := all.LoadRepositories(ctx); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}
	ctx.Data["OverviewLists"] = lists

	if len(pulls) > 0 {
		commitStatuses, lastStatus, err := pull_service.GetIssuesAllCommitStatus(ctx, ctx.Doer, pulls)
		if err != nil {
			ctx.ServerError("GetIssuesAllCommitStatus", err)
			return
		}
		approvalCounts, err := pulls.GetApprovalCounts(ctx)
		if err != nil {
			ctx.ServerError("GetApprovalCounts", err)
			return
		}
		ctx.Data["CommitStatuses"] = commitStatuses
		ctx.Data["CommitLastStatus"] = lastStatus
		ctx.Data["ApprovalCounts"] = makeApprovalCountsFunc(approvalCounts)
	}

	feeds, _, err := activities_model.GetFeeds(ctx, activities_model.GetFeedsOptions{
		RequestedUser:  ctxUser,
		RequestedTeam:  ctx.Org.Team,
		Actor:          ctx.Doer,
		IncludePrivate: true,
		DontCount:      true,
		ListOptions:    db.ListOptions{Page: 1, PageSize: dashboardOverviewListSize},
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}
	ctx.Data["Feeds"] = feeds
}
