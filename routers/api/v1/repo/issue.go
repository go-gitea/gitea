// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
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
	//   description: whether issue is open or closed
	//   type: string
	// - name: labels
	//   in: query
	//   description: comma separated list of labels. Fetch only issues that have any of this labels. Non existent labels are discarded
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of requested issues
	//   type: integer
	// - name: q
	//   in: query
	//   description: search string
	//   type: string
	// - name: priority_repo_id
	//   in: query
	//   description: repository to prioritize in the results
	//   type: integer
	//   format: int64
	// - name: type
	//   in: query
	//   description: filter by type (issues / pulls) if set
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"

	var isClosed util.OptionalBool
	switch ctx.Query("state") {
	case "closed":
		isClosed = util.OptionalBoolTrue
	case "all":
		isClosed = util.OptionalBoolNone
	default:
		isClosed = util.OptionalBoolFalse
	}

	// find repos user can access (for issue search)
	repoIDs := make([]int64, 0)
	opts := &models.SearchRepoOptions{
		PageSize:    15,
		Private:     false,
		AllPublic:   true,
		TopicOnly:   false,
		Collaborate: util.OptionalBoolNone,
		UserIsAdmin: ctx.IsUserSiteAdmin(),
		OrderBy:     models.SearchOrderByRecentUpdated,
	}
	if ctx.IsSigned {
		opts.Private = true
		opts.AllLimited = true
		opts.UserID = ctx.User.ID
	}
	issueCount := 0
	for page := 1; ; page++ {
		opts.Page = page
		repos, count, err := models.SearchRepositoryByName(opts)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "SearchRepositoryByName", err)
			return
		}

		if len(repos) == 0 {
			break
		}
		log.Trace("Processing next %d repos of %d", len(repos), count)
		for _, repo := range repos {
			switch isClosed {
			case util.OptionalBoolTrue:
				issueCount += repo.NumClosedIssues
			case util.OptionalBoolFalse:
				issueCount += repo.NumOpenIssues
			case util.OptionalBoolNone:
				issueCount += repo.NumIssues
			}
			repoIDs = append(repoIDs, repo.ID)
		}
	}

	var issues []*models.Issue

	keyword := strings.Trim(ctx.Query("q"), " ")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}
	var issueIDs []int64
	var labelIDs []int64
	var err error
	if len(keyword) > 0 && len(repoIDs) > 0 {
		issueIDs, err = issue_indexer.SearchIssuesByKeyword(repoIDs, keyword)
	}

	labels := ctx.Query("labels")
	if splitted := strings.Split(labels, ","); labels != "" && len(splitted) > 0 {
		labelIDs, err = models.GetLabelIDsInReposByNames(repoIDs, splitted)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelIDsInRepoByNames", err)
			return
		}
	}

	var isPull util.OptionalBool
	switch ctx.Query("type") {
	case "pulls":
		isPull = util.OptionalBoolTrue
	case "issues":
		isPull = util.OptionalBoolFalse
	default:
		isPull = util.OptionalBoolNone
	}

	// Only fetch the issues if we either don't have a keyword or the search returned issues
	// This would otherwise return all issues if no issues were found by the search.
	if len(keyword) == 0 || len(issueIDs) > 0 || len(labelIDs) > 0 {
		issues, err = models.Issues(&models.IssuesOptions{
			RepoIDs:        repoIDs,
			Page:           ctx.QueryInt("page"),
			PageSize:       setting.UI.IssuePagingNum,
			IsClosed:       isClosed,
			IssueIDs:       issueIDs,
			LabelIDs:       labelIDs,
			SortType:       "priorityrepo",
			PriorityRepoID: ctx.QueryInt64("priority_repo_id"),
			IsPull:         isPull,
		})
	}

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Issues", err)
		return
	}

	apiIssues := make([]*api.Issue, len(issues))
	for i := range issues {
		apiIssues[i] = issues[i].APIFormat()
	}

	ctx.SetLinkHeader(issueCount, setting.UI.IssuePagingNum)
	ctx.JSON(http.StatusOK, &apiIssues)
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
	// - name: labels
	//   in: query
	//   description: comma separated list of labels. Fetch only issues that have any of this labels. Non existent labels are discarded
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of requested issues
	//   type: integer
	// - name: q
	//   in: query
	//   description: search string
	//   type: string
	// - name: type
	//   in: query
	//   description: filter by type (issues / pulls) if set
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/IssueList"

	var isClosed util.OptionalBool
	switch ctx.Query("state") {
	case "closed":
		isClosed = util.OptionalBoolTrue
	case "all":
		isClosed = util.OptionalBoolNone
	default:
		isClosed = util.OptionalBoolFalse
	}

	var issues []*models.Issue

	keyword := strings.Trim(ctx.Query("q"), " ")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}
	var issueIDs []int64
	var labelIDs []int64
	var err error
	if len(keyword) > 0 {
		issueIDs, err = issue_indexer.SearchIssuesByKeyword([]int64{ctx.Repo.Repository.ID}, keyword)
	}

	if splitted := strings.Split(ctx.Query("labels"), ","); len(splitted) > 0 {
		labelIDs, err = models.GetLabelIDsInRepoByNames(ctx.Repo.Repository.ID, splitted)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelIDsInRepoByNames", err)
			return
		}
	}

	var isPull util.OptionalBool
	switch ctx.Query("type") {
	case "pulls":
		isPull = util.OptionalBoolTrue
	case "issues":
		isPull = util.OptionalBoolFalse
	default:
		isPull = util.OptionalBoolNone
	}

	// Only fetch the issues if we either don't have a keyword or the search returned issues
	// This would otherwise return all issues if no issues were found by the search.
	if len(keyword) == 0 || len(issueIDs) > 0 || len(labelIDs) > 0 {
		issues, err = models.Issues(&models.IssuesOptions{
			RepoIDs:  []int64{ctx.Repo.Repository.ID},
			Page:     ctx.QueryInt("page"),
			PageSize: setting.UI.IssuePagingNum,
			IsClosed: isClosed,
			IssueIDs: issueIDs,
			LabelIDs: labelIDs,
			IsPull:   isPull,
		})
	}

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Issues", err)
		return
	}

	apiIssues := make([]*api.Issue, len(issues))
	for i := range issues {
		apiIssues[i] = issues[i].APIFormat()
	}

	ctx.SetLinkHeader(ctx.Repo.Repository.NumIssues, setting.UI.IssuePagingNum)
	ctx.JSON(http.StatusOK, &apiIssues)
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

	issue, err := models.GetIssueWithAttrsByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}
	ctx.JSON(http.StatusOK, issue.APIFormat())
}

// CreateIssue create an issue of a repository
func CreateIssue(ctx *context.APIContext, form api.CreateIssueOption) {
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
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	var deadlineUnix timeutil.TimeStamp
	if form.Deadline != nil && ctx.Repo.CanWrite(models.UnitTypeIssues) {
		deadlineUnix = timeutil.TimeStamp(form.Deadline.Unix())
	}

	issue := &models.Issue{
		RepoID:       ctx.Repo.Repository.ID,
		Repo:         ctx.Repo.Repository,
		Title:        form.Title,
		PosterID:     ctx.User.ID,
		Poster:       ctx.User,
		Content:      form.Body,
		DeadlineUnix: deadlineUnix,
	}

	var assigneeIDs = make([]int64, 0)
	var err error
	if ctx.Repo.CanWrite(models.UnitTypeIssues) {
		issue.MilestoneID = form.Milestone
		assigneeIDs, err = models.MakeIDsFromAPIAssigneesToAdd(form.Assignee, form.Assignees)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
			} else {
				ctx.Error(http.StatusInternalServerError, "AddAssigneeByName", err)
			}
			return
		}

		// Check if the passed assignees is assignable
		for _, aID := range assigneeIDs {
			assignee, err := models.GetUserByID(aID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetUserByID", err)
				return
			}

			valid, err := models.CanBeAssigned(assignee, ctx.Repo.Repository, false)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "canBeAssigned", err)
				return
			}
			if !valid {
				ctx.Error(http.StatusUnprocessableEntity, "canBeAssigned", models.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: ctx.Repo.Repository.Name})
				return
			}
		}
	} else {
		// setting labels is not allowed if user is not a writer
		form.Labels = make([]int64, 0)
	}

	if err := issue_service.NewIssue(ctx.Repo.Repository, issue, form.Labels, nil, assigneeIDs); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "NewIssue", err)
		return
	}

	if form.Closed {
		if err := issue_service.ChangeStatus(issue, ctx.User, true); err != nil {
			if models.IsErrDependenciesLeft(err) {
				ctx.Error(http.StatusPreconditionFailed, "DependenciesLeft", "cannot close this issue because it still has open dependencies")
				return
			}
			ctx.Error(http.StatusInternalServerError, "ChangeStatus", err)
			return
		}
	}

	// Refetch from database to assign some automatic values
	issue, err = models.GetIssueByID(issue.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByID", err)
		return
	}
	ctx.JSON(http.StatusCreated, issue.APIFormat())
}

// EditIssue modify an issue of a repository
func EditIssue(ctx *context.APIContext, form api.EditIssueOption) {
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

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}
	issue.Repo = ctx.Repo.Repository
	canWrite := ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)

	err = issue.LoadAttributes()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	if !issue.IsPoster(ctx.User.ID) && !canWrite {
		ctx.Status(http.StatusForbidden)
		return
	}

	if len(form.Title) > 0 {
		issue.Title = form.Title
	}
	if form.Body != nil {
		issue.Content = *form.Body
	}

	// Update or remove the deadline, only if set and allowed
	if (form.Deadline != nil || form.RemoveDeadline != nil) && canWrite {
		var deadlineUnix timeutil.TimeStamp

		if (form.RemoveDeadline == nil || !*form.RemoveDeadline) && !form.Deadline.IsZero() {
			deadline := time.Date(form.Deadline.Year(), form.Deadline.Month(), form.Deadline.Day(),
				23, 59, 59, 0, form.Deadline.Location())
			deadlineUnix = timeutil.TimeStamp(deadline.Unix())
		}

		if err := models.UpdateIssueDeadline(issue, deadlineUnix, ctx.User); err != nil {
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

		err = issue_service.UpdateAssignees(issue, oneAssignee, form.Assignees, ctx.User)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateAssignees", err)
			return
		}
	}

	if canWrite && form.Milestone != nil &&
		issue.MilestoneID != *form.Milestone {
		oldMilestoneID := issue.MilestoneID
		issue.MilestoneID = *form.Milestone
		if err = issue_service.ChangeMilestoneAssign(issue, ctx.User, oldMilestoneID); err != nil {
			ctx.Error(http.StatusInternalServerError, "ChangeMilestoneAssign", err)
			return
		}
	}

	if err = models.UpdateIssueByAPI(issue); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateIssueByAPI", err)
		return
	}
	if form.State != nil {
		if err = issue_service.ChangeStatus(issue, ctx.User, api.StateClosed == api.StateType(*form.State)); err != nil {
			if models.IsErrDependenciesLeft(err) {
				ctx.Error(http.StatusPreconditionFailed, "DependenciesLeft", "cannot close this issue because it still has open dependencies")
				return
			}
			ctx.Error(http.StatusInternalServerError, "ChangeStatus", err)
			return
		}
	}

	// Refetch from database to assign some automatic values
	issue, err = models.GetIssueByID(issue.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if err = issue.LoadMilestone(); err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, issue.APIFormat())
}

// UpdateIssueDeadline updates an issue deadline
func UpdateIssueDeadline(ctx *context.APIContext, form api.EditDeadlineOption) {
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

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
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

	var deadlineUnix timeutil.TimeStamp
	var deadline time.Time
	if form.Deadline != nil && !form.Deadline.IsZero() {
		deadline = time.Date(form.Deadline.Year(), form.Deadline.Month(), form.Deadline.Day(),
			23, 59, 59, 0, form.Deadline.Location())
		deadlineUnix = timeutil.TimeStamp(deadline.Unix())
	}

	if err := models.UpdateIssueDeadline(issue, deadlineUnix, ctx.User); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateIssueDeadline", err)
		return
	}

	ctx.JSON(http.StatusCreated, api.IssueDeadline{Deadline: &deadline})
}
