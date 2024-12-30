// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	issue_service "code.gitea.io/gitea/services/issue"
)

// SearchIssues searches for issues across the repositories that the user has access to
func SearchIssues(ctx *context.APIContext) {
	// swagger:operation GET /repos/issues/search issue issueSearchIssues
	// ---
	// summary: Search for issues across the repositories that the user has access to
	// produces:
	// - application/json
	// parameters:
	// - name: state
	//   in: query
	//   description: State of the issue
	//   type: string
	//   enum: [open, closed, all]
	//   default: open
	// - name: labels
	//   in: query
	//   description: Comma-separated list of label names. Fetch only issues that have any of these labels. Non existent labels are discarded.
	//   type: string
	// - name: milestones
	//   in: query
	//   description: Comma-separated list of milestone names. Fetch only issues that have any of these milestones. Non existent milestones are discarded.
	//   type: string
	// - name: q
	//   in: query
	//   description: Search string
	//   type: string
	// - name: priority_repo_id
	//   in: query
	//   description: Repository ID to prioritize in the results
	//   type: integer
	//   format: int64
	// - name: type
	//   in: query
	//   description: Filter by issue type
	//   type: string
	//   enum: [issues, pulls]
	// - name: since
	//   in: query
	//   description: Only show issues updated after the given time (RFC 3339 format)
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: Only show issues updated before the given time (RFC 3339 format)
	//   type: string
	//   format: date-time
	// - name: assigned
	//   in: query
	//   description: Filter issues or pulls assigned to the authenticated user
	//   type: boolean
	//   default: false
	// - name: created
	//   in: query
	//   description: Filter issues or pulls created by the authenticated user
	//   type: boolean
	//   default: false
	// - name: mentioned
	//   in: query
	//   description: Filter issues or pulls mentioning the authenticated user
	//   type: boolean
	//   default: false
	// - name: review_requested
	//   in: query
	//   description: Filter pull requests where the authenticated user's review was requested
	//   type: boolean
	//   default: false
	// - name: reviewed
	//   in: query
	//   description: Filter pull requests reviewed by the authenticated user
	//   type: boolean
	//   default: false
	// - name: owner
	//   in: query
	//   description: Filter by repository owner
	//   type: string
	// - name: team
	//   in: query
	//   description: Filter by team (requires organization owner parameter)
	//   type: string
	// - name: page
	//   in: query
	//   description: Page number of results to return (1-based)
	//   type: integer
	//   minimum: 1
	//   default: 1
	// - name: limit
	//   in: query
	//   description: Number of items per page
	//   type: integer
	//   minimum: 0
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
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
			opts.Private = !ctx.PublicOnly
			opts.AllLimited = true
		}
		if ctx.FormString("owner") != "" {
			owner, err := user_model.GetUserByName(ctx, ctx.FormString("owner"))
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					ctx.Error(http.StatusBadRequest, "Owner not found", err)
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
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
					ctx.Error(http.StatusBadRequest, "Team not found", err)
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
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
			ctx.Error(http.StatusInternalServerError, "SearchRepositoryIDs", err)
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

	var isPull optional.Option[bool]
	switch ctx.FormString("type") {
	case "pulls":
		isPull = optional.Some(true)
	case "issues":
		isPull = optional.Some(false)
	default:
		isPull = optional.None[bool]()
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
			ctx.Error(http.StatusInternalServerError, "GetLabelIDsByNames", err)
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
			ctx.Error(http.StatusInternalServerError, "GetMilestoneIDsByNames", err)
			return
		}
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
			PageSize: limit,
			Page:     ctx.FormInt("page"),
		},
		Keyword:             keyword,
		RepoIDs:             repoIDs,
		AllPublic:           allPublic,
		IsPull:              isPull,
		IsClosed:            isClosed,
		IncludedAnyLabelIDs: includedAnyLabels,
		MilestoneIDs:        includedMilestones,
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
		ctx.Error(http.StatusInternalServerError, "SearchIssues", err)
		return
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindIssuesByIDs", err)
		return
	}

	ctx.SetLinkHeader(int(total), limit)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, ctx.Doer, issues))
}

// ListIssues list the issues of a repository
func ListIssues(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues issue issueListIssues
	// ---
	// summary: List a repository's issues
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: state
	//   in: query
	//   description: whether issue is open or closed
	//   type: string
	//   enum: [closed, open, all]
	// - name: labels
	//   in: query
	//   description: comma separated list of labels. Fetch only issues that have any of this labels. Non existent labels are discarded
	//   type: string
	// - name: q
	//   in: query
	//   description: search string
	//   type: string
	// - name: type
	//   in: query
	//   description: filter by type (issues / pulls) if set
	//   type: string
	//   enum: [issues, pulls]
	// - name: milestones
	//   in: query
	//   description: comma separated list of milestone names or ids. It uses names and fall back to ids. Fetch only issues that have any of this milestones. Non existent milestones are discarded
	//   type: string
	// - name: since
	//   in: query
	//   description: Only show items updated after the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	//   required: false
	// - name: before
	//   in: query
	//   description: Only show items updated before the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	//   required: false
	// - name: created_by
	//   in: query
	//   description: Only show items which were created by the given user
	//   type: string
	// - name: assigned_by
	//   in: query
	//   description: Only show items for which the given user is assigned
	//   type: string
	// - name: mentioned_by
	//   in: query
	//   description: Only show items in which the given user was mentioned
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
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
			ctx.Error(http.StatusInternalServerError, "GetLabelIDsInRepoByNames", err)
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
				ctx.Error(http.StatusInternalServerError, "GetMilestoneByRepoIDANDName", err)
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
			ctx.Error(http.StatusInternalServerError, "GetMilestoneByRepoID", err)
		}
	}

	listOptions := utils.GetListOptions(ctx)

	isPull := optional.None[bool]()
	switch ctx.FormString("type") {
	case "pulls":
		isPull = optional.Some(true)
	case "issues":
		isPull = optional.Some(false)
	}

	if isPull.Has() && !ctx.Repo.CanReadIssuesOrPulls(isPull.Value()) {
		ctx.NotFound()
		return
	}

	if !isPull.Has() {
		canReadIssues := ctx.Repo.CanRead(unit.TypeIssues)
		canReadPulls := ctx.Repo.CanRead(unit.TypePullRequests)
		if !canReadIssues && !canReadPulls {
			ctx.NotFound()
			return
		} else if !canReadIssues {
			isPull = optional.Some(true)
		} else if !canReadPulls {
			isPull = optional.Some(false)
		}
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
		Paginator: &listOptions,
		Keyword:   keyword,
		RepoIDs:   []int64{ctx.Repo.Repository.ID},
		IsPull:    isPull,
		IsClosed:  isClosed,
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
		ctx.Error(http.StatusInternalServerError, "SearchIssues", err)
		return
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindIssuesByIDs", err)
		return
	}

	ctx.SetLinkHeader(int(total), listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, convert.ToAPIIssueList(ctx, ctx.Doer, issues))
}

func getUserIDForFilter(ctx *context.APIContext, queryName string) int64 {
	userName := ctx.FormString(queryName)
	if len(userName) == 0 {
		return 0
	}

	user, err := user_model.GetUserByName(ctx, userName)
	if user_model.IsErrUserNotExist(err) {
		ctx.NotFound(err)
		return 0
	}

	if err != nil {
		ctx.InternalServerError(err)
		return 0
	}

	return user.ID
}

// GetIssue get an issue of a repository
func GetIssue(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index} issue issueGetIssue
	// ---
	// summary: Get an issue
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Issue"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}
	if !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIIssue(ctx, ctx.Doer, issue))
}

// CreateIssue create an issue of a repository
func CreateIssue(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues issue issueCreateIssue
	// ---
	// summary: Create an issue. If using deadline only the date will be taken into account, and time of day ignored.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateIssueOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Issue"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateIssueOption)
	var deadlineUnix timeutil.TimeStamp
	if form.Deadline != nil && ctx.Repo.CanWrite(unit.TypeIssues) {
		deadlineUnix = timeutil.TimeStamp(form.Deadline.Unix())
	}

	issue := &issues_model.Issue{
		RepoID:       ctx.Repo.Repository.ID,
		Repo:         ctx.Repo.Repository,
		Title:        form.Title,
		PosterID:     ctx.Doer.ID,
		Poster:       ctx.Doer,
		Content:      form.Body,
		Ref:          form.Ref,
		DeadlineUnix: deadlineUnix,
	}

	assigneeIDs := make([]int64, 0)
	var err error
	if ctx.Repo.CanWrite(unit.TypeIssues) {
		issue.MilestoneID = form.Milestone
		assigneeIDs, err = issues_model.MakeIDsFromAPIAssigneesToAdd(ctx, form.Assignee, form.Assignees)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
			} else {
				ctx.Error(http.StatusInternalServerError, "AddAssigneeByName", err)
			}
			return
		}

		// Check if the passed assignees is assignable
		for _, aID := range assigneeIDs {
			assignee, err := user_model.GetUserByID(ctx, aID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetUserByID", err)
				return
			}

			valid, err := access_model.CanBeAssigned(ctx, assignee, ctx.Repo.Repository, false)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "canBeAssigned", err)
				return
			}
			if !valid {
				ctx.Error(http.StatusUnprocessableEntity, "canBeAssigned", repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: ctx.Repo.Repository.Name})
				return
			}
		}
	} else {
		// setting labels is not allowed if user is not a writer
		form.Labels = make([]int64, 0)
	}

	if err := issue_service.NewIssue(ctx, ctx.Repo.Repository, issue, form.Labels, nil, assigneeIDs, 0); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err)
		} else if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Error(http.StatusForbidden, "NewIssue", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "NewIssue", err)
		}
		return
	}

	if form.Closed {
		if err := issue_service.CloseIssue(ctx, issue, ctx.Doer, ""); err != nil {
			if issues_model.IsErrDependenciesLeft(err) {
				ctx.Error(http.StatusPreconditionFailed, "DependenciesLeft", "cannot close this issue because it still has open dependencies")
				return
			}
			ctx.Error(http.StatusInternalServerError, "ChangeStatus", err)
			return
		}
	}

	// Refetch from database to assign some automatic values
	issue, err = issues_model.GetIssueByID(ctx, issue.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByID", err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, ctx.Doer, issue))
}

// EditIssue modify an issue of a repository
func EditIssue(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/{index} issue issueEditIssue
	// ---
	// summary: Edit an issue. If using deadline only the date will be taken into account, and time of day ignored.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditIssueOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Issue"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "412":
	//     "$ref": "#/responses/error"

	form := web.GetForm(ctx).(*api.EditIssueOption)
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}
	issue.Repo = ctx.Repo.Repository
	canWrite := ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)

	err = issue.LoadAttributes(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	if !issue.IsPoster(ctx.Doer.ID) && !canWrite {
		ctx.Status(http.StatusForbidden)
		return
	}

	if len(form.Title) > 0 {
		err = issue_service.ChangeTitle(ctx, issue, ctx.Doer, form.Title)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ChangeTitle", err)
			return
		}
	}
	if form.Body != nil {
		err = issue_service.ChangeContent(ctx, issue, ctx.Doer, *form.Body, issue.ContentVersion)
		if err != nil {
			if errors.Is(err, issues_model.ErrIssueAlreadyChanged) {
				ctx.Error(http.StatusBadRequest, "ChangeContent", err)
				return
			}

			ctx.Error(http.StatusInternalServerError, "ChangeContent", err)
			return
		}
	}
	if form.Ref != nil {
		err = issue_service.ChangeIssueRef(ctx, issue, ctx.Doer, *form.Ref)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateRef", err)
			return
		}
	}

	// Update or remove the deadline, only if set and allowed
	if (form.Deadline != nil || form.RemoveDeadline != nil) && canWrite {
		var deadlineUnix timeutil.TimeStamp

		if form.RemoveDeadline == nil || !*form.RemoveDeadline {
			if form.Deadline == nil {
				ctx.Error(http.StatusBadRequest, "", "The due_date cannot be empty")
				return
			}
			if !form.Deadline.IsZero() {
				deadline := time.Date(form.Deadline.Year(), form.Deadline.Month(), form.Deadline.Day(),
					23, 59, 59, 0, form.Deadline.Location())
				deadlineUnix = timeutil.TimeStamp(deadline.Unix())
			}
		}

		if err := issues_model.UpdateIssueDeadline(ctx, issue, deadlineUnix, ctx.Doer); err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateIssueDeadline", err)
			return
		}
		issue.DeadlineUnix = deadlineUnix
	}

	// Add/delete assignees

	// Deleting is done the GitHub way (quote from their api documentation):
	// https://developer.github.com/v3/issues/#edit-an-issue
	// "assignees" (array): Logins for Users to assign to this issue.
	// Pass one or more user logins to replace the set of assignees on this Issue.
	// Send an empty array ([]) to clear all assignees from the Issue.

	if canWrite && (form.Assignees != nil || form.Assignee != nil) {
		oneAssignee := ""
		if form.Assignee != nil {
			oneAssignee = *form.Assignee
		}

		err = issue_service.UpdateAssignees(ctx, issue, oneAssignee, form.Assignees, ctx.Doer)
		if err != nil {
			if errors.Is(err, user_model.ErrBlockedUser) {
				ctx.Error(http.StatusForbidden, "UpdateAssignees", err)
			} else {
				ctx.Error(http.StatusInternalServerError, "UpdateAssignees", err)
			}
			return
		}
	}

	if canWrite && form.Milestone != nil &&
		issue.MilestoneID != *form.Milestone {
		oldMilestoneID := issue.MilestoneID
		issue.MilestoneID = *form.Milestone
		if err = issue_service.ChangeMilestoneAssign(ctx, issue, ctx.Doer, oldMilestoneID); err != nil {
			ctx.Error(http.StatusInternalServerError, "ChangeMilestoneAssign", err)
			return
		}
	}
	if form.State != nil {
		if issue.IsPull {
			if err := issue.LoadPullRequest(ctx); err != nil {
				ctx.Error(http.StatusInternalServerError, "GetPullRequest", err)
				return
			}
			if issue.PullRequest.HasMerged {
				ctx.Error(http.StatusPreconditionFailed, "MergedPRState", "cannot change state of this pull request, it was already merged")
				return
			}
		}

		state := api.StateType(*form.State)
		closeOrReopenIssue(ctx, issue, state)
		if ctx.Written() {
			return
		}
	}

	// Refetch from database to assign some automatic values
	issue, err = issues_model.GetIssueByID(ctx, issue.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if err = issue.LoadMilestone(ctx); err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, ctx.Doer, issue))
}

func DeleteIssue(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index} issue issueDelete
	// ---
	// summary: Delete an issue
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of issue to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByID", err)
		}
		return
	}

	if err = issue_service.DeleteIssue(ctx, ctx.Doer, ctx.Repo.GitRepo, issue); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteIssueByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UpdateIssueDeadline updates an issue deadline
func UpdateIssueDeadline(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/deadline issue issueEditIssueDeadline
	// ---
	// summary: Set an issue deadline. If set to null, the deadline is deleted. If using deadline only the date will be taken into account, and time of day ignored.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue to create or update a deadline on
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditDeadlineOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/IssueDeadline"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.EditDeadlineOption)
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Error(http.StatusForbidden, "", "Not repo writer")
		return
	}

	deadlineUnix, _ := common.ParseAPIDeadlineToEndOfDay(form.Deadline)
	if err := issues_model.UpdateIssueDeadline(ctx, issue, deadlineUnix, ctx.Doer); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateIssueDeadline", err)
		return
	}

	ctx.JSON(http.StatusCreated, api.IssueDeadline{Deadline: deadlineUnix.AsTimePtr()})
}

func closeOrReopenIssue(ctx *context.APIContext, issue *issues_model.Issue, state api.StateType) {
	if state != api.StateOpen && state != api.StateClosed {
		ctx.Error(http.StatusPreconditionFailed, "UnknownIssueStateError", fmt.Sprintf("unknown state: %s", state))
		return
	}

	if state == api.StateClosed && !issue.IsClosed {
		if err := issue_service.CloseIssue(ctx, issue, ctx.Doer, ""); err != nil {
			if issues_model.IsErrDependenciesLeft(err) {
				ctx.Error(http.StatusPreconditionFailed, "DependenciesLeft", "cannot close this issue or pull request because it still has open dependencies")
				return
			}
			ctx.Error(http.StatusInternalServerError, "CloseIssue", err)
			return
		}
	} else if state == api.StateOpen && issue.IsClosed {
		if err := issue_service.ReopenIssue(ctx, issue, ctx.Doer, ""); err != nil {
			ctx.Error(http.StatusInternalServerError, "ReopenIssue", err)
			return
		}
	}
}
