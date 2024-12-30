// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/shared/issue"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
)

func issueIDsFromSearch(ctx *context.Context, keyword string, opts *issues_model.IssuesOptions) ([]int64, error) {
	ids, _, err := issue_indexer.SearchIssues(ctx, issue_indexer.ToSearchOptions(keyword, opts))
	if err != nil {
		return nil, fmt.Errorf("SearchIssues: %w", err)
	}
	return ids, nil
}

func retrieveProjectsForIssueList(ctx *context.Context, repo *repo_model.Repository) {
	ctx.Data["OpenProjects"], ctx.Data["ClosedProjects"] = retrieveProjectsInternal(ctx, repo)
}

// SearchIssues searches for issues across the repositories that the user has access to
func SearchIssues(ctx *context.Context) {
	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, err.Error())
		return
	}

	var isClosed optional.Option[bool]
	switch ctx.FormString("state") {
	case "closed":
		isClosed = optional.Some(true)
	case "all":
		isClosed = optional.None[bool]()
	default:
		isClosed = optional.Some(false)
	}

	var (
		repoIDs   []int64
		allPublic bool
	)
	{
		// find repos user can access (for issue search)
		opts := &repo_model.SearchRepoOptions{
			Private:     false,
			AllPublic:   true,
			TopicOnly:   false,
			Collaborate: optional.None[bool](),
			// This needs to be a column that is not nil in fixtures or
			// MySQL will return different results when sorting by null in some cases
			OrderBy: db.SearchOrderByAlphabetically,
			Actor:   ctx.Doer,
		}
		if ctx.IsSigned {
			opts.Private = true
			opts.AllLimited = true
		}
		if ctx.FormString("owner") != "" {
			owner, err := user_model.GetUserByName(ctx, ctx.FormString("owner"))
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					ctx.Error(http.StatusBadRequest, "Owner not found", err.Error())
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err.Error())
				}
				return
			}
			opts.OwnerID = owner.ID
			opts.AllLimited = false
			opts.AllPublic = false
			opts.Collaborate = optional.Some(false)
		}
		if ctx.FormString("team") != "" {
			if ctx.FormString("owner") == "" {
				ctx.Error(http.StatusBadRequest, "", "Owner organisation is required for filtering on team")
				return
			}
			team, err := organization.GetTeam(ctx, opts.OwnerID, ctx.FormString("team"))
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusBadRequest, "Team not found", err.Error())
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err.Error())
				}
				return
			}
			opts.TeamID = team.ID
		}

		if opts.AllPublic {
			allPublic = true
			opts.AllPublic = false // set it false to avoid returning too many repos, we could filter by indexer
		}
		repoIDs, _, err = repo_model.SearchRepositoryIDs(ctx, opts)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "SearchRepositoryIDs", err.Error())
			return
		}
		if len(repoIDs) == 0 {
			// no repos found, don't let the indexer return all repos
			repoIDs = []int64{0}
		}
	}

	keyword := ctx.FormTrim("q")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}

	isPull := optional.None[bool]()
	switch ctx.FormString("type") {
	case "pulls":
		isPull = optional.Some(true)
	case "issues":
		isPull = optional.Some(false)
	}

	var includedAnyLabels []int64
	{
		labels := ctx.FormTrim("labels")
		var includedLabelNames []string
		if len(labels) > 0 {
			includedLabelNames = strings.Split(labels, ",")
		}
		includedAnyLabels, err = issues_model.GetLabelIDsByNames(ctx, includedLabelNames)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelIDsByNames", err.Error())
			return
		}
	}

	var includedMilestones []int64
	{
		milestones := ctx.FormTrim("milestones")
		var includedMilestoneNames []string
		if len(milestones) > 0 {
			includedMilestoneNames = strings.Split(milestones, ",")
		}
		includedMilestones, err = issues_model.GetMilestoneIDsByNames(ctx, includedMilestoneNames)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetMilestoneIDsByNames", err.Error())
			return
		}
	}

	projectID := optional.None[int64]()
	if v := ctx.FormInt64("project"); v > 0 {
		projectID = optional.Some(v)
	}

	// this api is also used in UI,
	// so the default limit is set to fit UI needs
	limit := ctx.FormInt("limit")
	if limit == 0 {
		limit = setting.UI.IssuePagingNum
	} else if limit > setting.API.MaxResponseItems {
		limit = setting.API.MaxResponseItems
	}

	searchOpt := &issue_indexer.SearchOptions{
		Paginator: &db.ListOptions{
			Page:     ctx.FormInt("page"),
			PageSize: limit,
		},
		Keyword:             keyword,
		RepoIDs:             repoIDs,
		AllPublic:           allPublic,
		IsPull:              isPull,
		IsClosed:            isClosed,
		IncludedAnyLabelIDs: includedAnyLabels,
		MilestoneIDs:        includedMilestones,
		ProjectID:           projectID,
		SortBy:              issue_indexer.SortByCreatedDesc,
	}

	if since != 0 {
		searchOpt.UpdatedAfterUnix = optional.Some(since)
	}
	if before != 0 {
		searchOpt.UpdatedBeforeUnix = optional.Some(before)
	}

	if ctx.IsSigned {
		ctxUserID := ctx.Doer.ID
		if ctx.FormBool("created") {
			searchOpt.PosterID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("assigned") {
			searchOpt.AssigneeID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("mentioned") {
			searchOpt.MentionID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("review_requested") {
			searchOpt.ReviewRequestedID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("reviewed") {
			searchOpt.ReviewedID = optional.Some(ctxUserID)
		}
	}

	// FIXME: It's unsupported to sort by priority repo when searching by indexer,
	//        it's indeed an regression, but I think it is worth to support filtering by indexer first.
	_ = ctx.FormInt64("priority_repo_id")

	ids, total, err := issue_indexer.SearchIssues(ctx, searchOpt)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchIssues", err.Error())
		return
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindIssuesByIDs", err.Error())
		return
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, ctx.Doer, issues))
}

func getUserIDForFilter(ctx *context.Context, queryName string) int64 {
	userName := ctx.FormString(queryName)
	if len(userName) == 0 {
		return 0
	}

	user, err := user_model.GetUserByName(ctx, userName)
	if user_model.IsErrUserNotExist(err) {
		ctx.NotFound("", err)
		return 0
	}

	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return 0
	}

	return user.ID
}

// SearchRepoIssuesJSON lists the issues of a repository
// This function was copied from API (decouple the web and API routes),
// it is only used by frontend to search some dependency or related issues
func SearchRepoIssuesJSON(ctx *context.Context) {
	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, err.Error())
		return
	}

	var isClosed optional.Option[bool]
	switch ctx.FormString("state") {
	case "closed":
		isClosed = optional.Some(true)
	case "all":
		isClosed = optional.None[bool]()
	default:
		isClosed = optional.Some(false)
	}

	keyword := ctx.FormTrim("q")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}

	var mileIDs []int64
	if part := strings.Split(ctx.FormString("milestones"), ","); len(part) > 0 {
		for i := range part {
			// uses names and fall back to ids
			// non-existent milestones are discarded
			mile, err := issues_model.GetMilestoneByRepoIDANDName(ctx, ctx.Repo.Repository.ID, part[i])
			if err == nil {
				mileIDs = append(mileIDs, mile.ID)
				continue
			}
			if !issues_model.IsErrMilestoneNotExist(err) {
				ctx.Error(http.StatusInternalServerError, err.Error())
				return
			}
			id, err := strconv.ParseInt(part[i], 10, 64)
			if err != nil {
				continue
			}
			mile, err = issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, id)
			if err == nil {
				mileIDs = append(mileIDs, mile.ID)
				continue
			}
			if issues_model.IsErrMilestoneNotExist(err) {
				continue
			}
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
	}

	projectID := optional.None[int64]()
	if v := ctx.FormInt64("project"); v > 0 {
		projectID = optional.Some(v)
	}

	isPull := optional.None[bool]()
	switch ctx.FormString("type") {
	case "pulls":
		isPull = optional.Some(true)
	case "issues":
		isPull = optional.Some(false)
	}

	// FIXME: we should be more efficient here
	createdByID := getUserIDForFilter(ctx, "created_by")
	if ctx.Written() {
		return
	}
	assignedByID := getUserIDForFilter(ctx, "assigned_by")
	if ctx.Written() {
		return
	}
	mentionedByID := getUserIDForFilter(ctx, "mentioned_by")
	if ctx.Written() {
		return
	}

	searchOpt := &issue_indexer.SearchOptions{
		Paginator: &db.ListOptions{
			Page:     ctx.FormInt("page"),
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		Keyword:   keyword,
		RepoIDs:   []int64{ctx.Repo.Repository.ID},
		IsPull:    isPull,
		IsClosed:  isClosed,
		ProjectID: projectID,
		SortBy:    issue_indexer.SortByCreatedDesc,
	}
	if since != 0 {
		searchOpt.UpdatedAfterUnix = optional.Some(since)
	}
	if before != 0 {
		searchOpt.UpdatedBeforeUnix = optional.Some(before)
	}

	// TODO: the "labels" query parameter is never used, so no need to handle it

	if len(mileIDs) == 1 && mileIDs[0] == db.NoConditionID {
		searchOpt.MilestoneIDs = []int64{0}
	} else {
		searchOpt.MilestoneIDs = mileIDs
	}

	if createdByID > 0 {
		searchOpt.PosterID = optional.Some(createdByID)
	}
	if assignedByID > 0 {
		searchOpt.AssigneeID = optional.Some(assignedByID)
	}
	if mentionedByID > 0 {
		searchOpt.MentionID = optional.Some(mentionedByID)
	}

	ids, total, err := issue_indexer.SearchIssues(ctx, searchOpt)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchIssues", err.Error())
		return
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindIssuesByIDs", err.Error())
		return
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, ctx.Doer, issues))
}

func BatchDeleteIssues(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}
	for _, issue := range issues {
		if err := issue_service.DeleteIssue(ctx, ctx.Doer, ctx.Repo.GitRepo, issue); err != nil {
			ctx.ServerError("DeleteIssue", err)
			return
		}
	}
	ctx.JSONOK()
}

// UpdateIssueStatus change issue's status
func UpdateIssueStatus(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	action := ctx.FormString("action")
	if action != "open" && action != "close" {
		log.Warn("Unrecognized action: %s", action)
		ctx.JSONOK()
		return
	}

	if _, err := issues.LoadRepositories(ctx); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}
	if err := issues.LoadPullRequests(ctx); err != nil {
		ctx.ServerError("LoadPullRequests", err)
		return
	}

	for _, issue := range issues {
		if issue.IsPull && issue.PullRequest.HasMerged {
			continue
		}
		if action == "close" && !issue.IsClosed {
			if err := issue_service.CloseIssue(ctx, issue, ctx.Doer, ""); err != nil {
				if issues_model.IsErrDependenciesLeft(err) {
					ctx.JSON(http.StatusPreconditionFailed, map[string]any{
						"error": ctx.Tr("repo.issues.dependency.issue_batch_close_blocked", issue.Index),
					})
					return
				}
				ctx.ServerError("CloseIssue", err)
				return
			}
		} else if action == "open" && issue.IsClosed {
			if err := issue_service.ReopenIssue(ctx, issue, ctx.Doer, ""); err != nil {
				ctx.ServerError("ReopenIssue", err)
				return
			}
		}
	}
	ctx.JSONOK()
}

func renderMilestones(ctx *context.Context) {
	// Get milestones
	milestones, err := db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("GetAllRepoMilestones", err)
		return
	}

	openMilestones, closedMilestones := issues_model.MilestoneList{}, issues_model.MilestoneList{}
	for _, milestone := range milestones {
		if milestone.IsClosed {
			closedMilestones = append(closedMilestones, milestone)
		} else {
			openMilestones = append(openMilestones, milestone)
		}
	}
	ctx.Data["OpenMilestones"] = openMilestones
	ctx.Data["ClosedMilestones"] = closedMilestones
}

func issues(ctx *context.Context, milestoneID, projectID int64, isPullOption optional.Option[bool]) {
	var err error
	viewType := ctx.FormString("type")
	sortType := ctx.FormString("sort")
	types := []string{"all", "your_repositories", "assigned", "created_by", "mentioned", "review_requested", "reviewed_by"}
	if !util.SliceContainsString(types, viewType, true) {
		viewType = "all"
	}

	assigneeID := ctx.FormInt64("assignee") // TODO: use "optional" but not 0 in the future
	posterUsername := ctx.FormString("poster")
	posterUserID := shared_user.GetFilterUserIDByName(ctx, posterUsername)
	var mentionedID, reviewRequestedID, reviewedID int64

	if ctx.IsSigned {
		switch viewType {
		case "created_by":
			posterUserID = optional.Some(ctx.Doer.ID)
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

	repo := ctx.Repo.Repository
	keyword := strings.Trim(ctx.FormString("q"), " ")
	if bytes.Contains([]byte(keyword), []byte{0x00}) {
		keyword = ""
	}

	var mileIDs []int64
	if milestoneID > 0 || milestoneID == db.NoConditionID { // -1 to get those issues which have no any milestone assigned
		mileIDs = []int64{milestoneID}
	}

	labelIDs := issue.PrepareFilterIssueLabels(ctx, repo.ID, ctx.Repo.Owner)
	if ctx.Written() {
		return
	}

	var issueStats *issues_model.IssueStats
	statsOpts := &issues_model.IssuesOptions{
		RepoIDs:           []int64{repo.ID},
		LabelIDs:          labelIDs,
		MilestoneIDs:      mileIDs,
		ProjectID:         projectID,
		AssigneeID:        optional.Some(assigneeID),
		MentionedID:       mentionedID,
		PosterID:          posterUserID,
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

	var isShowClosed optional.Option[bool]
	switch ctx.FormString("state") {
	case "closed":
		isShowClosed = optional.Some(true)
	case "all":
		isShowClosed = optional.None[bool]()
	default:
		isShowClosed = optional.Some(false)
	}
	// if there are closed issues and no open issues, default to showing all issues
	if len(ctx.FormString("state")) == 0 && issueStats.OpenCount == 0 && issueStats.ClosedCount != 0 {
		isShowClosed = optional.None[bool]()
	}

	if repo.IsTimetrackerEnabled(ctx) {
		totalTrackedTime, err := issues_model.GetIssueTotalTrackedTime(ctx, statsOpts, isShowClosed)
		if err != nil {
			ctx.ServerError("GetIssueTotalTrackedTime", err)
			return
		}
		ctx.Data["TotalTrackedTime"] = totalTrackedTime
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	switch {
	case isShowClosed.Value():
		total = int(issueStats.ClosedCount)
	case !isShowClosed.Has():
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
			RepoIDs:           []int64{repo.ID},
			AssigneeID:        optional.Some(assigneeID),
			PosterID:          posterUserID,
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

	if ctx.IsSigned {
		if err := issues.LoadIsRead(ctx, ctx.Doer.ID); err != nil {
			ctx.ServerError("LoadIsRead", err)
			return
		}
	} else {
		for i := range issues {
			issues[i].IsRead = true
		}
	}

	commitStatuses, lastStatus, err := pull_service.GetIssuesAllCommitStatus(ctx, issues)
	if err != nil {
		ctx.ServerError("GetIssuesAllCommitStatus", err)
		return
	}
	if !ctx.Repo.CanRead(unit.TypeActions) {
		for key := range commitStatuses {
			git_model.CommitStatusesHideActionsURL(ctx, commitStatuses[key])
		}
	}

	if err := issues.LoadAttributes(ctx); err != nil {
		ctx.ServerError("issues.LoadAttributes", err)
		return
	}

	ctx.Data["Issues"] = issues
	ctx.Data["CommitLastStatus"] = lastStatus
	ctx.Data["CommitStatuses"] = commitStatuses

	// Get assignees.
	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, repo)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	handleMentionableAssigneesAndTeams(ctx, shared_user.MakeSelfOnTop(ctx.Doer, assigneeUsers))
	if ctx.Written() {
		return
	}

	ctx.Data["IssueRefEndNames"], ctx.Data["IssueRefURLs"] = issue_service.GetRefEndNamesAndURLs(issues, ctx.Repo.RepoLink)

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

	retrieveProjectsForIssueList(ctx, repo)
	if ctx.Written() {
		return
	}

	pinned, err := issues_model.GetPinnedIssues(ctx, repo.ID, isPullOption.Value())
	if err != nil {
		ctx.ServerError("GetPinnedIssues", err)
		return
	}

	showArchivedLabels := ctx.FormBool("archived_labels")
	ctx.Data["ShowArchivedLabels"] = showArchivedLabels
	ctx.Data["PinnedIssues"] = pinned
	ctx.Data["IsRepoAdmin"] = ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	ctx.Data["IssueStats"] = issueStats
	ctx.Data["OpenCount"] = issueStats.OpenCount
	ctx.Data["ClosedCount"] = issueStats.ClosedCount
	ctx.Data["SelLabelIDs"] = labelIDs
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["ProjectID"] = projectID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["PosterUsername"] = posterUsername
	ctx.Data["Keyword"] = keyword
	ctx.Data["IsShowClosed"] = isShowClosed
	switch {
	case isShowClosed.Value():
		ctx.Data["State"] = "closed"
	case !isShowClosed.Has():
		ctx.Data["State"] = "all"
	default:
		ctx.Data["State"] = "open"
	}
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
}

// Issues render issues page
func Issues(ctx *context.Context) {
	isPullList := ctx.PathParam("type") == "pulls"
	if isPullList {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.pulls")
		ctx.Data["PageIsPullList"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.issues")
		ctx.Data["PageIsIssueList"] = true
		ctx.Data["NewIssueChooseTemplate"] = issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)
	}

	issues(ctx, ctx.FormInt64("milestone"), ctx.FormInt64("project"), optional.Some(isPullList))
	if ctx.Written() {
		return
	}

	renderMilestones(ctx)
	if ctx.Written() {
		return
	}

	ctx.Data["CanWriteIssuesOrPulls"] = ctx.Repo.CanWriteIssuesOrPulls(isPullList)

	ctx.HTML(http.StatusOK, tplIssues)
}
