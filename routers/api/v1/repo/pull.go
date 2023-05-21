// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	activities_model "code.gitea.io/gitea/models/activities"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/automerge"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/gitdiff"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
)

// ListPullRequests returns a list of all PRs
func ListPullRequests(ctx *context.APIContext) {
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
	//     "$ref": "#/responses/PullRequestList"

	listOptions := utils.GetListOptions(ctx)

	prs, maxResults, err := issues_model.PullRequests(ctx.Repo.Repository.ID, &issues_model.PullRequestsOptions{
		ListOptions: listOptions,
		State:       ctx.FormTrim("state"),
		SortType:    ctx.FormTrim("sort"),
		Labels:      ctx.FormStrings("labels"),
		MilestoneID: ctx.FormInt64("milestone"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "PullRequests", err)
		return
	}

	apiPrs := make([]*api.PullRequest, len(prs))
	for i := range prs {
		if err = prs[i].LoadIssue(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
			return
		}
		if err = prs[i].LoadAttributes(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
			return
		}
		if err = prs[i].LoadBaseRepo(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadBaseRepo", err)
			return
		}
		if err = prs[i].LoadHeadRepo(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadHeadRepo", err)
			return
		}
		apiPrs[i] = convert.ToAPIPullRequest(ctx, prs[i], ctx.Doer)
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &apiPrs)
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.LoadBaseRepo(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadBaseRepo", err)
		return
	}
	if err = pr.LoadHeadRepo(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadHeadRepo", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIPullRequest(ctx, pr, ctx.Doer))
}

// DownloadPullDiffOrPatch render a pull's raw diff or patch
func DownloadPullDiffOrPatch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}.{diffType} repository repoDownloadPullDiffOrPatch
	// ---
	// summary: Get a pull request diff or patch
	// produces:
	// - text/plain
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
	// - name: diffType
	//   in: path
	//   description: whether the output is diff or patch
	//   type: string
	//   enum: [diff, patch]
	//   required: true
	// - name: binary
	//   in: query
	//   description: whether to include binary file changes. if true, the diff is applicable with `git apply`
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/string"
	//   "404":
	//     "$ref": "#/responses/notFound"
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.InternalServerError(err)
		}
		return
	}
	var patch bool
	if ctx.Params(":diffType") == "diff" {
		patch = false
	} else {
		patch = true
	}

	binary := ctx.FormBool("binary")

	if err := pull_service.DownloadDiffOrPatch(ctx, pr, ctx, patch, binary); err != nil {
		ctx.InternalServerError(err)
		return
	}
}

// CreatePullRequest does what it says
func CreatePullRequest(ctx *context.APIContext) {
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
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := *web.GetForm(ctx).(*api.CreatePullRequestOption)
	if form.Head == form.Base {
		ctx.Error(http.StatusUnprocessableEntity, "BaseHeadSame",
			"Invalid PullRequest: There are no changes between the head and the base")
		return
	}

	var (
		repo        = ctx.Repo.Repository
		labelIDs    []int64
		milestoneID int64
	)

	// Get repo/branch information
	_, headRepo, headGitRepo, compareInfo, baseBranch, headBranch := parseCompareInfo(ctx, form)
	if ctx.Written() {
		return
	}
	defer headGitRepo.Close()

	// Check if another PR exists with the same targets
	existingPr, err := issues_model.GetUnmergedPullRequest(ctx, headRepo.ID, ctx.Repo.Repository.ID, headBranch, baseBranch, issues_model.PullRequestFlowGithub)
	if err != nil {
		if !issues_model.IsErrPullRequestNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetUnmergedPullRequest", err)
			return
		}
	} else {
		err = issues_model.ErrPullRequestAlreadyExists{
			ID:         existingPr.ID,
			IssueID:    existingPr.Index,
			HeadRepoID: existingPr.HeadRepoID,
			BaseRepoID: existingPr.BaseRepoID,
			HeadBranch: existingPr.HeadBranch,
			BaseBranch: existingPr.BaseBranch,
		}
		ctx.Error(http.StatusConflict, "GetUnmergedPullRequest", err)
		return
	}

	if len(form.Labels) > 0 {
		labels, err := issues_model.GetLabelsInRepoByIDs(ctx, ctx.Repo.Repository.ID, form.Labels)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelsInRepoByIDs", err)
			return
		}

		labelIDs = make([]int64, len(form.Labels))
		orgLabelIDs := make([]int64, len(form.Labels))

		for i := range labels {
			labelIDs[i] = labels[i].ID
		}

		if ctx.Repo.Owner.IsOrganization() {
			orgLabels, err := issues_model.GetLabelsInOrgByIDs(ctx, ctx.Repo.Owner.ID, form.Labels)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetLabelsInOrgByIDs", err)
				return
			}

			for i := range orgLabels {
				orgLabelIDs[i] = orgLabels[i].ID
			}
		}

		labelIDs = append(labelIDs, orgLabelIDs...)
	}

	if form.Milestone > 0 {
		milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, form.Milestone)
		if err != nil {
			if issues_model.IsErrMilestoneNotExist(err) {
				ctx.NotFound()
			} else {
				ctx.Error(http.StatusInternalServerError, "GetMilestoneByRepoID", err)
			}
			return
		}

		milestoneID = milestone.ID
	}

	var deadlineUnix timeutil.TimeStamp
	if form.Deadline != nil {
		deadlineUnix = timeutil.TimeStamp(form.Deadline.Unix())
	}

	prIssue := &issues_model.Issue{
		RepoID:       repo.ID,
		Title:        form.Title,
		PosterID:     ctx.Doer.ID,
		Poster:       ctx.Doer,
		MilestoneID:  milestoneID,
		IsPull:       true,
		Content:      form.Body,
		DeadlineUnix: deadlineUnix,
	}
	pr := &issues_model.PullRequest{
		HeadRepoID: headRepo.ID,
		BaseRepoID: repo.ID,
		HeadBranch: headBranch,
		BaseBranch: baseBranch,
		HeadRepo:   headRepo,
		BaseRepo:   repo,
		MergeBase:  compareInfo.MergeBase,
		Type:       issues_model.PullRequestGitea,
	}

	// Get all assignee IDs
	assigneeIDs, err := issues_model.MakeIDsFromAPIAssigneesToAdd(ctx, form.Assignee, form.Assignees)
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

		valid, err := access_model.CanBeAssigned(ctx, assignee, repo, true)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "canBeAssigned", err)
			return
		}
		if !valid {
			ctx.Error(http.StatusUnprocessableEntity, "canBeAssigned", repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: repo.Name})
			return
		}
	}

	if err := pull_service.NewPullRequest(ctx, repo, prIssue, labelIDs, []string{}, pr, assigneeIDs); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "NewPullRequest", err)
		return
	}

	log.Trace("Pull request created: %d/%d", repo.ID, prIssue.ID)
	ctx.JSON(http.StatusCreated, convert.ToAPIPullRequest(ctx, pr, ctx.Doer))
}

// EditPullRequest does what it says
func EditPullRequest(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/pulls/{index} repository repoEditPullRequest
	// ---
	// summary: Update a pull request. If using deadline only the date will be taken into account, and time of day ignored.
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.EditPullRequestOption)
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	err = pr.LoadIssue(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}
	issue := pr.Issue
	issue.Repo = ctx.Repo.Repository

	if err := issue.LoadAttributes(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	if !issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWrite(unit.TypePullRequests) {
		ctx.Status(http.StatusForbidden)
		return
	}

	oldTitle := issue.Title
	if len(form.Title) > 0 {
		issue.Title = form.Title
	}
	if len(form.Body) > 0 {
		issue.Content = form.Body
	}

	// Update or remove deadline if set
	if form.Deadline != nil || form.RemoveDeadline != nil {
		var deadlineUnix timeutil.TimeStamp
		if (form.RemoveDeadline == nil || !*form.RemoveDeadline) && !form.Deadline.IsZero() {
			deadline := time.Date(form.Deadline.Year(), form.Deadline.Month(), form.Deadline.Day(),
				23, 59, 59, 0, form.Deadline.Location())
			deadlineUnix = timeutil.TimeStamp(deadline.Unix())
		}

		if err := issues_model.UpdateIssueDeadline(issue, deadlineUnix, ctx.Doer); err != nil {
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

	if ctx.Repo.CanWrite(unit.TypePullRequests) && (form.Assignees != nil || len(form.Assignee) > 0) {
		err = issue_service.UpdateAssignees(ctx, issue, form.Assignee, form.Assignees, ctx.Doer)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
			} else {
				ctx.Error(http.StatusInternalServerError, "UpdateAssignees", err)
			}
			return
		}
	}

	if ctx.Repo.CanWrite(unit.TypePullRequests) && form.Milestone != 0 &&
		issue.MilestoneID != form.Milestone {
		oldMilestoneID := issue.MilestoneID
		issue.MilestoneID = form.Milestone
		if err = issue_service.ChangeMilestoneAssign(issue, ctx.Doer, oldMilestoneID); err != nil {
			ctx.Error(http.StatusInternalServerError, "ChangeMilestoneAssign", err)
			return
		}
	}

	if ctx.Repo.CanWrite(unit.TypePullRequests) && form.Labels != nil {
		labels, err := issues_model.GetLabelsInRepoByIDs(ctx, ctx.Repo.Repository.ID, form.Labels)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelsInRepoByIDsError", err)
			return
		}

		if ctx.Repo.Owner.IsOrganization() {
			orgLabels, err := issues_model.GetLabelsInOrgByIDs(ctx, ctx.Repo.Owner.ID, form.Labels)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetLabelsInOrgByIDs", err)
				return
			}

			labels = append(labels, orgLabels...)
		}

		if err = issues_model.ReplaceIssueLabels(issue, labels, ctx.Doer); err != nil {
			ctx.Error(http.StatusInternalServerError, "ReplaceLabelsError", err)
			return
		}
	}

	if form.State != nil {
		if pr.HasMerged {
			ctx.Error(http.StatusPreconditionFailed, "MergedPRState", "cannot change state of this pull request, it was already merged")
			return
		}
		issue.IsClosed = api.StateClosed == api.StateType(*form.State)
	}
	statusChangeComment, titleChanged, err := issues_model.UpdateIssueByAPI(issue, ctx.Doer)
	if err != nil {
		if issues_model.IsErrDependenciesLeft(err) {
			ctx.Error(http.StatusPreconditionFailed, "DependenciesLeft", "cannot close this pull request because it still has open dependencies")
			return
		}
		ctx.Error(http.StatusInternalServerError, "UpdateIssueByAPI", err)
		return
	}

	if titleChanged {
		notification.NotifyIssueChangeTitle(ctx, ctx.Doer, issue, oldTitle)
	}

	if statusChangeComment != nil {
		notification.NotifyIssueChangeStatus(ctx, ctx.Doer, "", issue, statusChangeComment, issue.IsClosed)
	}

	// change pull target branch
	if !pr.HasMerged && len(form.Base) != 0 && form.Base != pr.BaseBranch {
		if !ctx.Repo.GitRepo.IsBranchExist(form.Base) {
			ctx.Error(http.StatusNotFound, "NewBaseBranchNotExist", fmt.Errorf("new base '%s' not exist", form.Base))
			return
		}
		if err := pull_service.ChangeTargetBranch(ctx, pr, ctx.Doer, form.Base); err != nil {
			if issues_model.IsErrPullRequestAlreadyExists(err) {
				ctx.Error(http.StatusConflict, "IsErrPullRequestAlreadyExists", err)
				return
			} else if issues_model.IsErrIssueIsClosed(err) {
				ctx.Error(http.StatusUnprocessableEntity, "IsErrIssueIsClosed", err)
				return
			} else if models.IsErrPullRequestHasMerged(err) {
				ctx.Error(http.StatusConflict, "IsErrPullRequestHasMerged", err)
				return
			} else {
				ctx.InternalServerError(err)
			}
			return
		}
		notification.NotifyPullRequestChangeTargetBranch(ctx, ctx.Doer, pr, form.Base)
	}

	// update allow edits
	if form.AllowMaintainerEdit != nil {
		if err := pull_service.SetAllowEdits(ctx, ctx.Doer, pr, *form.AllowMaintainerEdit); err != nil {
			if errors.Is(pull_service.ErrUserHasNoPermissionForAction, err) {
				ctx.Error(http.StatusForbidden, "SetAllowEdits", fmt.Sprintf("SetAllowEdits: %s", err))
				return
			}
			ctx.ServerError("SetAllowEdits", err)
			return
		}
	}

	// Refetch from database
	pr, err = issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, pr.Index)
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	// TODO this should be 200, not 201
	ctx.JSON(http.StatusCreated, convert.ToAPIPullRequest(ctx, pr, ctx.Doer))
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

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if pr.HasMerged {
		ctx.Status(http.StatusNoContent)
	}
	ctx.NotFound()
}

// MergePullRequest merges a PR given an index
func MergePullRequest(ctx *context.APIContext) {
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
	//   "409":
	//     "$ref": "#/responses/error"

	form := web.GetForm(ctx).(*forms.MergePullRequestForm)

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadHeadRepo", err)
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}
	pr.Issue.Repo = ctx.Repo.Repository

	if ctx.IsSigned {
		// Update issue-user.
		if err = activities_model.SetIssueReadBy(ctx, pr.Issue.ID, ctx.Doer.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "ReadBy", err)
			return
		}
	}

	manuallyMerged := repo_model.MergeStyle(form.Do) == repo_model.MergeStyleManuallyMerged

	mergeCheckType := pull_service.MergeCheckTypeGeneral
	if form.MergeWhenChecksSucceed {
		mergeCheckType = pull_service.MergeCheckTypeAuto
	}
	if manuallyMerged {
		mergeCheckType = pull_service.MergeCheckTypeManually
	}

	// start with merging by checking
	if err := pull_service.CheckPullMergable(ctx, ctx.Doer, &ctx.Repo.Permission, pr, mergeCheckType, form.ForceMerge); err != nil {
		if errors.Is(err, pull_service.ErrIsClosed) {
			ctx.NotFound()
		} else if errors.Is(err, pull_service.ErrUserNotAllowedToMerge) {
			ctx.Error(http.StatusMethodNotAllowed, "Merge", "User not allowed to merge PR")
		} else if errors.Is(err, pull_service.ErrHasMerged) {
			ctx.Error(http.StatusMethodNotAllowed, "PR already merged", "")
		} else if errors.Is(err, pull_service.ErrIsWorkInProgress) {
			ctx.Error(http.StatusMethodNotAllowed, "PR is a work in progress", "Work in progress PRs cannot be merged")
		} else if errors.Is(err, pull_service.ErrNotMergableState) {
			ctx.Error(http.StatusMethodNotAllowed, "PR not in mergeable state", "Please try again later")
		} else if models.IsErrDisallowedToMerge(err) {
			ctx.Error(http.StatusMethodNotAllowed, "PR is not ready to be merged", err)
		} else if asymkey_service.IsErrWontSign(err) {
			ctx.Error(http.StatusMethodNotAllowed, fmt.Sprintf("Protected branch %s requires signed commits but this merge would not be signed", pr.BaseBranch), err)
		} else {
			ctx.InternalServerError(err)
		}
		return
	}

	// handle manually-merged mark
	if manuallyMerged {
		if err := pull_service.MergedManually(pr, ctx.Doer, ctx.Repo.GitRepo, form.MergeCommitID); err != nil {
			if models.IsErrInvalidMergeStyle(err) {
				ctx.Error(http.StatusMethodNotAllowed, "Invalid merge style", fmt.Errorf("%s is not allowed an allowed merge style for this repository", repo_model.MergeStyle(form.Do)))
				return
			}
			if strings.Contains(err.Error(), "Wrong commit ID") {
				ctx.JSON(http.StatusConflict, err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "Manually-Merged", err)
			return
		}
		ctx.Status(http.StatusOK)
		return
	}

	if len(form.Do) == 0 {
		form.Do = string(repo_model.MergeStyleMerge)
	}

	message := strings.TrimSpace(form.MergeTitleField)
	if len(message) == 0 {
		message, _, err = pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pr, repo_model.MergeStyle(form.Do))
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetDefaultMergeMessage", err)
			return
		}
	}

	form.MergeMessageField = strings.TrimSpace(form.MergeMessageField)
	if len(form.MergeMessageField) > 0 {
		message += "\n\n" + form.MergeMessageField
	}

	if form.MergeWhenChecksSucceed {
		scheduled, err := automerge.ScheduleAutoMerge(ctx, ctx.Doer, pr, repo_model.MergeStyle(form.Do), message)
		if err != nil {
			if pull_model.IsErrAlreadyScheduledToAutoMerge(err) {
				ctx.Error(http.StatusConflict, "ScheduleAutoMerge", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "ScheduleAutoMerge", err)
			return
		} else if scheduled {
			// nothing more to do ...
			ctx.Status(http.StatusCreated)
			return
		}
	}

	if err := pull_service.Merge(ctx, pr, ctx.Doer, ctx.Repo.GitRepo, repo_model.MergeStyle(form.Do), form.HeadCommitID, message, false); err != nil {
		if models.IsErrInvalidMergeStyle(err) {
			ctx.Error(http.StatusMethodNotAllowed, "Invalid merge style", fmt.Errorf("%s is not allowed an allowed merge style for this repository", repo_model.MergeStyle(form.Do)))
		} else if models.IsErrMergeConflicts(err) {
			conflictError := err.(models.ErrMergeConflicts)
			ctx.JSON(http.StatusConflict, conflictError)
		} else if models.IsErrRebaseConflicts(err) {
			conflictError := err.(models.ErrRebaseConflicts)
			ctx.JSON(http.StatusConflict, conflictError)
		} else if models.IsErrMergeUnrelatedHistories(err) {
			conflictError := err.(models.ErrMergeUnrelatedHistories)
			ctx.JSON(http.StatusConflict, conflictError)
		} else if git.IsErrPushOutOfDate(err) {
			ctx.Error(http.StatusConflict, "Merge", "merge push out of date")
		} else if models.IsErrSHADoesNotMatch(err) {
			ctx.Error(http.StatusConflict, "Merge", "head out of date")
		} else if git.IsErrPushRejected(err) {
			errPushRej := err.(*git.ErrPushRejected)
			if len(errPushRej.Message) == 0 {
				ctx.Error(http.StatusConflict, "Merge", "PushRejected without remote error message")
			} else {
				ctx.Error(http.StatusConflict, "Merge", "PushRejected with remote message: "+errPushRej.Message)
			}
		} else {
			ctx.Error(http.StatusInternalServerError, "Merge", err)
		}
		return
	}
	log.Trace("Pull request merged: %d", pr.ID)

	if form.DeleteBranchAfterMerge {
		// Don't cleanup when there are other PR's that use this branch as head branch.
		exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(ctx, pr.HeadRepoID, pr.HeadBranch)
		if err != nil {
			ctx.ServerError("HasUnmergedPullRequestsByHeadInfo", err)
			return
		}
		if exist {
			ctx.Status(http.StatusOK)
			return
		}

		var headRepo *git.Repository
		if ctx.Repo != nil && ctx.Repo.Repository != nil && ctx.Repo.Repository.ID == pr.HeadRepoID && ctx.Repo.GitRepo != nil {
			headRepo = ctx.Repo.GitRepo
		} else {
			headRepo, err = git.OpenRepository(ctx, pr.HeadRepo.RepoPath())
			if err != nil {
				ctx.ServerError(fmt.Sprintf("OpenRepository[%s]", pr.HeadRepo.RepoPath()), err)
				return
			}
			defer headRepo.Close()
		}
		if err := repo_service.DeleteBranch(ctx, ctx.Doer, pr.HeadRepo, headRepo, pr.HeadBranch); err != nil {
			switch {
			case git.IsErrBranchNotExist(err):
				ctx.NotFound(err)
			case errors.Is(err, repo_service.ErrBranchIsDefault):
				ctx.Error(http.StatusForbidden, "DefaultBranch", fmt.Errorf("can not delete default branch"))
			case errors.Is(err, git_model.ErrBranchIsProtected):
				ctx.Error(http.StatusForbidden, "IsProtectedBranch", fmt.Errorf("branch protected"))
			default:
				ctx.Error(http.StatusInternalServerError, "DeleteBranch", err)
			}
			return
		}
		if err := issues_model.AddDeletePRBranchComment(ctx, ctx.Doer, pr.BaseRepo, pr.Issue.ID, pr.HeadBranch); err != nil {
			// Do not fail here as branch has already been deleted
			log.Error("DeleteBranch: %v", err)
		}
	}

	ctx.Status(http.StatusOK)
}

func parseCompareInfo(ctx *context.APIContext, form api.CreatePullRequestOption) (*user_model.User, *repo_model.Repository, *git.Repository, *git.CompareInfo, string, string) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches information
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	// TODO: Validate form first?

	baseBranch := form.Base

	var (
		headUser   *user_model.User
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
		headUser, err = user_model.GetUserByName(ctx, headInfos[0])
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.NotFound("GetUserByName")
			} else {
				ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
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
	headRepo := repo_model.GetForkedRepo(headUser.ID, baseRepo.ID)
	if headRepo == nil && !isSameRepo {
		log.Trace("parseCompareInfo[%d]: does not have fork or in same repository", baseRepo.ID)
		ctx.NotFound("GetForkedRepo")
		return nil, nil, nil, nil, "", ""
	}

	var headGitRepo *git.Repository
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
	} else {
		headGitRepo, err = git.OpenRepository(ctx, repo_model.RepoPath(headUser.Name, headRepo.Name))
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
			return nil, nil, nil, nil, "", ""
		}
	}

	// user should have permission to read baseRepo's codes and pulls, NOT headRepo's
	permBase, err := access_model.GetUserRepoPermission(ctx, baseRepo, ctx.Doer)
	if err != nil {
		headGitRepo.Close()
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permBase.CanReadIssuesOrPulls(true) || !permBase.CanRead(unit.TypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User %-v cannot create/read pull requests or cannot read code in Repo %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.Doer,
				baseRepo,
				permBase)
		}
		headGitRepo.Close()
		ctx.NotFound("Can't read pulls or can't read UnitTypeCode")
		return nil, nil, nil, nil, "", ""
	}

	// user should have permission to read headrepo's codes
	permHead, err := access_model.GetUserRepoPermission(ctx, headRepo, ctx.Doer)
	if err != nil {
		headGitRepo.Close()
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permHead.CanRead(unit.TypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
				ctx.Doer,
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

	compareInfo, err := headGitRepo.GetCompareInfo(repo_model.RepoPath(baseRepo.Owner.Name, baseRepo.Name), baseBranch, headBranch, false, false)
	if err != nil {
		headGitRepo.Close()
		ctx.Error(http.StatusInternalServerError, "GetCompareInfo", err)
		return nil, nil, nil, nil, "", ""
	}

	return headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch
}

// UpdatePullRequest merge PR's baseBranch into headBranch
func UpdatePullRequest(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/update repository repoUpdatePullRequest
	// ---
	// summary: Merge PR's baseBranch into headBranch
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
	// - name: style
	//   in: query
	//   description: how to update pull request
	//   type: string
	//   enum: [merge, rebase]
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if pr.HasMerged {
		ctx.Error(http.StatusUnprocessableEntity, "UpdatePullRequest", err)
		return
	}

	if err = pr.LoadIssue(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	if pr.Issue.IsClosed {
		ctx.Error(http.StatusUnprocessableEntity, "UpdatePullRequest", err)
		return
	}

	if err = pr.LoadBaseRepo(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadBaseRepo", err)
		return
	}
	if err = pr.LoadHeadRepo(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadHeadRepo", err)
		return
	}

	rebase := ctx.FormString("style") == "rebase"

	allowedUpdateByMerge, allowedUpdateByRebase, err := pull_service.IsUserAllowedToUpdate(ctx, pr, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsUserAllowedToMerge", err)
		return
	}

	if (!allowedUpdateByMerge && !rebase) || (rebase && !allowedUpdateByRebase) {
		ctx.Status(http.StatusForbidden)
		return
	}

	// default merge commit message
	message := fmt.Sprintf("Merge branch '%s' into %s", pr.BaseBranch, pr.HeadBranch)

	if err = pull_service.Update(ctx, pr, ctx.Doer, message, rebase); err != nil {
		if models.IsErrMergeConflicts(err) {
			ctx.Error(http.StatusConflict, "Update", "merge failed because of conflict")
			return
		} else if models.IsErrRebaseConflicts(err) {
			ctx.Error(http.StatusConflict, "Update", "rebase failed because of conflict")
			return
		}
		ctx.Error(http.StatusInternalServerError, "pull_service.Update", err)
		return
	}

	ctx.Status(http.StatusOK)
}

// MergePullRequest cancel an auto merge scheduled for a given PullRequest by index
func CancelScheduledAutoMerge(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/pulls/{index}/merge repository repoCancelScheduledAutoMerge
	// ---
	// summary: Cancel the scheduled auto merge for the given pull request
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pullIndex := ctx.ParamsInt64(":index")
	pull, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, pullIndex)
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
			return
		}
		ctx.InternalServerError(err)
		return
	}

	exist, autoMerge, err := pull_model.GetScheduledMergeByPullID(ctx, pull.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if !exist {
		ctx.NotFound()
		return
	}

	if ctx.Doer.ID != autoMerge.DoerID {
		allowed, err := access_model.IsUserRepoAdmin(ctx, ctx.Repo.Repository, ctx.Doer)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
		if !allowed {
			ctx.Error(http.StatusForbidden, "No permission to cancel", "user has no permission to cancel the scheduled auto merge")
			return
		}
	}

	if err := automerge.RemoveScheduledAutoMerge(ctx, ctx.Doer, pull); err != nil {
		ctx.InternalServerError(err)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// GetPullRequestCommits gets all commits associated with a given PR
func GetPullRequestCommits(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/commits repository repoGetPullRequestCommits
	// ---
	// summary: Get commits for a pull request
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
	//     "$ref": "#/responses/CommitList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		ctx.InternalServerError(err)
		return
	}

	var prInfo *git.CompareInfo
	baseGitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, pr.BaseRepo.RepoPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer closer.Close()

	if pr.HasMerged {
		prInfo, err = baseGitRepo.GetCompareInfo(pr.BaseRepo.RepoPath(), pr.MergeBase, pr.GetGitRefName(), false, false)
	} else {
		prInfo, err = baseGitRepo.GetCompareInfo(pr.BaseRepo.RepoPath(), pr.BaseBranch, pr.GetGitRefName(), false, false)
	}
	if err != nil {
		ctx.ServerError("GetCompareInfo", err)
		return
	}
	commits := prInfo.Commits

	listOptions := utils.GetListOptions(ctx)

	totalNumberOfCommits := len(commits)
	totalNumberOfPages := int(math.Ceil(float64(totalNumberOfCommits) / float64(listOptions.PageSize)))

	userCache := make(map[string]*user_model.User)

	start, end := listOptions.GetStartEnd()

	if end > totalNumberOfCommits {
		end = totalNumberOfCommits
	}

	apiCommits := make([]*api.Commit, 0, end-start)
	for i := start; i < end; i++ {
		apiCommit, err := convert.ToCommit(ctx, ctx.Repo.Repository, baseGitRepo, commits[i], userCache, convert.ToCommitOptions{Stat: true})
		if err != nil {
			ctx.ServerError("toCommit", err)
			return
		}
		apiCommits = append(apiCommits, apiCommit)
	}

	ctx.SetLinkHeader(totalNumberOfCommits, listOptions.PageSize)
	ctx.SetTotalCountHeader(int64(totalNumberOfCommits))

	ctx.RespHeader().Set("X-Page", strconv.Itoa(listOptions.Page))
	ctx.RespHeader().Set("X-PerPage", strconv.Itoa(listOptions.PageSize))
	ctx.RespHeader().Set("X-PageCount", strconv.Itoa(totalNumberOfPages))
	ctx.RespHeader().Set("X-HasMore", strconv.FormatBool(listOptions.Page < totalNumberOfPages))
	ctx.AppendAccessControlExposeHeaders("X-Page", "X-PerPage", "X-PageCount", "X-HasMore")

	ctx.JSON(http.StatusOK, &apiCommits)
}

// GetPullRequestFiles gets all changed files associated with a given PR
func GetPullRequestFiles(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/files repository repoGetPullRequestFiles
	// ---
	// summary: Get changed files for a pull request
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
	// - name: skip-to
	//   in: query
	//   description: skip to given file
	//   type: string
	// - name: whitespace
	//   in: query
	//   description: whitespace behavior
	//   type: string
	//   enum: [ignore-all, ignore-change, ignore-eol, show-all]
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
	//     "$ref": "#/responses/ChangedFileList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		ctx.InternalServerError(err)
		return
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		ctx.InternalServerError(err)
		return
	}

	baseGitRepo := ctx.Repo.GitRepo

	var prInfo *git.CompareInfo
	if pr.HasMerged {
		prInfo, err = baseGitRepo.GetCompareInfo(pr.BaseRepo.RepoPath(), pr.MergeBase, pr.GetGitRefName(), true, false)
	} else {
		prInfo, err = baseGitRepo.GetCompareInfo(pr.BaseRepo.RepoPath(), pr.BaseBranch, pr.GetGitRefName(), true, false)
	}
	if err != nil {
		ctx.ServerError("GetCompareInfo", err)
		return
	}

	headCommitID, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
	if err != nil {
		ctx.ServerError("GetRefCommitID", err)
		return
	}

	startCommitID := prInfo.MergeBase
	endCommitID := headCommitID

	maxLines := setting.Git.MaxGitDiffLines

	// FIXME: If there are too many files in the repo, may cause some unpredictable issues.
	diff, err := gitdiff.GetDiff(baseGitRepo,
		&gitdiff.DiffOptions{
			BeforeCommitID:     startCommitID,
			AfterCommitID:      endCommitID,
			SkipTo:             ctx.FormString("skip-to"),
			MaxLines:           maxLines,
			MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
			MaxFiles:           -1, // GetDiff() will return all files
			WhitespaceBehavior: gitdiff.GetWhitespaceFlag(ctx.FormString("whitespace")),
		})
	if err != nil {
		ctx.ServerError("GetDiff", err)
		return
	}

	listOptions := utils.GetListOptions(ctx)

	totalNumberOfFiles := diff.NumFiles
	totalNumberOfPages := int(math.Ceil(float64(totalNumberOfFiles) / float64(listOptions.PageSize)))

	start, end := listOptions.GetStartEnd()

	if end > totalNumberOfFiles {
		end = totalNumberOfFiles
	}

	lenFiles := end - start
	if lenFiles < 0 {
		lenFiles = 0
	}

	apiFiles := make([]*api.ChangedFile, 0, lenFiles)
	for i := start; i < end; i++ {
		apiFiles = append(apiFiles, convert.ToChangedFile(diff.Files[i], pr.HeadRepo, endCommitID))
	}

	ctx.SetLinkHeader(totalNumberOfFiles, listOptions.PageSize)
	ctx.SetTotalCountHeader(int64(totalNumberOfFiles))

	ctx.RespHeader().Set("X-Page", strconv.Itoa(listOptions.Page))
	ctx.RespHeader().Set("X-PerPage", strconv.Itoa(listOptions.PageSize))
	ctx.RespHeader().Set("X-PageCount", strconv.Itoa(totalNumberOfPages))
	ctx.RespHeader().Set("X-HasMore", strconv.FormatBool(listOptions.Page < totalNumberOfPages))
	ctx.AppendAccessControlExposeHeaders("X-Page", "X-PerPage", "X-PageCount", "X-HasMore")

	ctx.JSON(http.StatusOK, &apiFiles)
}
