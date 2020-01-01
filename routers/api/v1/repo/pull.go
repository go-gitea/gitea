// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	milestone_service "code.gitea.io/gitea/services/milestone"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ListPullRequests returns a list of all PRs
func ListPullRequests(ctx *context.APIContext, form api.ListPullRequestsOptions) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls repository repoListPullRequests
	// ---
	// summary: List a repo's pull requests
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
	// - name: page
	//   in: query
	//   description: Page number
	//   type: integer
	// - name: state
	//   in: query
	//   description: "State of pull request: open or closed (optional)"
	//   type: string
	//   enum: [closed, open, all]
	// - name: sort
	//   in: query
	//   description: "Type of sort"
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate, mostcomment, leastcomment, priority]
	// - name: milestone
	//   in: query
	//   description: "ID of the milestone"
	//   type: integer
	//   format: int64
	// - name: labels
	//   in: query
	//   description: "Label IDs"
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: integer
	//     format: int64
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullRequestList"
	prs, maxResults, err := models.PullRequests(ctx.Repo.Repository.ID, &models.PullRequestsOptions{
		Page:        ctx.QueryInt("page"),
		State:       ctx.QueryTrim("state"),
		SortType:    ctx.QueryTrim("sort"),
		Labels:      ctx.QueryStrings("labels"),
		MilestoneID: ctx.QueryInt64("milestone"),
	})

	if err != nil {
		ctx.Error(500, "PullRequests", err)
		return
	}

	apiPrs := make([]*api.PullRequest, len(prs))
	for i := range prs {
		if err = prs[i].LoadIssue(); err != nil {
			ctx.Error(500, "LoadIssue", err)
			return
		}
		if err = prs[i].LoadAttributes(); err != nil {
			ctx.Error(500, "LoadAttributes", err)
			return
		}
		if err = prs[i].GetBaseRepo(); err != nil {
			ctx.Error(500, "GetBaseRepo", err)
			return
		}
		if err = prs[i].GetHeadRepo(); err != nil {
			ctx.Error(500, "GetHeadRepo", err)
			return
		}
		apiPrs[i] = prs[i].APIFormat()
	}

	ctx.SetLinkHeader(int(maxResults), models.ItemsPerPage)
	ctx.JSON(200, &apiPrs)
}

// GetPullRequest returns a single PR based on index
func GetPullRequest(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index} repository repoGetPullRequest
	// ---
	// summary: Get a pull request
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
	//   description: index of the pull request to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullRequest"
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.GetBaseRepo(); err != nil {
		ctx.Error(500, "GetBaseRepo", err)
		return
	}
	if err = pr.GetHeadRepo(); err != nil {
		ctx.Error(500, "GetHeadRepo", err)
		return
	}
	ctx.JSON(200, pr.APIFormat())
}

// CreatePullRequest does what it says
func CreatePullRequest(ctx *context.APIContext, form api.CreatePullRequestOption) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls repository repoCreatePullRequest
	// ---
	// summary: Create a pull request
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
	//     "$ref": "#/definitions/CreatePullRequestOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PullRequest"
	var (
		repo        = ctx.Repo.Repository
		labelIDs    []int64
		assigneeID  int64
		milestoneID int64
	)

	// Get repo/branch information
	_, headRepo, headGitRepo, compareInfo, baseBranch, headBranch := parseCompareInfo(ctx, form)
	if ctx.Written() {
		return
	}
	defer headGitRepo.Close()

	// Check if another PR exists with the same targets
	existingPr, err := models.GetUnmergedPullRequest(headRepo.ID, ctx.Repo.Repository.ID, headBranch, baseBranch)
	if err != nil {
		if !models.IsErrPullRequestNotExist(err) {
			ctx.Error(500, "GetUnmergedPullRequest", err)
			return
		}
	} else {
		err = models.ErrPullRequestAlreadyExists{
			ID:         existingPr.ID,
			IssueID:    existingPr.Index,
			HeadRepoID: existingPr.HeadRepoID,
			BaseRepoID: existingPr.BaseRepoID,
			HeadBranch: existingPr.HeadBranch,
			BaseBranch: existingPr.BaseBranch,
		}
		ctx.Error(409, "GetUnmergedPullRequest", err)
		return
	}

	if len(form.Labels) > 0 {
		labels, err := models.GetLabelsInRepoByIDs(ctx.Repo.Repository.ID, form.Labels)
		if err != nil {
			ctx.Error(500, "GetLabelsInRepoByIDs", err)
			return
		}

		labelIDs = make([]int64, len(labels))
		for i := range labels {
			labelIDs[i] = labels[i].ID
		}
	}

	if form.Milestone > 0 {
		milestone, err := models.GetMilestoneByRepoID(ctx.Repo.Repository.ID, milestoneID)
		if err != nil {
			if models.IsErrMilestoneNotExist(err) {
				ctx.NotFound()
			} else {
				ctx.Error(500, "GetMilestoneByRepoID", err)
			}
			return
		}

		milestoneID = milestone.ID
	}

	patch, err := headGitRepo.GetPatch(compareInfo.MergeBase, headBranch)
	if err != nil {
		ctx.Error(500, "GetPatch", err)
		return
	}

	var deadlineUnix timeutil.TimeStamp
	if form.Deadline != nil {
		deadlineUnix = timeutil.TimeStamp(form.Deadline.Unix())
	}

	prIssue := &models.Issue{
		RepoID:       repo.ID,
		Title:        form.Title,
		PosterID:     ctx.User.ID,
		Poster:       ctx.User,
		MilestoneID:  milestoneID,
		AssigneeID:   assigneeID,
		IsPull:       true,
		Content:      form.Body,
		DeadlineUnix: deadlineUnix,
	}
	pr := &models.PullRequest{
		HeadRepoID: headRepo.ID,
		BaseRepoID: repo.ID,
		HeadBranch: headBranch,
		BaseBranch: baseBranch,
		HeadRepo:   headRepo,
		BaseRepo:   repo,
		MergeBase:  compareInfo.MergeBase,
		Type:       models.PullRequestGitea,
	}

	// Get all assignee IDs
	assigneeIDs, err := models.MakeIDsFromAPIAssigneesToAdd(form.Assignee, form.Assignees)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(422, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
		} else {
			ctx.Error(500, "AddAssigneeByName", err)
		}
		return
	}

	if err := pull_service.NewPullRequest(repo, prIssue, labelIDs, []string{}, pr, patch, assigneeIDs); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(400, "UserDoesNotHaveAccessToRepo", err)
			return
		}
		ctx.Error(500, "NewPullRequest", err)
		return
	} else if err := pr.PushToBaseRepo(); err != nil {
		ctx.Error(500, "PushToBaseRepo", err)
		return
	}

	notification.NotifyNewPullRequest(pr)

	log.Trace("Pull request created: %d/%d", repo.ID, prIssue.ID)
	ctx.JSON(201, pr.APIFormat())
}

// EditPullRequest does what it says
func EditPullRequest(ctx *context.APIContext, form api.EditPullRequestOption) {
	// swagger:operation PATCH /repos/{owner}/{repo}/pulls/{index} repository repoEditPullRequest
	// ---
	// summary: Update a pull request
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
	//   description: index of the pull request to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditPullRequestOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PullRequest"
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	err = pr.LoadIssue()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}
	issue := pr.Issue
	issue.Repo = ctx.Repo.Repository

	if !issue.IsPoster(ctx.User.ID) && !ctx.Repo.CanWrite(models.UnitTypePullRequests) {
		ctx.Status(403)
		return
	}

	if len(form.Title) > 0 {
		issue.Title = form.Title
	}
	if len(form.Body) > 0 {
		issue.Content = form.Body
	}

	// Update Deadline
	if form.Deadline != nil {
		deadlineUnix := timeutil.TimeStamp(form.Deadline.Unix())
		if err := models.UpdateIssueDeadline(issue, deadlineUnix, ctx.User); err != nil {
			ctx.Error(500, "UpdateIssueDeadline", err)
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

	if ctx.Repo.CanWrite(models.UnitTypePullRequests) && (form.Assignees != nil || len(form.Assignee) > 0) {
		err = models.UpdateAPIAssignee(issue, form.Assignee, form.Assignees, ctx.User)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(422, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
			} else {
				ctx.Error(500, "UpdateAPIAssignee", err)
			}
			return
		}
	}

	if ctx.Repo.CanWrite(models.UnitTypePullRequests) && form.Milestone != 0 &&
		issue.MilestoneID != form.Milestone {
		oldMilestoneID := issue.MilestoneID
		issue.MilestoneID = form.Milestone
		if err = milestone_service.ChangeMilestoneAssign(issue, ctx.User, oldMilestoneID); err != nil {
			ctx.Error(500, "ChangeMilestoneAssign", err)
			return
		}
	}

	if ctx.Repo.CanWrite(models.UnitTypePullRequests) && form.Labels != nil {
		labels, err := models.GetLabelsInRepoByIDs(ctx.Repo.Repository.ID, form.Labels)
		if err != nil {
			ctx.Error(500, "GetLabelsInRepoByIDsError", err)
			return
		}
		if err = issue.ReplaceLabels(labels, ctx.User); err != nil {
			ctx.Error(500, "ReplaceLabelsError", err)
			return
		}
	}

	if err = models.UpdateIssueByAPI(issue); err != nil {
		ctx.InternalServerError(err)
		return
	}
	if form.State != nil {
		if err = issue.ChangeStatus(ctx.User, api.StateClosed == api.StateType(*form.State)); err != nil {
			if models.IsErrDependenciesLeft(err) {
				ctx.Error(http.StatusPreconditionFailed, "DependenciesLeft", "cannot close this pull request because it still has open dependencies")
				return
			}
			ctx.Error(500, "ChangeStatus", err)
			return
		}

		notification.NotifyIssueChangeStatus(ctx.User, issue, api.StateClosed == api.StateType(*form.State))
	}

	// Refetch from database
	pr, err = models.GetPullRequestByIndex(ctx.Repo.Repository.ID, pr.Index)
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	// TODO this should be 200, not 201
	ctx.JSON(201, pr.APIFormat())
}

// IsPullRequestMerged checks if a PR exists given an index
func IsPullRequestMerged(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/merge repository repoPullRequestIsMerged
	// ---
	// summary: Check if a pull request has been merged
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: pull request has been merged
	//   "404":
	//     description: pull request has not been merged
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	if pr.HasMerged {
		ctx.Status(204)
	}
	ctx.NotFound()
}

// MergePullRequest merges a PR given an index
func MergePullRequest(ctx *context.APIContext, form auth.MergePullRequestForm) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/merge repository repoMergePullRequest
	// ---
	// summary: Merge a pull request
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
	//   description: index of the pull request to merge
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     $ref: "#/definitions/MergePullRequestOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "405":
	//     "$ref": "#/responses/empty"
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.GetHeadRepo(); err != nil {
		ctx.ServerError("GetHeadRepo", err)
		return
	}

	err = pr.LoadIssue()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}
	pr.Issue.Repo = ctx.Repo.Repository

	if ctx.IsSigned {
		// Update issue-user.
		if err = pr.Issue.ReadBy(ctx.User.ID); err != nil {
			ctx.Error(500, "ReadBy", err)
			return
		}
	}

	if pr.Issue.IsClosed {
		ctx.NotFound()
		return
	}

	if !pr.CanAutoMerge() || pr.HasMerged || pr.IsWorkInProgress() {
		ctx.Status(405)
		return
	}

	isPass, err := pull_service.IsPullCommitStatusPass(pr)
	if err != nil {
		ctx.Error(500, "IsPullCommitStatusPass", err)
		return
	}

	if !isPass && !ctx.IsUserRepoAdmin() {
		ctx.Status(405)
		return
	}

	if len(form.Do) == 0 {
		form.Do = string(models.MergeStyleMerge)
	}

	message := strings.TrimSpace(form.MergeTitleField)
	if len(message) == 0 {
		if models.MergeStyle(form.Do) == models.MergeStyleMerge {
			message = pr.GetDefaultMergeMessage()
		}
		if models.MergeStyle(form.Do) == models.MergeStyleSquash {
			message = pr.GetDefaultSquashMessage()
		}
	}

	form.MergeMessageField = strings.TrimSpace(form.MergeMessageField)
	if len(form.MergeMessageField) > 0 {
		message += "\n\n" + form.MergeMessageField
	}

	if err := pull_service.Merge(pr, ctx.User, ctx.Repo.GitRepo, models.MergeStyle(form.Do), message); err != nil {
		if models.IsErrInvalidMergeStyle(err) {
			ctx.Status(405)
			return
		}
		ctx.Error(500, "Merge", err)
		return
	}

	log.Trace("Pull request merged: %d", pr.ID)
	ctx.Status(200)
}

func parseCompareInfo(ctx *context.APIContext, form api.CreatePullRequestOption) (*models.User, *models.Repository, *git.Repository, *git.CompareInfo, string, string) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches information
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	// TODO: Validate form first?

	baseBranch := form.Base

	var (
		headUser   *models.User
		headBranch string
		isSameRepo bool
		err        error
	)

	// If there is no head repository, it means pull request between same repository.
	headInfos := strings.Split(form.Head, ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headUser = ctx.Repo.Owner
		headBranch = headInfos[0]

	} else if len(headInfos) == 2 {
		headUser, err = models.GetUserByName(headInfos[0])
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.NotFound("GetUserByName")
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return nil, nil, nil, nil, "", ""
		}
		headBranch = headInfos[1]

	} else {
		ctx.NotFound()
		return nil, nil, nil, nil, "", ""
	}

	ctx.Repo.PullRequest.SameRepo = isSameRepo
	log.Info("Base branch: %s", baseBranch)
	log.Info("Repo path: %s", ctx.Repo.GitRepo.Path)
	// Check if base branch is valid.
	if !ctx.Repo.GitRepo.IsBranchExist(baseBranch) {
		ctx.NotFound("IsBranchExist")
		return nil, nil, nil, nil, "", ""
	}

	// Check if current user has fork of repository or in the same repository.
	headRepo, has := models.HasForkedRepo(headUser.ID, baseRepo.ID)
	if !has && !isSameRepo {
		log.Trace("parseCompareInfo[%d]: does not have fork or in same repository", baseRepo.ID)
		ctx.NotFound("HasForkedRepo")
		return nil, nil, nil, nil, "", ""
	}

	var headGitRepo *git.Repository
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
	} else {
		headGitRepo, err = git.OpenRepository(models.RepoPath(headUser.Name, headRepo.Name))
		if err != nil {
			ctx.Error(500, "OpenRepository", err)
			return nil, nil, nil, nil, "", ""
		}
	}

	// user should have permission to read baseRepo's codes and pulls, NOT headRepo's
	permBase, err := models.GetUserRepoPermission(baseRepo, ctx.User)
	if err != nil {
		headGitRepo.Close()
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permBase.CanReadIssuesOrPulls(true) || !permBase.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User %-v cannot create/read pull requests or cannot read code in Repo %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		headGitRepo.Close()
		ctx.NotFound("Can't read pulls or can't read UnitTypeCode")
		return nil, nil, nil, nil, "", ""
	}

	// user should have permission to read headrepo's codes
	permHead, err := models.GetUserRepoPermission(headRepo, ctx.User)
	if err != nil {
		headGitRepo.Close()
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permHead.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
				ctx.User,
				headRepo,
				permHead)
		}
		headGitRepo.Close()
		ctx.NotFound("Can't read headRepo UnitTypeCode")
		return nil, nil, nil, nil, "", ""
	}

	// Check if head branch is valid.
	if !headGitRepo.IsBranchExist(headBranch) {
		headGitRepo.Close()
		ctx.NotFound()
		return nil, nil, nil, nil, "", ""
	}

	compareInfo, err := headGitRepo.GetCompareInfo(models.RepoPath(baseRepo.Owner.Name, baseRepo.Name), baseBranch, headBranch)
	if err != nil {
		headGitRepo.Close()
		ctx.Error(500, "GetCompareInfo", err)
		return nil, nil, nil, nil, "", ""
	}

	return headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch
}
