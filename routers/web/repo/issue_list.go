// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
)

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

// ListIssues list the issues of a repository
func ListIssues(ctx *context.Context) {
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

	var labelIDs []int64
	if splitted := strings.Split(ctx.FormString("labels"), ","); len(splitted) > 0 {
		labelIDs, err = issues_model.GetLabelIDsInRepoByNames(ctx, ctx.Repo.Repository.ID, splitted)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
	}

	var mileIDs []int64
	if part := strings.Split(ctx.FormString("milestones"), ","); len(part) > 0 {
		for i := range part {
			// uses names and fall back to ids
			// non existent milestones are discarded
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
	if len(labelIDs) == 1 && labelIDs[0] == 0 {
		searchOpt.NoLabelOnly = true
	} else {
		for _, labelID := range labelIDs {
			if labelID > 0 {
				searchOpt.IncludedLabelIDs = append(searchOpt.IncludedLabelIDs, labelID)
			} else {
				searchOpt.ExcludedLabelIDs = append(searchOpt.ExcludedLabelIDs, -labelID)
			}
		}
	}

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

	var isClosed bool
	switch action := ctx.FormString("action"); action {
	case "open":
		isClosed = false
	case "close":
		isClosed = true
	default:
		log.Warn("Unrecognized action: %s", action)
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
		if issue.IsClosed != isClosed {
			if err := issue_service.ChangeStatus(ctx, issue, ctx.Doer, "", isClosed); err != nil {
				if issues_model.IsErrDependenciesLeft(err) {
					ctx.JSON(http.StatusPreconditionFailed, map[string]any{
						"error": ctx.Tr("repo.issues.dependency.issue_batch_close_blocked", issue.Index),
					})
					return
				}
				ctx.ServerError("ChangeStatus", err)
				return
			}
		}
	}
	ctx.JSONOK()
}
