// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/feed"
	context_service "code.gitea.io/gitea/services/context"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
	"xorm.io/builder"
)

const (
	tplDashboard  base.TplName = "user/dashboard/dashboard"
	tplIssues     base.TplName = "user/dashboard/issues"
	tplMilestones base.TplName = "user/dashboard/milestones"
	tplProfile    base.TplName = "user/profile"
)

// getDashboardContextUser finds out which context user dashboard is being viewed as .
func getDashboardContextUser(ctx *context.Context) *user_model.User {
	ctxUser := ctx.Doer
	orgName := ctx.Params(":org")
	if len(orgName) > 0 {
		ctxUser = ctx.Org.Organization.AsUser()
		ctx.Data["Teams"] = ctx.Org.Teams
	}
	ctx.Data["ContextUser"] = ctxUser

	orgs, err := organization.GetUserOrgsList(ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserOrgsList", err)
		return nil
	}
	ctx.Data["Orgs"] = orgs

	return ctxUser
}

// Dashboard render the dashboard page
func Dashboard(ctx *context.Context) {
	ctxUser := getDashboardContextUser(ctx)
	if ctx.Written() {
		return
	}

	var (
		date = ctx.FormString("date")
		page = ctx.FormInt("page")
	)

	// Make sure page number is at least 1. Will be posted to ctx.Data.
	if page <= 1 {
		page = 1
	}

	ctx.Data["Title"] = ctxUser.DisplayName() + " - " + ctx.Tr("dashboard")
	ctx.Data["PageIsDashboard"] = true
	ctx.Data["PageIsNews"] = true
	cnt, _ := organization.GetOrganizationCount(ctx, ctxUser)
	ctx.Data["UserOrgsCount"] = cnt
	ctx.Data["MirrorsEnabled"] = setting.Mirror.Enabled
	ctx.Data["Date"] = date

	var uid int64
	if ctxUser != nil {
		uid = ctxUser.ID
	}

	ctx.PageData["dashboardRepoList"] = map[string]interface{}{
		"searchLimit": setting.UI.User.RepoPagingNum,
		"uid":         uid,
	}

	if setting.Service.EnableUserHeatmap {
		data, err := activities_model.GetUserHeatmapDataByUserTeam(ctxUser, ctx.Org.Team, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserHeatmapDataByUserTeam", err)
			return
		}
		ctx.Data["HeatmapData"] = data
		ctx.Data["HeatmapTotalContributions"] = activities_model.GetTotalContributionsInHeatmap(data)
	}

	feeds, count, err := activities_model.GetFeeds(ctx, activities_model.GetFeedsOptions{
		RequestedUser:   ctxUser,
		RequestedTeam:   ctx.Org.Team,
		Actor:           ctx.Doer,
		IncludePrivate:  true,
		OnlyPerformedBy: false,
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.FeedPagingNum,
		},
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}

	ctx.Data["Feeds"] = feeds

	pager := context.NewPagination(int(count), setting.UI.FeedPagingNum, page, 5)
	pager.AddParam(ctx, "date", "Date")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplDashboard)
}

// Milestones render the user milestones page
func Milestones(ctx *context.Context) {
	if unit.TypeIssues.UnitGlobalDisabled() && unit.TypePullRequests.UnitGlobalDisabled() {
		log.Debug("Milestones overview page not available as both issues and pull requests are globally disabled")
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.Data["Title"] = ctx.Tr("milestones")
	ctx.Data["PageIsMilestonesDashboard"] = true

	ctxUser := getDashboardContextUser(ctx)
	if ctx.Written() {
		return
	}

	repoOpts := repo_model.SearchRepoOptions{
		Actor:         ctxUser,
		OwnerID:       ctxUser.ID,
		Private:       true,
		AllPublic:     false, // Include also all public repositories of users and public organisations
		AllLimited:    false, // Include also all public repositories of limited organisations
		Archived:      util.OptionalBoolFalse,
		HasMilestones: util.OptionalBoolTrue, // Just needs display repos has milestones
	}

	if ctxUser.IsOrganization() && ctx.Org.Team != nil {
		repoOpts.TeamID = ctx.Org.Team.ID
	}

	var (
		userRepoCond = repo_model.SearchRepositoryCondition(&repoOpts) // all repo condition user could visit
		repoCond     = userRepoCond
		repoIDs      []int64

		reposQuery   = ctx.FormString("repos")
		isShowClosed = ctx.FormString("state") == "closed"
		sortType     = ctx.FormString("sort")
		page         = ctx.FormInt("page")
		keyword      = ctx.FormTrim("q")
	)

	if page <= 1 {
		page = 1
	}

	if len(reposQuery) != 0 {
		if issueReposQueryPattern.MatchString(reposQuery) {
			// remove "[" and "]" from string
			reposQuery = reposQuery[1 : len(reposQuery)-1]
			// for each ID (delimiter ",") add to int to repoIDs

			for _, rID := range strings.Split(reposQuery, ",") {
				// Ensure nonempty string entries
				if rID != "" && rID != "0" {
					rIDint64, err := strconv.ParseInt(rID, 10, 64)
					// If the repo id specified by query is not parseable or not accessible by user, just ignore it.
					if err == nil {
						repoIDs = append(repoIDs, rIDint64)
					}
				}
			}
			if len(repoIDs) > 0 {
				// Don't just let repoCond = builder.In("id", repoIDs) because user may has no permission on repoIDs
				// But the original repoCond has a limitation
				repoCond = repoCond.And(builder.In("id", repoIDs))
			}
		} else {
			log.Warn("issueReposQueryPattern not match with query")
		}
	}

	counts, err := issues_model.CountMilestonesByRepoCondAndKw(userRepoCond, keyword, isShowClosed)
	if err != nil {
		ctx.ServerError("CountMilestonesByRepoIDs", err)
		return
	}

	milestones, err := issues_model.SearchMilestones(repoCond, page, isShowClosed, sortType, keyword)
	if err != nil {
		ctx.ServerError("SearchMilestones", err)
		return
	}

	showRepos, _, err := repo_model.SearchRepositoryByCondition(ctx, &repoOpts, userRepoCond, false)
	if err != nil {
		ctx.ServerError("SearchRepositoryByCondition", err)
		return
	}
	sort.Sort(showRepos)

	for i := 0; i < len(milestones); {
		for _, repo := range showRepos {
			if milestones[i].RepoID == repo.ID {
				milestones[i].Repo = repo
				break
			}
		}
		if milestones[i].Repo == nil {
			log.Warn("Cannot find milestone %d 's repository %d", milestones[i].ID, milestones[i].RepoID)
			milestones = append(milestones[:i], milestones[i+1:]...)
			continue
		}

		milestones[i].RenderedContent, err = markdown.RenderString(&markup.RenderContext{
			URLPrefix: milestones[i].Repo.Link(),
			Metas:     milestones[i].Repo.ComposeMetas(),
			Ctx:       ctx,
		}, milestones[i].Content)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}

		if milestones[i].Repo.IsTimetrackerEnabled(ctx) {
			err := milestones[i].LoadTotalTrackedTime()
			if err != nil {
				ctx.ServerError("LoadTotalTrackedTime", err)
				return
			}
		}
		i++
	}

	milestoneStats, err := issues_model.GetMilestonesStatsByRepoCondAndKw(repoCond, keyword)
	if err != nil {
		ctx.ServerError("GetMilestoneStats", err)
		return
	}

	var totalMilestoneStats *issues_model.MilestonesStats
	if len(repoIDs) == 0 {
		totalMilestoneStats = milestoneStats
	} else {
		totalMilestoneStats, err = issues_model.GetMilestonesStatsByRepoCondAndKw(userRepoCond, keyword)
		if err != nil {
			ctx.ServerError("GetMilestoneStats", err)
			return
		}
	}

	var pagerCount int
	if isShowClosed {
		ctx.Data["State"] = "closed"
		ctx.Data["Total"] = totalMilestoneStats.ClosedCount
		pagerCount = int(milestoneStats.ClosedCount)
	} else {
		ctx.Data["State"] = "open"
		ctx.Data["Total"] = totalMilestoneStats.OpenCount
		pagerCount = int(milestoneStats.OpenCount)
	}

	ctx.Data["Milestones"] = milestones
	ctx.Data["Repos"] = showRepos
	ctx.Data["Counts"] = counts
	ctx.Data["MilestoneStats"] = milestoneStats
	ctx.Data["SortType"] = sortType
	ctx.Data["Keyword"] = keyword
	if milestoneStats.Total() != totalMilestoneStats.Total() {
		ctx.Data["RepoIDs"] = repoIDs
	}
	ctx.Data["IsShowClosed"] = isShowClosed

	pager := context.NewPagination(pagerCount, setting.UI.IssuePagingNum, page, 5)
	pager.AddParam(ctx, "q", "Keyword")
	pager.AddParam(ctx, "repos", "RepoIDs")
	pager.AddParam(ctx, "sort", "SortType")
	pager.AddParam(ctx, "state", "State")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplMilestones)
}

// Pulls renders the user's pull request overview page
func Pulls(ctx *context.Context) {
	if unit.TypePullRequests.UnitGlobalDisabled() {
		log.Debug("Pull request overview page not available as it is globally disabled.")
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.Data["Title"] = ctx.Tr("pull_requests")
	ctx.Data["PageIsPulls"] = true
	ctx.Data["SingleRepoAction"] = "pull"
	buildIssueOverview(ctx, unit.TypePullRequests)
}

// Issues renders the user's issues overview page
func Issues(ctx *context.Context) {
	if unit.TypeIssues.UnitGlobalDisabled() {
		log.Debug("Issues overview page not available as it is globally disabled.")
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.Data["Title"] = ctx.Tr("issues")
	ctx.Data["PageIsIssues"] = true
	ctx.Data["SingleRepoAction"] = "issue"
	buildIssueOverview(ctx, unit.TypeIssues)
}

// Regexp for repos query
var issueReposQueryPattern = regexp.MustCompile(`^\[\d+(,\d+)*,?\]$`)

func buildIssueOverview(ctx *context.Context, unitType unit.Type) {
	// ----------------------------------------------------
	// Determine user; can be either user or organization.
	// Return with NotFound or ServerError if unsuccessful.
	// ----------------------------------------------------

	ctxUser := getDashboardContextUser(ctx)
	if ctx.Written() {
		return
	}

	var (
		viewType   string
		sortType   = ctx.FormString("sort")
		filterMode int
	)

	// Default to recently updated, unlike repository issues list
	if sortType == "" {
		sortType = "recentupdate"
	}

	// --------------------------------------------------------------------------------
	// Distinguish User from Organization.
	// Org:
	// - Remember pre-determined viewType string for later. Will be posted to ctx.Data.
	//   Organization does not have view type and filter mode.
	// User:
	// - Use ctx.FormString("type") to determine filterMode.
	//  The type is set when clicking for example "assigned to me" on the overview page.
	// - Remember either this or a fallback. Will be posted to ctx.Data.
	// --------------------------------------------------------------------------------

	// TODO: distinguish during routing

	viewType = ctx.FormString("type")
	switch viewType {
	case "assigned":
		filterMode = issues_model.FilterModeAssign
	case "created_by":
		filterMode = issues_model.FilterModeCreate
	case "mentioned":
		filterMode = issues_model.FilterModeMention
	case "review_requested":
		filterMode = issues_model.FilterModeReviewRequested
	case "reviewed_by":
		filterMode = issues_model.FilterModeReviewed
	case "your_repositories":
		fallthrough
	default:
		filterMode = issues_model.FilterModeYourRepositories
		viewType = "your_repositories"
	}

	// --------------------------------------------------------------------------
	// Build opts (IssuesOptions), which contains filter information.
	// Will eventually be used to retrieve issues relevant for the overview page.
	// Note: Non-final states of opts are used in-between, namely for:
	//       - Keyword search
	//       - Count Issues by repo
	// --------------------------------------------------------------------------

	// Get repository IDs where User/Org/Team has access.
	var team *organization.Team
	var org *organization.Organization
	if ctx.Org != nil {
		org = ctx.Org.Organization
		team = ctx.Org.Team
	}

	isPullList := unitType == unit.TypePullRequests
	opts := &issues_model.IssuesOptions{
		IsPull:     util.OptionalBoolOf(isPullList),
		SortType:   sortType,
		IsArchived: util.OptionalBoolFalse,
		Org:        org,
		Team:       team,
		User:       ctx.Doer,
	}

	// Search all repositories which
	//
	// As user:
	// - Owns the repository.
	// - Have collaborator permissions in repository.
	//
	// As org:
	// - Owns the repository.
	//
	// As team:
	// - Team org's owns the repository.
	// - Team has read permission to repository.
	repoOpts := &repo_model.SearchRepoOptions{
		Actor:      ctx.Doer,
		OwnerID:    ctx.Doer.ID,
		Private:    true,
		AllPublic:  false,
		AllLimited: false,
	}

	if team != nil {
		repoOpts.TeamID = team.ID
	}

	switch filterMode {
	case issues_model.FilterModeAll:
	case issues_model.FilterModeYourRepositories:
	case issues_model.FilterModeAssign:
		opts.AssigneeID = ctx.Doer.ID
	case issues_model.FilterModeCreate:
		opts.PosterID = ctx.Doer.ID
	case issues_model.FilterModeMention:
		opts.MentionedID = ctx.Doer.ID
	case issues_model.FilterModeReviewRequested:
		opts.ReviewRequestedID = ctx.Doer.ID
	case issues_model.FilterModeReviewed:
		opts.ReviewedID = ctx.Doer.ID
	}

	// keyword holds the search term entered into the search field.
	keyword := strings.Trim(ctx.FormString("q"), " ")
	ctx.Data["Keyword"] = keyword

	// Execute keyword search for issues.
	// USING NON-FINAL STATE OF opts FOR A QUERY.
	issueIDsFromSearch, err := issueIDsFromSearch(ctx, ctxUser, keyword, opts)
	if err != nil {
		ctx.ServerError("issueIDsFromSearch", err)
		return
	}

	// Ensure no issues are returned if a keyword was provided that didn't match any issues.
	var forceEmpty bool

	if len(issueIDsFromSearch) > 0 {
		opts.IssueIDs = issueIDsFromSearch
	} else if len(keyword) > 0 {
		forceEmpty = true
	}

	// Educated guess: Do or don't show closed issues.
	isShowClosed := ctx.FormString("state") == "closed"
	opts.IsClosed = util.OptionalBoolOf(isShowClosed)

	// Filter repos and count issues in them. Count will be used later.
	// USING NON-FINAL STATE OF opts FOR A QUERY.
	var issueCountByRepo map[int64]int64
	if !forceEmpty {
		issueCountByRepo, err = issues_model.CountIssuesByRepo(ctx, opts)
		if err != nil {
			ctx.ServerError("CountIssuesByRepo", err)
			return
		}
	}

	// Make sure page number is at least 1. Will be posted to ctx.Data.
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	opts.Page = page
	opts.PageSize = setting.UI.IssuePagingNum

	// Get IDs for labels (a filter option for issues/pulls).
	// Required for IssuesOptions.
	var labelIDs []int64
	selectedLabels := ctx.FormString("labels")
	if len(selectedLabels) > 0 && selectedLabels != "0" {
		labelIDs, err = base.StringsToInt64s(strings.Split(selectedLabels, ","))
		if err != nil {
			ctx.ServerError("StringsToInt64s", err)
			return
		}
	}
	opts.LabelIDs = labelIDs

	// Parse ctx.FormString("repos") and remember matched repo IDs for later.
	// Gets set when clicking filters on the issues overview page.
	repoIDs := getRepoIDs(ctx.FormString("repos"))
	if len(repoIDs) > 0 {
		opts.RepoCond = builder.In("issue.repo_id", repoIDs)
	}

	// ------------------------------
	// Get issues as defined by opts.
	// ------------------------------

	// Slice of Issues that will be displayed on the overview page
	// USING FINAL STATE OF opts FOR A QUERY.
	var issues []*issues_model.Issue
	if !forceEmpty {
		issues, err = issues_model.Issues(ctx, opts)
		if err != nil {
			ctx.ServerError("Issues", err)
			return
		}
	} else {
		issues = []*issues_model.Issue{}
	}

	// ----------------------------------
	// Add repository pointers to Issues.
	// ----------------------------------

	// showReposMap maps repository IDs to their Repository pointers.
	showReposMap, err := loadRepoByIDs(ctxUser, issueCountByRepo, unitType)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.NotFound("GetRepositoryByID", err)
			return
		}
		ctx.ServerError("loadRepoByIDs", err)
		return
	}

	// a RepositoryList
	showRepos := repo_model.RepositoryListOfMap(showReposMap)
	sort.Sort(showRepos)

	// maps pull request IDs to their CommitStatus. Will be posted to ctx.Data.
	for _, issue := range issues {
		if issue.Repo == nil {
			issue.Repo = showReposMap[issue.RepoID]
		}
	}

	commitStatuses, lastStatus, err := pull_service.GetIssuesAllCommitStatus(ctx, issues)
	if err != nil {
		ctx.ServerError("GetIssuesLastCommitStatus", err)
		return
	}

	// -------------------------------
	// Fill stats to post to ctx.Data.
	// -------------------------------
	var issueStats *issues_model.IssueStats
	if !forceEmpty {
		statsOpts := issues_model.UserIssueStatsOptions{
			UserID:     ctx.Doer.ID,
			FilterMode: filterMode,
			IsPull:     isPullList,
			IsClosed:   isShowClosed,
			IssueIDs:   issueIDsFromSearch,
			IsArchived: util.OptionalBoolFalse,
			LabelIDs:   opts.LabelIDs,
			Org:        org,
			Team:       team,
			RepoCond:   opts.RepoCond,
		}

		issueStats, err = issues_model.GetUserIssueStats(statsOpts)
		if err != nil {
			ctx.ServerError("GetUserIssueStats Shown", err)
			return
		}
	} else {
		issueStats = &issues_model.IssueStats{}
	}

	// Will be posted to ctx.Data.
	var shownIssues int
	if !isShowClosed {
		shownIssues = int(issueStats.OpenCount)
	} else {
		shownIssues = int(issueStats.ClosedCount)
	}
	if len(repoIDs) != 0 {
		shownIssues = 0
		for _, repoID := range repoIDs {
			shownIssues += int(issueCountByRepo[repoID])
		}
	}

	var allIssueCount int64
	for _, issueCount := range issueCountByRepo {
		allIssueCount += issueCount
	}
	ctx.Data["TotalIssueCount"] = allIssueCount

	if len(repoIDs) == 1 {
		repo := showReposMap[repoIDs[0]]
		if repo != nil {
			ctx.Data["SingleRepoLink"] = repo.Link()
		}
	}

	ctx.Data["IsShowClosed"] = isShowClosed

	ctx.Data["IssueRefEndNames"], ctx.Data["IssueRefURLs"] = issue_service.GetRefEndNamesAndURLs(issues, ctx.FormString("RepoLink"))

	ctx.Data["Issues"] = issues

	approvalCounts, err := issues_model.IssueList(issues).GetApprovalCounts(ctx)
	if err != nil {
		ctx.ServerError("ApprovalCounts", err)
		return
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
	ctx.Data["CommitLastStatus"] = lastStatus
	ctx.Data["CommitStatuses"] = commitStatuses
	ctx.Data["Repos"] = showRepos
	ctx.Data["Counts"] = issueCountByRepo
	ctx.Data["IssueStats"] = issueStats
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["RepoIDs"] = repoIDs
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["SelectLabels"] = selectedLabels

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	// Convert []int64 to string
	reposParam, _ := json.Marshal(repoIDs)

	ctx.Data["ReposParam"] = string(reposParam)

	pager := context.NewPagination(shownIssues, setting.UI.IssuePagingNum, page, 5)
	pager.AddParam(ctx, "q", "Keyword")
	pager.AddParam(ctx, "type", "ViewType")
	pager.AddParam(ctx, "repos", "ReposParam")
	pager.AddParam(ctx, "sort", "SortType")
	pager.AddParam(ctx, "state", "State")
	pager.AddParam(ctx, "labels", "SelectLabels")
	pager.AddParam(ctx, "milestone", "MilestoneID")
	pager.AddParam(ctx, "assignee", "AssigneeID")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplIssues)
}

func getRepoIDs(reposQuery string) []int64 {
	if len(reposQuery) == 0 || reposQuery == "[]" {
		return []int64{}
	}
	if !issueReposQueryPattern.MatchString(reposQuery) {
		log.Warn("issueReposQueryPattern does not match query")
		return []int64{}
	}

	var repoIDs []int64
	// remove "[" and "]" from string
	reposQuery = reposQuery[1 : len(reposQuery)-1]
	// for each ID (delimiter ",") add to int to repoIDs
	for _, rID := range strings.Split(reposQuery, ",") {
		// Ensure nonempty string entries
		if rID != "" && rID != "0" {
			rIDint64, err := strconv.ParseInt(rID, 10, 64)
			if err == nil {
				repoIDs = append(repoIDs, rIDint64)
			}
		}
	}

	return repoIDs
}

func issueIDsFromSearch(ctx *context.Context, ctxUser *user_model.User, keyword string, opts *issues_model.IssuesOptions) ([]int64, error) {
	if len(keyword) == 0 {
		return []int64{}, nil
	}

	searchRepoIDs, err := issues_model.GetRepoIDsForIssuesOptions(opts, ctxUser)
	if err != nil {
		return nil, fmt.Errorf("GetRepoIDsForIssuesOptions: %w", err)
	}
	issueIDsFromSearch, err := issue_indexer.SearchIssuesByKeyword(ctx, searchRepoIDs, keyword)
	if err != nil {
		return nil, fmt.Errorf("SearchIssuesByKeyword: %w", err)
	}

	return issueIDsFromSearch, nil
}

func loadRepoByIDs(ctxUser *user_model.User, issueCountByRepo map[int64]int64, unitType unit.Type) (map[int64]*repo_model.Repository, error) {
	totalRes := make(map[int64]*repo_model.Repository, len(issueCountByRepo))
	repoIDs := make([]int64, 0, 500)
	for id := range issueCountByRepo {
		if id <= 0 {
			continue
		}
		repoIDs = append(repoIDs, id)
		if len(repoIDs) == 500 {
			if err := repo_model.FindReposMapByIDs(repoIDs, totalRes); err != nil {
				return nil, err
			}
			repoIDs = repoIDs[:0]
		}
	}
	if len(repoIDs) > 0 {
		if err := repo_model.FindReposMapByIDs(repoIDs, totalRes); err != nil {
			return nil, err
		}
	}
	return totalRes, nil
}

// ShowSSHKeys output all the ssh keys of user by uid
func ShowSSHKeys(ctx *context.Context) {
	keys, err := asymkey_model.ListPublicKeys(ctx.ContextUser.ID, db.ListOptions{})
	if err != nil {
		ctx.ServerError("ListPublicKeys", err)
		return
	}

	var buf bytes.Buffer
	for i := range keys {
		buf.WriteString(keys[i].OmitEmail())
		buf.WriteString("\n")
	}
	ctx.PlainTextBytes(http.StatusOK, buf.Bytes())
}

// ShowGPGKeys output all the public GPG keys of user by uid
func ShowGPGKeys(ctx *context.Context) {
	keys, err := asymkey_model.ListGPGKeys(ctx, ctx.ContextUser.ID, db.ListOptions{})
	if err != nil {
		ctx.ServerError("ListGPGKeys", err)
		return
	}

	entities := make([]*openpgp.Entity, 0)
	failedEntitiesID := make([]string, 0)
	for _, k := range keys {
		e, err := asymkey_model.GPGKeyToEntity(k)
		if err != nil {
			if asymkey_model.IsErrGPGKeyImportNotExist(err) {
				failedEntitiesID = append(failedEntitiesID, k.KeyID)
				continue // Skip previous import without backup of imported armored key
			}
			ctx.ServerError("ShowGPGKeys", err)
			return
		}
		entities = append(entities, e)
	}
	var buf bytes.Buffer

	headers := make(map[string]string)
	if len(failedEntitiesID) > 0 { // If some key need re-import to be exported
		headers["Note"] = fmt.Sprintf("The keys with the following IDs couldn't be exported and need to be reuploaded %s", strings.Join(failedEntitiesID, ", "))
	} else if len(entities) == 0 {
		headers["Note"] = "This user hasn't uploaded any GPG keys."
	}
	writer, _ := armor.Encode(&buf, "PGP PUBLIC KEY BLOCK", headers)
	for _, e := range entities {
		err = e.Serialize(writer) // TODO find why key are exported with a different cipherTypeByte as original (should not be blocking but strange)
		if err != nil {
			ctx.ServerError("ShowGPGKeys", err)
			return
		}
	}
	writer.Close()
	ctx.PlainTextBytes(http.StatusOK, buf.Bytes())
}

func UsernameSubRoute(ctx *context.Context) {
	// WORKAROUND to support usernames with "." in it
	// https://github.com/go-chi/chi/issues/781
	username := ctx.Params("username")
	reloadParam := func(suffix string) (success bool) {
		ctx.SetParams("username", strings.TrimSuffix(username, suffix))
		context_service.UserAssignmentWeb()(ctx)
		return !ctx.Written()
	}
	switch {
	case strings.HasSuffix(username, ".png"):
		if reloadParam(".png") {
			AvatarByUserName(ctx)
		}
	case strings.HasSuffix(username, ".keys"):
		if reloadParam(".keys") {
			ShowSSHKeys(ctx)
		}
	case strings.HasSuffix(username, ".gpg"):
		if reloadParam(".gpg") {
			ShowGPGKeys(ctx)
		}
	case strings.HasSuffix(username, ".rss"):
		if !setting.Other.EnableFeed {
			ctx.Error(http.StatusNotFound)
			return
		}
		if reloadParam(".rss") {
			context_service.UserAssignmentWeb()(ctx)
			feed.ShowUserFeedRSS(ctx)
		}
	case strings.HasSuffix(username, ".atom"):
		if !setting.Other.EnableFeed {
			ctx.Error(http.StatusNotFound)
			return
		}
		if reloadParam(".atom") {
			feed.ShowUserFeedAtom(ctx)
		}
	default:
		context_service.UserAssignmentWeb()(ctx)
		if !ctx.Written() {
			ctx.Data["EnableFeed"] = setting.Other.EnableFeed
			Profile(ctx)
		}
	}
}
