// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	api "code.gitea.io/sdk/gitea"
)

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
	// - name: page
	//   in: query
	//   description: page number of requested issues
	//   type: integer
	// - name: q
	//   in: query
	//   description: search string
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
	var err error
	if len(keyword) > 0 {
		issueIDs, err = indexer.SearchIssuesByKeyword(ctx.Repo.Repository.ID, keyword)
	}

	// Only fetch the issues if we either don't have a keyword or the search returned issues
	// This would otherwise return all issues if no issues were found by the search.
	if len(keyword) == 0 || len(issueIDs) > 0 {
		issues, err = models.Issues(&models.IssuesOptions{
			RepoIDs:  []int64{ctx.Repo.Repository.ID},
			Page:     ctx.QueryInt("page"),
			PageSize: setting.UI.IssuePagingNum,
			IsClosed: isClosed,
			IssueIDs: issueIDs,
		})
	}

	if err != nil {
		ctx.Error(500, "Issues", err)
		return
	}

	apiIssues := make([]*api.Issue, len(issues))
	for i := range issues {
		apiIssues[i] = issues[i].APIFormat()
	}

	ctx.SetLinkHeader(ctx.Repo.Repository.NumIssues, setting.UI.IssuePagingNum)
	ctx.JSON(200, &apiIssues)
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
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Issue"
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}
	ctx.JSON(200, issue.APIFormat())
}

// CreateIssue create an issue of a repository
func CreateIssue(ctx *context.APIContext, form api.CreateIssueOption) {
	// swagger:operation POST /repos/{owner}/{repo}/issues issue issueCreateIssue
	// ---
	// summary: Create an issue
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

	var deadlineUnix util.TimeStamp
	if form.Deadline != nil && ctx.Repo.IsWriter() {
		deadlineUnix = util.TimeStamp(form.Deadline.Unix())
	}

	issue := &models.Issue{
		RepoID:       ctx.Repo.Repository.ID,
		Title:        form.Title,
		PosterID:     ctx.User.ID,
		Poster:       ctx.User,
		Content:      form.Body,
		DeadlineUnix: deadlineUnix,
	}

	var assigneeIDs = make([]int64, 0)
	var err error
	if ctx.Repo.IsWriter() {
		issue.MilestoneID = form.Milestone
		assigneeIDs, err = models.MakeIDsFromAPIAssigneesToAdd(form.Assignee, form.Assignees)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(422, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
			} else {
				ctx.Error(500, "AddAssigneeByName", err)
			}
			return
		}
	} else {
		// setting labels is not allowed if user is not a writer
		form.Labels = make([]int64, 0)
	}

	if err := models.NewIssue(ctx.Repo.Repository, issue, form.Labels, assigneeIDs, nil); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(400, "UserDoesNotHaveAccessToRepo", err)
			return
		}
		ctx.Error(500, "NewIssue", err)
		return
	}

	if form.Closed {
		if err := issue.ChangeStatus(ctx.User, ctx.Repo.Repository, true); err != nil {
			ctx.Error(500, "ChangeStatus", err)
			return
		}
	}

	// Refetch from database to assign some automatic values
	issue, err = models.GetIssueByID(issue.ID)
	if err != nil {
		ctx.Error(500, "GetIssueByID", err)
		return
	}
	ctx.JSON(201, issue.APIFormat())
}

// EditIssue modify an issue of a repository
func EditIssue(ctx *context.APIContext, form api.EditIssueOption) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/{index} issue issueEditIssue
	// ---
	// summary: Edit an issue
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
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditIssueOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Issue"
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	if !issue.IsPoster(ctx.User.ID) && !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	if len(form.Title) > 0 {
		issue.Title = form.Title
	}
	if form.Body != nil {
		issue.Content = *form.Body
	}

	// Update the deadline
	var deadlineUnix util.TimeStamp
	if form.Deadline != nil && !form.Deadline.IsZero() {
		deadlineUnix = util.TimeStamp(form.Deadline.Unix())
	}

	if err := models.UpdateIssueDeadline(issue, deadlineUnix, ctx.User); err != nil {
		ctx.Error(500, "UpdateIssueDeadline", err)
		return
	}

	// Add/delete assignees

	// Deleting is done the Github way (quote from their api documentation):
	// https://developer.github.com/v3/issues/#edit-an-issue
	// "assignees" (array): Logins for Users to assign to this issue.
	// Pass one or more user logins to replace the set of assignees on this Issue.
	// Send an empty array ([]) to clear all assignees from the Issue.

	if ctx.Repo.IsWriter() && (form.Assignees != nil || form.Assignee != nil) {

		oneAssignee := ""
		if form.Assignee != nil {
			oneAssignee = *form.Assignee
		}

		err = models.UpdateAPIAssignee(issue, oneAssignee, form.Assignees, ctx.User)
		if err != nil {
			ctx.Error(500, "UpdateAPIAssignee", err)
			return
		}
	}

	if ctx.Repo.IsWriter() && form.Milestone != nil &&
		issue.MilestoneID != *form.Milestone {
		oldMilestoneID := issue.MilestoneID
		issue.MilestoneID = *form.Milestone
		if err = models.ChangeMilestoneAssign(issue, ctx.User, oldMilestoneID); err != nil {
			ctx.Error(500, "ChangeMilestoneAssign", err)
			return
		}
	}

	if err = models.UpdateIssue(issue); err != nil {
		ctx.Error(500, "UpdateIssue", err)
		return
	}
	if form.State != nil {
		if err = issue.ChangeStatus(ctx.User, ctx.Repo.Repository, api.StateClosed == api.StateType(*form.State)); err != nil {
			ctx.Error(500, "ChangeStatus", err)
			return
		}
	}

	// Refetch from database to assign some automatic values
	issue, err = models.GetIssueByID(issue.ID)
	if err != nil {
		ctx.Error(500, "GetIssueByID", err)
		return
	}
	ctx.JSON(201, issue.APIFormat())
}
