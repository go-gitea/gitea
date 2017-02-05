// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"

	api "code.gitea.io/sdk/gitea"
)

// ListPullRequests returns a list of all PRs
func ListPullRequests(ctx *context.APIContext, form api.ListPullRequestsOptions) {
	prs, maxResults, err := models.PullRequests(ctx.Repo.Repository.ID, &models.PullRequestsOptions{
		Page:        ctx.QueryInt("page"),
		State:       ctx.QueryTrim("state"),
		SortType:    ctx.QueryTrim("sort"),
		Labels:      ctx.QueryStrings("labels"),
		MilestoneID: ctx.QueryInt64("milestone"),
	})

	/*prs, maxResults, err := models.PullRequests(ctx.Repo.Repository.ID, &models.PullRequestsOptions{
		Page:  form.Page,
		State: form.State,
	})*/
	if err != nil {
		ctx.Error(500, "PullRequests", err)
		return
	}

	apiPrs := make([]*api.PullRequest, len(prs))
	for i := range prs {
		prs[i].LoadIssue()
		prs[i].LoadAttributes()
		prs[i].GetBaseRepo()
		prs[i].GetHeadRepo()
		apiPrs[i] = prs[i].APIFormat()
	}

	ctx.SetLinkHeader(int(maxResults), models.ItemsPerPage)
	ctx.JSON(200, &apiPrs)
}

// GetPullRequest returns a single PR based on index
func GetPullRequest(ctx *context.APIContext) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	pr.GetBaseRepo()
	pr.GetHeadRepo()
	ctx.JSON(200, pr.APIFormat())
}

// CreatePullRequest does what it says
func CreatePullRequest(ctx *context.APIContext, form api.CreatePullRequestOption) {
	var (
		repo        = ctx.Repo.Repository
		labelIDs    []int64
		assigneeID  int64
		milestoneID int64
	)

	// Get repo/branch information
	headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch := parseCompareInfo(ctx, form)
	if ctx.Written() {
		return
	}

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
				ctx.Status(404)
			} else {
				ctx.Error(500, "GetMilestoneByRepoID", err)
			}
			return
		}

		milestoneID = milestone.ID
	}

	if len(form.Assignee) > 0 {
		assigneeUser, err := models.GetUserByName(form.Assignee)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(422, "", fmt.Sprintf("assignee does not exist: [name: %s]", form.Assignee))
			} else {
				ctx.Error(500, "GetUserByName", err)
			}
			return
		}

		assignee, err := repo.GetAssigneeByID(assigneeUser.ID)
		if err != nil {
			ctx.Error(500, "GetAssigneeByID", err)
			return
		}

		assigneeID = assignee.ID
	}

	patch, err := headGitRepo.GetPatch(prInfo.MergeBase, headBranch)
	if err != nil {
		ctx.Error(500, "GetPatch", err)
		return
	}

	prIssue := &models.Issue{
		RepoID:      repo.ID,
		Index:       repo.NextIssueIndex(),
		Title:       form.Title,
		PosterID:    ctx.User.ID,
		Poster:      ctx.User,
		MilestoneID: milestoneID,
		AssigneeID:  assigneeID,
		IsPull:      true,
		Content:     form.Body,
	}
	pr := &models.PullRequest{
		HeadRepoID:   headRepo.ID,
		BaseRepoID:   repo.ID,
		HeadUserName: headUser.Name,
		HeadBranch:   headBranch,
		BaseBranch:   baseBranch,
		HeadRepo:     headRepo,
		BaseRepo:     repo,
		MergeBase:    prInfo.MergeBase,
		Type:         models.PullRequestGitea,
	}

	if err := models.NewPullRequest(repo, prIssue, labelIDs, []string{}, pr, patch); err != nil {
		ctx.Error(500, "NewPullRequest", err)
		return
	} else if err := pr.PushToBaseRepo(); err != nil {
		ctx.Error(500, "PushToBaseRepo", err)
		return
	}

	log.Trace("Pull request created: %d/%d", repo.ID, prIssue.ID)
	ctx.JSON(201, pr.APIFormat())
}

// EditPullRequest does what it says
func EditPullRequest(ctx *context.APIContext, form api.EditPullRequestOption) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	pr.LoadIssue()
	issue := pr.Issue

	if !issue.IsPoster(ctx.User.ID) && !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	if len(form.Title) > 0 {
		issue.Title = form.Title
	}
	if len(form.Body) > 0 {
		issue.Content = form.Body
	}

	if ctx.Repo.IsWriter() && len(form.Assignee) > 0 &&
		(issue.Assignee == nil || issue.Assignee.LowerName != strings.ToLower(form.Assignee)) {
		if len(form.Assignee) == 0 {
			issue.AssigneeID = 0
		} else {
			assignee, err := models.GetUserByName(form.Assignee)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.Error(422, "", fmt.Sprintf("assignee does not exist: [name: %s]", form.Assignee))
				} else {
					ctx.Error(500, "GetUserByName", err)
				}
				return
			}
			issue.AssigneeID = assignee.ID
		}

		if err = models.UpdateIssueUserByAssignee(issue); err != nil {
			ctx.Error(500, "UpdateIssueUserByAssignee", err)
			return
		}
	}
	if ctx.Repo.IsWriter() && form.Milestone != 0 &&
		issue.MilestoneID != form.Milestone {
		oldMilestoneID := issue.MilestoneID
		issue.MilestoneID = form.Milestone
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

	// Refetch from database
	pr, err = models.GetPullRequestByIndex(ctx.Repo.Repository.ID, pr.Index)
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	ctx.JSON(201, pr.APIFormat())
}

// IsPullRequestMerged checks if a PR exists given an index
//  - Returns 204 if it exists
//    Otherwise 404
func IsPullRequestMerged(ctx *context.APIContext) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	if pr.HasMerged {
		ctx.Status(204)
	}
	ctx.Status(404)
}

// MergePullRequest merges a PR given an index
func MergePullRequest(ctx *context.APIContext) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.Handle(404, "GetPullRequestByIndex", err)
		} else {
			ctx.Error(500, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.GetHeadRepo(); err != nil {
		ctx.Handle(500, "GetHeadRepo", err)
		return
	}

	pr.LoadIssue()
	pr.Issue.Repo = ctx.Repo.Repository

	if ctx.IsSigned {
		// Update issue-user.
		if err = pr.Issue.ReadBy(ctx.User.ID); err != nil {
			ctx.Error(500, "ReadBy", err)
			return
		}
	}

	if pr.Issue.IsClosed {
		ctx.Status(404)
		return
	}

	if !pr.CanAutoMerge() || pr.HasMerged {
		ctx.Status(405)
		return
	}

	if err := pr.Merge(ctx.User, ctx.Repo.GitRepo); err != nil {
		ctx.Error(500, "Merge", err)
		return
	}

	log.Trace("Pull request merged: %d", pr.ID)
	ctx.Status(200)
}

func parseCompareInfo(ctx *context.APIContext, form api.CreatePullRequestOption) (*models.User, *models.Repository, *git.Repository, *git.PullRequestInfo, string, string) {
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
				ctx.Handle(404, "GetUserByName", nil)
			} else {
				ctx.Handle(500, "GetUserByName", err)
			}
			return nil, nil, nil, nil, "", ""
		}
		headBranch = headInfos[1]

	} else {
		ctx.Status(404)
		return nil, nil, nil, nil, "", ""
	}

	ctx.Repo.PullRequest.SameRepo = isSameRepo
	log.Info("Base branch: %s", baseBranch)
	log.Info("Repo path: %s", ctx.Repo.GitRepo.Path)
	// Check if base branch is valid.
	if !ctx.Repo.GitRepo.IsBranchExist(baseBranch) {
		ctx.Status(404)
		return nil, nil, nil, nil, "", ""
	}

	// Check if current user has fork of repository or in the same repository.
	headRepo, has := models.HasForkedRepo(headUser.ID, baseRepo.ID)
	if !has && !isSameRepo {
		log.Trace("parseCompareInfo[%d]: does not have fork or in same repository", baseRepo.ID)
		ctx.Status(404)
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

	if !ctx.User.IsWriterOfRepo(headRepo) && !ctx.User.IsAdmin {
		log.Trace("ParseCompareInfo[%d]: does not have write access or site admin", baseRepo.ID)
		ctx.Status(404)
		return nil, nil, nil, nil, "", ""
	}

	// Check if head branch is valid.
	if !headGitRepo.IsBranchExist(headBranch) {
		ctx.Status(404)
		return nil, nil, nil, nil, "", ""
	}

	prInfo, err := headGitRepo.GetPullRequestInfo(models.RepoPath(baseRepo.Owner.Name, baseRepo.Name), baseBranch, headBranch)
	if err != nil {
		ctx.Error(500, "GetPullRequestInfo", err)
		return nil, nil, nil, nil, "", ""
	}

	return headUser, headRepo, headGitRepo, prInfo, baseBranch, headBranch
}
