// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	stdCtx "context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	issue_template "code.gitea.io/gitea/modules/issue/template"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplAttachment base.TplName = "repo/issue/view_content/attachments"

	tplIssues      base.TplName = "repo/issue/list"
	tplIssueNew    base.TplName = "repo/issue/new"
	tplIssueChoose base.TplName = "repo/issue/choose"
	tplIssueView   base.TplName = "repo/issue/view"

	tplReactions base.TplName = "repo/issue/view_content/reactions"

	issueTemplateKey      = "IssueTemplate"
	issueTemplateTitleKey = "IssueTemplateTitle"
)

// IssueTemplateCandidates issue templates
var IssueTemplateCandidates = []string{
	"ISSUE_TEMPLATE.md",
	"ISSUE_TEMPLATE.yaml",
	"ISSUE_TEMPLATE.yml",
	"issue_template.md",
	"issue_template.yaml",
	"issue_template.yml",
	".gitea/ISSUE_TEMPLATE.md",
	".gitea/ISSUE_TEMPLATE.yaml",
	".gitea/ISSUE_TEMPLATE.yml",
	".gitea/issue_template.md",
	".gitea/issue_template.yaml",
	".gitea/issue_template.yml",
	".github/ISSUE_TEMPLATE.md",
	".github/ISSUE_TEMPLATE.yaml",
	".github/ISSUE_TEMPLATE.yml",
	".github/issue_template.md",
	".github/issue_template.yaml",
	".github/issue_template.yml",
}

// MustAllowUserComment checks to make sure if an issue is locked.
// If locked and user has permissions to write to the repository,
// then the comment is allowed, else it is blocked
func MustAllowUserComment(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.Doer.IsAdmin {
		ctx.Flash.Error(ctx.Tr("repo.issues.comment_on_locked"))
		ctx.Redirect(issue.Link())
		return
	}
}

// MustEnableIssues check if repository enable internal issues
func MustEnableIssues(ctx *context.Context) {
	if !ctx.Repo.CanRead(unit.TypeIssues) &&
		!ctx.Repo.CanRead(unit.TypeExternalTracker) {
		ctx.NotFound("MustEnableIssues", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
	if err == nil {
		ctx.Redirect(unit.ExternalTrackerConfig().ExternalTrackerURL)
		return
	}
}

// MustAllowPulls check if repository enable pull requests and user have right to do that
func MustAllowPulls(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(unit.TypePullRequests) {
		ctx.NotFound("MustAllowPulls", nil)
		return
	}

	// User can send pull request if owns a forked repository.
	if ctx.IsSigned && repo_model.HasForkedRepo(ctx.Doer.ID, ctx.Repo.Repository.ID) {
		ctx.Repo.PullRequest.Allowed = true
		ctx.Repo.PullRequest.HeadInfoSubURL = url.PathEscape(ctx.Doer.Name) + ":" + util.PathEscapeSegments(ctx.Repo.BranchName)
	}
}

func issues(ctx *context.Context, milestoneID, projectID int64, isPullOption util.OptionalBool) {
	var err error
	viewType := ctx.FormString("type")
	sortType := ctx.FormString("sort")
	types := []string{"all", "your_repositories", "assigned", "created_by", "mentioned", "review_requested", "reviewed_by"}
	if !util.SliceContainsString(types, viewType, true) {
		viewType = "all"
	}

	var (
		assigneeID        = ctx.FormInt64("assignee")
		posterID          = ctx.FormInt64("poster")
		mentionedID       int64
		reviewRequestedID int64
		reviewedID        int64
		forceEmpty        bool
	)

	if ctx.IsSigned {
		switch viewType {
		case "created_by":
			posterID = ctx.Doer.ID
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
	var labelIDs []int64
	// 1,-2 means including label 1 and excluding label 2
	// 0 means issues with no label
	// blank means labels will not be filtered for issues
	selectLabels := ctx.FormString("labels")
	if len(selectLabels) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(selectLabels, ","))
		if err != nil {
			ctx.ServerError("StringsToInt64s", err)
			return
		}
	}

	keyword := strings.Trim(ctx.FormString("q"), " ")
	if bytes.Contains([]byte(keyword), []byte{0x00}) {
		keyword = ""
	}

	var issueIDs []int64
	if len(keyword) > 0 {
		issueIDs, err = issue_indexer.SearchIssuesByKeyword(ctx, []int64{repo.ID}, keyword)
		if err != nil {
			if issue_indexer.IsAvailable() {
				ctx.ServerError("issueIndexer.Search", err)
				return
			}
			ctx.Data["IssueIndexerUnavailable"] = true
		}
		if len(issueIDs) == 0 {
			forceEmpty = true
		}
	}

	var mileIDs []int64
	if milestoneID > 0 || milestoneID == db.NoConditionID { // -1 to get those issues which have no any milestone assigned
		mileIDs = []int64{milestoneID}
	}

	var issueStats *issues_model.IssueStats
	if forceEmpty {
		issueStats = &issues_model.IssueStats{}
	} else {
		issueStats, err = issues_model.GetIssueStats(&issues_model.IssuesOptions{
			RepoIDs:           []int64{repo.ID},
			LabelIDs:          labelIDs,
			MilestoneIDs:      mileIDs,
			ProjectID:         projectID,
			AssigneeID:        assigneeID,
			MentionedID:       mentionedID,
			PosterID:          posterID,
			ReviewRequestedID: reviewRequestedID,
			ReviewedID:        reviewedID,
			IsPull:            isPullOption,
			IssueIDs:          issueIDs,
		})
		if err != nil {
			ctx.ServerError("GetIssueStats", err)
			return
		}
	}

	isShowClosed := ctx.FormString("state") == "closed"
	// if open issues are zero and close don't, use closed as default
	if len(ctx.FormString("state")) == 0 && issueStats.OpenCount == 0 && issueStats.ClosedCount != 0 {
		isShowClosed = true
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	if !isShowClosed {
		total = int(issueStats.OpenCount)
	} else {
		total = int(issueStats.ClosedCount)
	}
	pager := context.NewPagination(total, setting.UI.IssuePagingNum, page, 5)

	var issues []*issues_model.Issue
	if forceEmpty {
		issues = []*issues_model.Issue{}
	} else {
		issues, err = issues_model.Issues(ctx, &issues_model.IssuesOptions{
			ListOptions: db.ListOptions{
				Page:     pager.Paginater.Current(),
				PageSize: setting.UI.IssuePagingNum,
			},
			RepoIDs:           []int64{repo.ID},
			AssigneeID:        assigneeID,
			PosterID:          posterID,
			MentionedID:       mentionedID,
			ReviewRequestedID: reviewRequestedID,
			ReviewedID:        reviewedID,
			MilestoneIDs:      mileIDs,
			ProjectID:         projectID,
			IsClosed:          util.OptionalBoolOf(isShowClosed),
			IsPull:            isPullOption,
			LabelIDs:          labelIDs,
			SortType:          sortType,
			IssueIDs:          issueIDs,
		})
		if err != nil {
			ctx.ServerError("Issues", err)
			return
		}
	}

	issueList := issues_model.IssueList(issues)
	approvalCounts, err := issueList.GetApprovalCounts(ctx)
	if err != nil {
		ctx.ServerError("ApprovalCounts", err)
		return
	}

	// Get posters.
	for i := range issues {
		// Check read status
		if !ctx.IsSigned {
			issues[i].IsRead = true
		} else if err = issues[i].GetIsRead(ctx.Doer.ID); err != nil {
			ctx.ServerError("GetIsRead", err)
			return
		}
	}

	commitStatuses, lastStatus, err := pull_service.GetIssuesAllCommitStatus(ctx, issues)
	if err != nil {
		ctx.ServerError("GetIssuesAllCommitStatus", err)
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
	ctx.Data["Assignees"] = makeSelfOnTop(ctx, assigneeUsers)

	handleTeamMentions(ctx)
	if ctx.Written() {
		return
	}

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}

	if repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return
		}

		ctx.Data["OrgLabels"] = orgLabels
		labels = append(labels, orgLabels...)
	}

	// Get the exclusive scope for every label ID
	labelExclusiveScopes := make([]string, 0, len(labelIDs))
	for _, labelID := range labelIDs {
		foundExclusiveScope := false
		for _, label := range labels {
			if label.ID == labelID || label.ID == -labelID {
				labelExclusiveScopes = append(labelExclusiveScopes, label.ExclusiveScope())
				foundExclusiveScope = true
				break
			}
		}
		if !foundExclusiveScope {
			labelExclusiveScopes = append(labelExclusiveScopes, "")
		}
	}

	for _, l := range labels {
		l.LoadSelectedLabelsAfterClick(labelIDs, labelExclusiveScopes)
	}
	ctx.Data["Labels"] = labels
	ctx.Data["NumLabels"] = len(labels)

	if ctx.FormInt64("assignee") == 0 {
		assigneeID = 0 // Reset ID to prevent unexpected selection of assignee.
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

	retrieveProjects(ctx, repo)
	if ctx.Written() {
		return
	}

	pinned, err := issues_model.GetPinnedIssues(ctx, repo.ID, isPullOption.IsTrue())
	if err != nil {
		ctx.ServerError("GetPinnedIssues", err)
		return
	}

	ctx.Data["PinnedIssues"] = pinned
	ctx.Data["IsRepoAdmin"] = ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	ctx.Data["IssueStats"] = issueStats
	ctx.Data["SelLabelIDs"] = labelIDs
	ctx.Data["SelectLabels"] = selectLabels
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["ProjectID"] = projectID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["PosterID"] = posterID
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["Keyword"] = keyword
	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	pager.AddParam(ctx, "q", "Keyword")
	pager.AddParam(ctx, "type", "ViewType")
	pager.AddParam(ctx, "sort", "SortType")
	pager.AddParam(ctx, "state", "State")
	pager.AddParam(ctx, "labels", "SelectLabels")
	pager.AddParam(ctx, "milestone", "MilestoneID")
	pager.AddParam(ctx, "project", "ProjectID")
	pager.AddParam(ctx, "assignee", "AssigneeID")
	pager.AddParam(ctx, "poster", "PosterID")
	ctx.Data["Page"] = pager
}

// Issues render issues page
func Issues(ctx *context.Context) {
	isPullList := ctx.Params(":type") == "pulls"
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

	issues(ctx, ctx.FormInt64("milestone"), ctx.FormInt64("project"), util.OptionalBoolOf(isPullList))
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

func renderMilestones(ctx *context.Context) {
	// Get milestones
	milestones, _, err := issues_model.GetMilestones(issues_model.GetMilestonesOption{
		RepoID: ctx.Repo.Repository.ID,
		State:  api.StateAll,
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

// RetrieveRepoMilestonesAndAssignees find all the milestones and assignees of a repository
func RetrieveRepoMilestonesAndAssignees(ctx *context.Context, repo *repo_model.Repository) {
	var err error
	ctx.Data["OpenMilestones"], _, err = issues_model.GetMilestones(issues_model.GetMilestonesOption{
		RepoID: repo.ID,
		State:  api.StateOpen,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
	ctx.Data["ClosedMilestones"], _, err = issues_model.GetMilestones(issues_model.GetMilestonesOption{
		RepoID: repo.ID,
		State:  api.StateClosed,
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}

	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, repo)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = makeSelfOnTop(ctx, assigneeUsers)

	handleTeamMentions(ctx)
}

func retrieveProjects(ctx *context.Context, repo *repo_model.Repository) {
	// Distinguish whether the owner of the repository
	// is an individual or an organization
	repoOwnerType := project_model.TypeIndividual
	if repo.Owner.IsOrganization() {
		repoOwnerType = project_model.TypeOrganization
	}
	var err error
	projects, _, err := project_model.FindProjects(ctx, project_model.SearchOptions{
		RepoID:   repo.ID,
		Page:     -1,
		IsClosed: util.OptionalBoolFalse,
		Type:     project_model.TypeRepository,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}
	projects2, _, err := project_model.FindProjects(ctx, project_model.SearchOptions{
		OwnerID:  repo.OwnerID,
		Page:     -1,
		IsClosed: util.OptionalBoolFalse,
		Type:     repoOwnerType,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	ctx.Data["OpenProjects"] = append(projects, projects2...)

	projects, _, err = project_model.FindProjects(ctx, project_model.SearchOptions{
		RepoID:   repo.ID,
		Page:     -1,
		IsClosed: util.OptionalBoolTrue,
		Type:     project_model.TypeRepository,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}
	projects2, _, err = project_model.FindProjects(ctx, project_model.SearchOptions{
		OwnerID:  repo.OwnerID,
		Page:     -1,
		IsClosed: util.OptionalBoolTrue,
		Type:     repoOwnerType,
	})
	if err != nil {
		ctx.ServerError("GetProjects", err)
		return
	}

	ctx.Data["ClosedProjects"] = append(projects, projects2...)
}

// repoReviewerSelection items to bee shown
type repoReviewerSelection struct {
	IsTeam    bool
	Team      *organization.Team
	User      *user_model.User
	Review    *issues_model.Review
	CanChange bool
	Checked   bool
	ItemID    int64
}

// RetrieveRepoReviewers find all reviewers of a repository
func RetrieveRepoReviewers(ctx *context.Context, repo *repo_model.Repository, issue *issues_model.Issue, canChooseReviewer bool) {
	ctx.Data["CanChooseReviewer"] = canChooseReviewer

	originalAuthorReviews, err := issues_model.GetReviewersFromOriginalAuthorsByIssueID(issue.ID)
	if err != nil {
		ctx.ServerError("GetReviewersFromOriginalAuthorsByIssueID", err)
		return
	}
	ctx.Data["OriginalReviews"] = originalAuthorReviews

	reviews, err := issues_model.GetReviewsByIssueID(issue.ID)
	if err != nil {
		ctx.ServerError("GetReviewersByIssueID", err)
		return
	}

	if len(reviews) == 0 && !canChooseReviewer {
		return
	}

	var (
		pullReviews         []*repoReviewerSelection
		reviewersResult     []*repoReviewerSelection
		teamReviewersResult []*repoReviewerSelection
		teamReviewers       []*organization.Team
		reviewers           []*user_model.User
	)

	if canChooseReviewer {
		posterID := issue.PosterID
		if issue.OriginalAuthorID > 0 {
			posterID = 0
		}

		reviewers, err = repo_model.GetReviewers(ctx, repo, ctx.Doer.ID, posterID)
		if err != nil {
			ctx.ServerError("GetReviewers", err)
			return
		}

		teamReviewers, err = repo_service.GetReviewerTeams(ctx, repo)
		if err != nil {
			ctx.ServerError("GetReviewerTeams", err)
			return
		}

		if len(reviewers) > 0 {
			reviewersResult = make([]*repoReviewerSelection, 0, len(reviewers))
		}

		if len(teamReviewers) > 0 {
			teamReviewersResult = make([]*repoReviewerSelection, 0, len(teamReviewers))
		}
	}

	pullReviews = make([]*repoReviewerSelection, 0, len(reviews))

	for _, review := range reviews {
		tmp := &repoReviewerSelection{
			Checked: review.Type == issues_model.ReviewTypeRequest,
			Review:  review,
			ItemID:  review.ReviewerID,
		}
		if review.ReviewerTeamID > 0 {
			tmp.IsTeam = true
			tmp.ItemID = -review.ReviewerTeamID
		}

		if ctx.Repo.IsAdmin() {
			// Admin can dismiss or re-request any review requests
			tmp.CanChange = true
		} else if ctx.Doer != nil && ctx.Doer.ID == review.ReviewerID && review.Type == issues_model.ReviewTypeRequest {
			// A user can refuse review requests
			tmp.CanChange = true
		} else if (canChooseReviewer || (ctx.Doer != nil && ctx.Doer.ID == issue.PosterID)) && review.Type != issues_model.ReviewTypeRequest &&
			ctx.Doer.ID != review.ReviewerID {
			// The poster of the PR, a manager, or official reviewers can re-request review from other reviewers
			tmp.CanChange = true
		}

		pullReviews = append(pullReviews, tmp)

		if canChooseReviewer {
			if tmp.IsTeam {
				teamReviewersResult = append(teamReviewersResult, tmp)
			} else {
				reviewersResult = append(reviewersResult, tmp)
			}
		}
	}

	if len(pullReviews) > 0 {
		// Drop all non-existing users and teams from the reviews
		currentPullReviewers := make([]*repoReviewerSelection, 0, len(pullReviews))
		for _, item := range pullReviews {
			if item.Review.ReviewerID > 0 {
				if err = item.Review.LoadReviewer(ctx); err != nil {
					if user_model.IsErrUserNotExist(err) {
						continue
					}
					ctx.ServerError("LoadReviewer", err)
					return
				}
				item.User = item.Review.Reviewer
			} else if item.Review.ReviewerTeamID > 0 {
				if err = item.Review.LoadReviewerTeam(ctx); err != nil {
					if organization.IsErrTeamNotExist(err) {
						continue
					}
					ctx.ServerError("LoadReviewerTeam", err)
					return
				}
				item.Team = item.Review.ReviewerTeam
			} else {
				continue
			}

			currentPullReviewers = append(currentPullReviewers, item)
		}
		ctx.Data["PullReviewers"] = currentPullReviewers
	}

	if canChooseReviewer && reviewersResult != nil {
		preadded := len(reviewersResult)
		for _, reviewer := range reviewers {
			found := false
		reviewAddLoop:
			for _, tmp := range reviewersResult[:preadded] {
				if tmp.ItemID == reviewer.ID {
					tmp.User = reviewer
					found = true
					break reviewAddLoop
				}
			}

			if found {
				continue
			}

			reviewersResult = append(reviewersResult, &repoReviewerSelection{
				IsTeam:    false,
				CanChange: true,
				User:      reviewer,
				ItemID:    reviewer.ID,
			})
		}

		ctx.Data["Reviewers"] = reviewersResult
	}

	if canChooseReviewer && teamReviewersResult != nil {
		preadded := len(teamReviewersResult)
		for _, team := range teamReviewers {
			found := false
		teamReviewAddLoop:
			for _, tmp := range teamReviewersResult[:preadded] {
				if tmp.ItemID == -team.ID {
					tmp.Team = team
					found = true
					break teamReviewAddLoop
				}
			}

			if found {
				continue
			}

			teamReviewersResult = append(teamReviewersResult, &repoReviewerSelection{
				IsTeam:    true,
				CanChange: true,
				Team:      team,
				ItemID:    -team.ID,
			})
		}

		ctx.Data["TeamReviewers"] = teamReviewersResult
	}
}

// RetrieveRepoMetas find all the meta information of a repository
func RetrieveRepoMetas(ctx *context.Context, repo *repo_model.Repository, isPull bool) []*issues_model.Label {
	if !ctx.Repo.CanWriteIssuesOrPulls(isPull) {
		return nil
	}

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return nil
	}
	ctx.Data["Labels"] = labels
	if repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{})
		if err != nil {
			return nil
		}

		ctx.Data["OrgLabels"] = orgLabels
		labels = append(labels, orgLabels...)
	}

	RetrieveRepoMilestonesAndAssignees(ctx, repo)
	if ctx.Written() {
		return nil
	}

	retrieveProjects(ctx, repo)
	if ctx.Written() {
		return nil
	}

	brs, _, err := ctx.Repo.GitRepo.GetBranchNames(0, 0)
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return nil
	}
	ctx.Data["Branches"] = brs

	// Contains true if the user can create issue dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx.Doer, isPull)

	return labels
}

// Tries to load and set an issue template. The first return value indicates if a template was loaded.
func setTemplateIfExists(ctx *context.Context, ctxDataKey string, possibleFiles []string) (bool, map[string]error) {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		return false, nil
	}

	templateCandidates := make([]string, 0, 1+len(possibleFiles))
	if t := ctx.FormString("template"); t != "" {
		templateCandidates = append(templateCandidates, t)
	}
	templateCandidates = append(templateCandidates, possibleFiles...) // Append files to the end because they should be fallback

	templateErrs := map[string]error{}
	for _, filename := range templateCandidates {
		if ok, _ := commit.HasFile(filename); !ok {
			continue
		}
		template, err := issue_template.UnmarshalFromCommit(commit, filename)
		if err != nil {
			templateErrs[filename] = err
			continue
		}
		ctx.Data[issueTemplateTitleKey] = template.Title
		ctx.Data[ctxDataKey] = template.Content

		if template.Type() == api.IssueTemplateTypeYaml {
			// Replace field default values by values from query
			for _, field := range template.Fields {
				fieldValue := ctx.FormString("field:" + field.ID)
				if fieldValue != "" {
					field.Attributes["value"] = fieldValue
				}
			}

			ctx.Data["Fields"] = template.Fields
			ctx.Data["TemplateFile"] = template.FileName
		}
		labelIDs := make([]string, 0, len(template.Labels))
		if repoLabels, err := issues_model.GetLabelsByRepoID(ctx, ctx.Repo.Repository.ID, "", db.ListOptions{}); err == nil {
			ctx.Data["Labels"] = repoLabels
			if ctx.Repo.Owner.IsOrganization() {
				if orgLabels, err := issues_model.GetLabelsByOrgID(ctx, ctx.Repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{}); err == nil {
					ctx.Data["OrgLabels"] = orgLabels
					repoLabels = append(repoLabels, orgLabels...)
				}
			}

			for _, metaLabel := range template.Labels {
				for _, repoLabel := range repoLabels {
					if strings.EqualFold(repoLabel.Name, metaLabel) {
						repoLabel.IsChecked = true
						labelIDs = append(labelIDs, strconv.FormatInt(repoLabel.ID, 10))
						break
					}
				}
			}

		}

		if template.Ref != "" && !strings.HasPrefix(template.Ref, "refs/") { // Assume that the ref intended is always a branch - for tags users should use refs/tags/<ref>
			template.Ref = git.BranchPrefix + template.Ref
		}
		ctx.Data["HasSelectedLabel"] = len(labelIDs) > 0
		ctx.Data["label_ids"] = strings.Join(labelIDs, ",")
		ctx.Data["Reference"] = template.Ref
		ctx.Data["RefEndName"] = git.RefName(template.Ref).ShortName()
		return true, templateErrs
	}
	return false, templateErrs
}

// NewIssue render creating issue page
func NewIssue(ctx *context.Context) {
	issueConfig, _ := issue_service.GetTemplateConfigFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	hasTemplates := issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)

	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["NewIssueChooseTemplate"] = hasTemplates
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	title := ctx.FormString("title")
	ctx.Data["TitleQuery"] = title
	body := ctx.FormString("body")
	ctx.Data["BodyQuery"] = body

	isProjectsEnabled := ctx.Repo.CanRead(unit.TypeProjects)
	ctx.Data["IsProjectsEnabled"] = isProjectsEnabled
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	milestoneID := ctx.FormInt64("milestone")
	if milestoneID > 0 {
		milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
		if err != nil {
			log.Error("GetMilestoneByID: %d: %v", milestoneID, err)
		} else {
			ctx.Data["milestone_id"] = milestoneID
			ctx.Data["Milestone"] = milestone
		}
	}

	projectID := ctx.FormInt64("project")
	if projectID > 0 && isProjectsEnabled {
		project, err := project_model.GetProjectByID(ctx, projectID)
		if err != nil {
			log.Error("GetProjectByID: %d: %v", projectID, err)
		} else if project.RepoID != ctx.Repo.Repository.ID {
			log.Error("GetProjectByID: %d: %v", projectID, fmt.Errorf("project[%d] not in repo [%d]", project.ID, ctx.Repo.Repository.ID))
		} else {
			ctx.Data["project_id"] = projectID
			ctx.Data["Project"] = project
		}

		if len(ctx.Req.URL.Query().Get("project")) > 0 {
			ctx.Data["redirect_after_creation"] = "project"
		}
	}

	RetrieveRepoMetas(ctx, ctx.Repo.Repository, false)

	_, templateErrs := issue_service.GetTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	templateLoaded, errs := setTemplateIfExists(ctx, issueTemplateKey, IssueTemplateCandidates)
	if len(errs) > 0 {
		for k, v := range errs {
			templateErrs[k] = v
		}
	}
	if ctx.Written() {
		return
	}

	if len(templateErrs) > 0 {
		ctx.Flash.Warning(renderErrorOfTemplates(ctx, templateErrs), true)
	}

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWrite(unit.TypeIssues)

	if !issueConfig.BlankIssuesEnabled && hasTemplates && !templateLoaded {
		// The "issues/new" and "issues/new/choose" share the same query parameters "project" and "milestone", if blank issues are disabled, just redirect to the "issues/choose" page with these parameters.
		ctx.Redirect(fmt.Sprintf("%s/issues/new/choose?%s", ctx.Repo.Repository.Link(), ctx.Req.URL.RawQuery), http.StatusSeeOther)
		return
	}

	ctx.HTML(http.StatusOK, tplIssueNew)
}

func renderErrorOfTemplates(ctx *context.Context, errs map[string]error) string {
	var files []string
	for k := range errs {
		files = append(files, k)
	}
	sort.Strings(files) // keep the output stable

	var lines []string
	for _, file := range files {
		lines = append(lines, fmt.Sprintf("%s: %v", file, errs[file]))
	}

	flashError, err := ctx.RenderToString(tplAlertDetails, map[string]any{
		"Message": ctx.Tr("repo.issues.choose.ignore_invalid_templates"),
		"Summary": ctx.Tr("repo.issues.choose.invalid_templates", len(errs)),
		"Details": utils.SanitizeFlashErrorString(strings.Join(lines, "\n")),
	})
	if err != nil {
		log.Debug("render flash error: %v", err)
		flashError = ctx.Tr("repo.issues.choose.ignore_invalid_templates")
	}
	return flashError
}

// NewIssueChooseTemplate render creating issue from template page
func NewIssueChooseTemplate(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true

	issueTemplates, errs := issue_service.GetTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["IssueTemplates"] = issueTemplates

	if len(errs) > 0 {
		ctx.Flash.Warning(renderErrorOfTemplates(ctx, errs), true)
	}

	if !issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo) {
		// The "issues/new" and "issues/new/choose" share the same query parameters "project" and "milestone", if no template here, just redirect to the "issues/new" page with these parameters.
		ctx.Redirect(fmt.Sprintf("%s/issues/new?%s", ctx.Repo.Repository.Link(), ctx.Req.URL.RawQuery), http.StatusSeeOther)
		return
	}

	issueConfig, err := issue_service.GetTemplateConfigFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["IssueConfig"] = issueConfig
	ctx.Data["IssueConfigError"] = err // ctx.Flash.Err makes problems here

	ctx.Data["milestone"] = ctx.FormInt64("milestone")
	ctx.Data["project"] = ctx.FormInt64("project")

	ctx.HTML(http.StatusOK, tplIssueChoose)
}

// DeleteIssue deletes an issue
func DeleteIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if err := issue_service.DeleteIssue(ctx, ctx.Doer, ctx.Repo.GitRepo, issue); err != nil {
		ctx.ServerError("DeleteIssueByID", err)
		return
	}

	if issue.IsPull {
		ctx.Redirect(fmt.Sprintf("%s/pulls", ctx.Repo.Repository.Link()), http.StatusSeeOther)
		return
	}

	ctx.Redirect(fmt.Sprintf("%s/issues", ctx.Repo.Repository.Link()), http.StatusSeeOther)
}

// ValidateRepoMetas check and returns repository's meta information
func ValidateRepoMetas(ctx *context.Context, form forms.CreateIssueForm, isPull bool) ([]int64, []int64, int64, int64) {
	var (
		repo = ctx.Repo.Repository
		err  error
	)

	labels := RetrieveRepoMetas(ctx, ctx.Repo.Repository, isPull)
	if ctx.Written() {
		return nil, nil, 0, 0
	}

	var labelIDs []int64
	hasSelected := false
	// Check labels.
	if len(form.LabelIDs) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
		if err != nil {
			return nil, nil, 0, 0
		}
		labelIDMark := make(container.Set[int64])
		labelIDMark.AddMultiple(labelIDs...)

		for i := range labels {
			if labelIDMark.Contains(labels[i].ID) {
				labels[i].IsChecked = true
				hasSelected = true
			}
		}
	}

	ctx.Data["Labels"] = labels
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["label_ids"] = form.LabelIDs

	// Check milestone.
	milestoneID := form.MilestoneID
	if milestoneID > 0 {
		milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
		if err != nil {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0, 0
		}
		if milestone.RepoID != repo.ID {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0, 0
		}
		ctx.Data["Milestone"] = milestone
		ctx.Data["milestone_id"] = milestoneID
	}

	if form.ProjectID > 0 {
		p, err := project_model.GetProjectByID(ctx, form.ProjectID)
		if err != nil {
			ctx.ServerError("GetProjectByID", err)
			return nil, nil, 0, 0
		}
		if p.RepoID != ctx.Repo.Repository.ID && p.OwnerID != ctx.Repo.Repository.OwnerID {
			ctx.NotFound("", nil)
			return nil, nil, 0, 0
		}

		ctx.Data["Project"] = p
		ctx.Data["project_id"] = form.ProjectID
	}

	// Check assignees
	var assigneeIDs []int64
	if len(form.AssigneeIDs) > 0 {
		assigneeIDs, err = base.StringsToInt64s(strings.Split(form.AssigneeIDs, ","))
		if err != nil {
			return nil, nil, 0, 0
		}

		// Check if the passed assignees actually exists and is assignable
		for _, aID := range assigneeIDs {
			assignee, err := user_model.GetUserByID(ctx, aID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return nil, nil, 0, 0
			}

			valid, err := access_model.CanBeAssigned(ctx, assignee, repo, isPull)
			if err != nil {
				ctx.ServerError("CanBeAssigned", err)
				return nil, nil, 0, 0
			}

			if !valid {
				ctx.ServerError("canBeAssigned", repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: repo.Name})
				return nil, nil, 0, 0
			}
		}
	}

	// Keep the old assignee id thingy for compatibility reasons
	if form.AssigneeID > 0 {
		assigneeIDs = append(assigneeIDs, form.AssigneeID)
	}

	return labelIDs, assigneeIDs, milestoneID, form.ProjectID
}

// NewIssuePost response for creating new issue
func NewIssuePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateIssueForm)
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["NewIssueChooseTemplate"] = issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	var (
		repo        = ctx.Repo.Repository
		attachments []string
	)

	labelIDs, assigneeIDs, milestoneID, projectID := ValidateRepoMetas(ctx, *form, false)
	if ctx.Written() {
		return
	}

	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplIssueNew)
		return
	}

	if util.IsEmptyString(form.Title) {
		ctx.RenderWithErr(ctx.Tr("repo.issues.new.title_empty"), tplIssueNew, form)
		return
	}

	content := form.Content
	if filename := ctx.Req.Form.Get("template-file"); filename != "" {
		if template, err := issue_template.UnmarshalFromRepo(ctx.Repo.GitRepo, ctx.Repo.Repository.DefaultBranch, filename); err == nil {
			content = issue_template.RenderToMarkdown(template, ctx.Req.Form)
		}
	}

	issue := &issues_model.Issue{
		RepoID:      repo.ID,
		Repo:        repo,
		Title:       form.Title,
		PosterID:    ctx.Doer.ID,
		Poster:      ctx.Doer,
		MilestoneID: milestoneID,
		Content:     content,
		Ref:         form.Ref,
	}

	if err := issue_service.NewIssue(ctx, repo, issue, labelIDs, attachments, assigneeIDs); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err.Error())
			return
		}
		ctx.ServerError("NewIssue", err)
		return
	}

	if projectID > 0 {
		if !ctx.Repo.CanRead(unit.TypeProjects) {
			// User must also be able to see the project.
			ctx.Error(http.StatusBadRequest, "user hasn't permissions to read projects")
			return
		}
		if err := issues_model.ChangeProjectAssign(issue, ctx.Doer, projectID); err != nil {
			ctx.ServerError("ChangeProjectAssign", err)
			return
		}
	}

	log.Trace("Issue created: %d/%d", repo.ID, issue.ID)
	if ctx.FormString("redirect_after_creation") == "project" && projectID > 0 {
		ctx.Redirect(ctx.Repo.RepoLink + "/projects/" + strconv.FormatInt(projectID, 10))
	} else {
		ctx.Redirect(issue.Link())
	}
}

// roleDescriptor returns the Role Descriptor for a comment in/with the given repo, poster and issue
func roleDescriptor(ctx stdCtx.Context, repo *repo_model.Repository, poster *user_model.User, issue *issues_model.Issue, hasOriginalAuthor bool) (issues_model.RoleDescriptor, error) {
	if hasOriginalAuthor {
		return issues_model.RoleDescriptorNone, nil
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, poster)
	if err != nil {
		return issues_model.RoleDescriptorNone, err
	}

	// By default the poster has no roles on the comment.
	roleDescriptor := issues_model.RoleDescriptorNone

	// Check if the poster is owner of the repo.
	if perm.IsOwner() {
		// If the poster isn't a admin, enable the owner role.
		if !poster.IsAdmin {
			roleDescriptor = roleDescriptor.WithRole(issues_model.RoleDescriptorOwner)
		} else {

			// Otherwise check if poster is the real repo admin.
			ok, err := access_model.IsUserRealRepoAdmin(repo, poster)
			if err != nil {
				return issues_model.RoleDescriptorNone, err
			}
			if ok {
				roleDescriptor = roleDescriptor.WithRole(issues_model.RoleDescriptorOwner)
			}
		}
	}

	// Is the poster can write issues or pulls to the repo, enable the Writer role.
	// Only enable this if the poster doesn't have the owner role already.
	if !roleDescriptor.HasRole("Owner") && perm.CanWriteIssuesOrPulls(issue.IsPull) {
		roleDescriptor = roleDescriptor.WithRole(issues_model.RoleDescriptorWriter)
	}

	// If the poster is the actual poster of the issue, enable Poster role.
	if issue.IsPoster(poster.ID) {
		roleDescriptor = roleDescriptor.WithRole(issues_model.RoleDescriptorPoster)
	}

	return roleDescriptor, nil
}

func getBranchData(ctx *context.Context, issue *issues_model.Issue) {
	ctx.Data["BaseBranch"] = nil
	ctx.Data["HeadBranch"] = nil
	ctx.Data["HeadUserName"] = nil
	ctx.Data["BaseName"] = ctx.Repo.Repository.OwnerName
	if issue.IsPull {
		pull := issue.PullRequest
		ctx.Data["BaseBranch"] = pull.BaseBranch
		ctx.Data["HeadBranch"] = pull.HeadBranch
		ctx.Data["HeadUserName"] = pull.MustHeadUserName(ctx)
	}
}

// ViewIssue render issue view page
func ViewIssue(ctx *context.Context) {
	if ctx.Params(":type") == "issues" {
		// If issue was requested we check if repo has external tracker and redirect
		extIssueUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
		if err == nil && extIssueUnit != nil {
			if extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == markup.IssueNameStyleNumeric || extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == "" {
				metas := ctx.Repo.Repository.ComposeMetas()
				metas["index"] = ctx.Params(":index")
				res, err := vars.Expand(extIssueUnit.ExternalTrackerConfig().ExternalTrackerFormat, metas)
				if err != nil {
					log.Error("unable to expand template vars for issue url. issue: %s, err: %v", metas["index"], err)
					ctx.ServerError("Expand", err)
					return
				}
				ctx.Redirect(res)
				return
			}
		} else if err != nil && !repo_model.IsErrUnitTypeNotExist(err) {
			ctx.ServerError("GetUnit", err)
			return
		}
	}

	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}
	if issue.Repo == nil {
		issue.Repo = ctx.Repo.Repository
	}

	// Make sure type and URL matches.
	if ctx.Params(":type") == "issues" && issue.IsPull {
		ctx.Redirect(issue.Link())
		return
	} else if ctx.Params(":type") == "pulls" && !issue.IsPull {
		ctx.Redirect(issue.Link())
		return
	}

	if issue.IsPull {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsPullList"] = true
		ctx.Data["PageIsPullConversation"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsIssueList"] = true
		ctx.Data["NewIssueChooseTemplate"] = issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)
	}

	if issue.IsPull && !ctx.Repo.CanRead(unit.TypeIssues) {
		ctx.Data["IssueType"] = "pulls"
	} else if !issue.IsPull && !ctx.Repo.CanRead(unit.TypePullRequests) {
		ctx.Data["IssueType"] = "issues"
	} else {
		ctx.Data["IssueType"] = "all"
	}

	ctx.Data["IsProjectsEnabled"] = ctx.Repo.CanRead(unit.TypeProjects)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	if err = issue.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	if err = filterXRefComments(ctx, issue); err != nil {
		ctx.ServerError("filterXRefComments", err)
		return
	}

	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, issue.Title)

	iw := new(issues_model.IssueWatch)
	if ctx.Doer != nil {
		iw.UserID = ctx.Doer.ID
		iw.IssueID = issue.ID
		iw.IsWatching, err = issues_model.CheckIssueWatch(ctx.Doer, issue)
		if err != nil {
			ctx.ServerError("CheckIssueWatch", err)
			return
		}
	}
	ctx.Data["IssueWatch"] = iw

	issue.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.Repo.RepoLink,
		Metas:     ctx.Repo.Repository.ComposeMetas(),
		GitRepo:   ctx.Repo.GitRepo,
		Ctx:       ctx,
	}, issue.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	repo := ctx.Repo.Repository

	// Get more information if it's a pull request.
	if issue.IsPull {
		if issue.PullRequest.HasMerged {
			ctx.Data["DisableStatusChange"] = issue.PullRequest.HasMerged
			PrepareMergedViewPullInfo(ctx, issue)
		} else {
			PrepareViewPullInfo(ctx, issue)
			ctx.Data["DisableStatusChange"] = ctx.Data["IsPullRequestBroken"] == true && issue.IsClosed
		}
		if ctx.Written() {
			return
		}
	}

	// Metas.
	// Check labels.
	labelIDMark := make(container.Set[int64])
	for _, label := range issue.Labels {
		labelIDMark.Add(label.ID)
	}
	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}
	ctx.Data["Labels"] = labels

	if repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return
		}
		ctx.Data["OrgLabels"] = orgLabels

		labels = append(labels, orgLabels...)
	}

	hasSelected := false
	for i := range labels {
		if labelIDMark.Contains(labels[i].ID) {
			labels[i].IsChecked = true
			hasSelected = true
		}
	}
	ctx.Data["HasSelectedLabel"] = hasSelected

	// Check milestone and assignee.
	if ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		RetrieveRepoMilestonesAndAssignees(ctx, repo)
		retrieveProjects(ctx, repo)

		if ctx.Written() {
			return
		}
	}

	if issue.IsPull {
		canChooseReviewer := ctx.Repo.CanWrite(unit.TypePullRequests)
		if ctx.Doer != nil && ctx.IsSigned {
			if !canChooseReviewer {
				canChooseReviewer = ctx.Doer.ID == issue.PosterID
			}
			if !canChooseReviewer {
				canChooseReviewer, err = issues_model.IsOfficialReviewer(ctx, issue, ctx.Doer)
				if err != nil {
					ctx.ServerError("IsOfficialReviewer", err)
					return
				}
			}
		}

		RetrieveRepoReviewers(ctx, repo, issue, canChooseReviewer)
		if ctx.Written() {
			return
		}
	}

	if ctx.IsSigned {
		// Update issue-user.
		if err = activities_model.SetIssueReadBy(ctx, issue.ID, ctx.Doer.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return
		}
	}

	var (
		role                 issues_model.RoleDescriptor
		ok                   bool
		marked               = make(map[int64]issues_model.RoleDescriptor)
		comment              *issues_model.Comment
		participants         = make([]*user_model.User, 1, 10)
		latestCloseCommentID int64
	)
	if ctx.Repo.Repository.IsTimetrackerEnabled(ctx) {
		if ctx.IsSigned {
			// Deal with the stopwatch
			ctx.Data["IsStopwatchRunning"] = issues_model.StopwatchExists(ctx.Doer.ID, issue.ID)
			if !ctx.Data["IsStopwatchRunning"].(bool) {
				var exists bool
				var swIssue *issues_model.Issue
				if exists, _, swIssue, err = issues_model.HasUserStopwatch(ctx, ctx.Doer.ID); err != nil {
					ctx.ServerError("HasUserStopwatch", err)
					return
				}
				ctx.Data["HasUserStopwatch"] = exists
				if exists {
					// Add warning if the user has already a stopwatch
					// Add link to the issue of the already running stopwatch
					ctx.Data["OtherStopwatchURL"] = swIssue.Link()
				}
			}
			ctx.Data["CanUseTimetracker"] = ctx.Repo.CanUseTimetracker(issue, ctx.Doer)
		} else {
			ctx.Data["CanUseTimetracker"] = false
		}
		if ctx.Data["WorkingUsers"], err = issues_model.TotalTimes(&issues_model.FindTrackedTimesOptions{IssueID: issue.ID}); err != nil {
			ctx.ServerError("TotalTimes", err)
			return
		}
	}

	// Check if the user can use the dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx.Doer, issue.IsPull)

	// check if dependencies can be created across repositories
	ctx.Data["AllowCrossRepositoryDependencies"] = setting.Service.AllowCrossRepositoryDependencies

	if issue.ShowRole, err = roleDescriptor(ctx, repo, issue.Poster, issue, issue.HasOriginalAuthor()); err != nil {
		ctx.ServerError("roleDescriptor", err)
		return
	}
	marked[issue.PosterID] = issue.ShowRole

	// Render comments and and fetch participants.
	participants[0] = issue.Poster
	for _, comment = range issue.Comments {
		comment.Issue = issue

		if err := comment.LoadPoster(ctx); err != nil {
			ctx.ServerError("LoadPoster", err)
			return
		}

		if comment.Type == issues_model.CommentTypeComment || comment.Type == issues_model.CommentTypeReview {
			if err := comment.LoadAttachments(ctx); err != nil {
				ctx.ServerError("LoadAttachments", err)
				return
			}

			comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
				URLPrefix: ctx.Repo.RepoLink,
				Metas:     ctx.Repo.Repository.ComposeMetas(),
				GitRepo:   ctx.Repo.GitRepo,
				Ctx:       ctx,
			}, comment.Content)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
			// Check tag.
			role, ok = marked[comment.PosterID]
			if ok {
				comment.ShowRole = role
				continue
			}

			comment.ShowRole, err = roleDescriptor(ctx, repo, comment.Poster, issue, comment.HasOriginalAuthor())
			if err != nil {
				ctx.ServerError("roleDescriptor", err)
				return
			}
			marked[comment.PosterID] = comment.ShowRole
			participants = addParticipant(comment.Poster, participants)
		} else if comment.Type == issues_model.CommentTypeLabel {
			if err = comment.LoadLabel(); err != nil {
				ctx.ServerError("LoadLabel", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeMilestone {
			if err = comment.LoadMilestone(ctx); err != nil {
				ctx.ServerError("LoadMilestone", err)
				return
			}
			ghostMilestone := &issues_model.Milestone{
				ID:   -1,
				Name: ctx.Tr("repo.issues.deleted_milestone"),
			}
			if comment.OldMilestoneID > 0 && comment.OldMilestone == nil {
				comment.OldMilestone = ghostMilestone
			}
			if comment.MilestoneID > 0 && comment.Milestone == nil {
				comment.Milestone = ghostMilestone
			}
		} else if comment.Type == issues_model.CommentTypeProject {

			if err = comment.LoadProject(); err != nil {
				ctx.ServerError("LoadProject", err)
				return
			}

			ghostProject := &project_model.Project{
				ID:    -1,
				Title: ctx.Tr("repo.issues.deleted_project"),
			}

			if comment.OldProjectID > 0 && comment.OldProject == nil {
				comment.OldProject = ghostProject
			}

			if comment.ProjectID > 0 && comment.Project == nil {
				comment.Project = ghostProject
			}

		} else if comment.Type == issues_model.CommentTypeAssignees || comment.Type == issues_model.CommentTypeReviewRequest {
			if err = comment.LoadAssigneeUserAndTeam(); err != nil {
				ctx.ServerError("LoadAssigneeUserAndTeam", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeRemoveDependency || comment.Type == issues_model.CommentTypeAddDependency {
			if err = comment.LoadDepIssueDetails(); err != nil {
				if !issues_model.IsErrIssueNotExist(err) {
					ctx.ServerError("LoadDepIssueDetails", err)
					return
				}
			}
		} else if comment.Type.HasContentSupport() {
			comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
				URLPrefix: ctx.Repo.RepoLink,
				Metas:     ctx.Repo.Repository.ComposeMetas(),
				GitRepo:   ctx.Repo.GitRepo,
				Ctx:       ctx,
			}, comment.Content)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
			if err = comment.LoadReview(); err != nil && !issues_model.IsErrReviewNotExist(err) {
				ctx.ServerError("LoadReview", err)
				return
			}
			participants = addParticipant(comment.Poster, participants)
			if comment.Review == nil {
				continue
			}
			if err = comment.Review.LoadAttributes(ctx); err != nil {
				if !user_model.IsErrUserNotExist(err) {
					ctx.ServerError("Review.LoadAttributes", err)
					return
				}
				comment.Review.Reviewer = user_model.NewGhostUser()
			}
			if err = comment.Review.LoadCodeComments(ctx); err != nil {
				ctx.ServerError("Review.LoadCodeComments", err)
				return
			}
			for _, codeComments := range comment.Review.CodeComments {
				for _, lineComments := range codeComments {
					for _, c := range lineComments {
						// Check tag.
						role, ok = marked[c.PosterID]
						if ok {
							c.ShowRole = role
							continue
						}

						c.ShowRole, err = roleDescriptor(ctx, repo, c.Poster, issue, c.HasOriginalAuthor())
						if err != nil {
							ctx.ServerError("roleDescriptor", err)
							return
						}
						marked[c.PosterID] = c.ShowRole
						participants = addParticipant(c.Poster, participants)
					}
				}
			}
			if err = comment.LoadResolveDoer(); err != nil {
				ctx.ServerError("LoadResolveDoer", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypePullRequestPush {
			participants = addParticipant(comment.Poster, participants)
			if err = comment.LoadPushCommits(ctx); err != nil {
				ctx.ServerError("LoadPushCommits", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeAddTimeManual ||
			comment.Type == issues_model.CommentTypeStopTracking {
			// drop error since times could be pruned from DB..
			_ = comment.LoadTime()
		}

		if comment.Type == issues_model.CommentTypeClose || comment.Type == issues_model.CommentTypeMergePull {
			// record ID of the latest closed/merged comment.
			// if PR is closed, the comments whose type is CommentTypePullRequestPush(29) after latestCloseCommentID won't be rendered.
			latestCloseCommentID = comment.ID
		}
	}

	ctx.Data["LatestCloseCommentID"] = latestCloseCommentID

	// Combine multiple label assignments into a single comment
	combineLabelComments(issue)

	getBranchData(ctx, issue)
	if issue.IsPull {
		pull := issue.PullRequest
		pull.Issue = issue
		canDelete := false
		ctx.Data["AllowMerge"] = false

		if ctx.IsSigned {
			if err := pull.LoadHeadRepo(ctx); err != nil {
				log.Error("LoadHeadRepo: %v", err)
			} else if pull.HeadRepo != nil {
				perm, err := access_model.GetUserRepoPermission(ctx, pull.HeadRepo, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}
				if perm.CanWrite(unit.TypeCode) {
					// Check if branch is not protected
					if pull.HeadBranch != pull.HeadRepo.DefaultBranch {
						if protected, err := git_model.IsBranchProtected(ctx, pull.HeadRepo.ID, pull.HeadBranch); err != nil {
							log.Error("IsProtectedBranch: %v", err)
						} else if !protected {
							canDelete = true
							ctx.Data["DeleteBranchLink"] = issue.Link() + "/cleanup"
						}
					}
					ctx.Data["CanWriteToHeadRepo"] = true
				}
			}

			if err := pull.LoadBaseRepo(ctx); err != nil {
				log.Error("LoadBaseRepo: %v", err)
			}
			perm, err := access_model.GetUserRepoPermission(ctx, pull.BaseRepo, ctx.Doer)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return
			}
			ctx.Data["AllowMerge"], err = pull_service.IsUserAllowedToMerge(ctx, pull, perm, ctx.Doer)
			if err != nil {
				ctx.ServerError("IsUserAllowedToMerge", err)
				return
			}

			if ctx.Data["CanMarkConversation"], err = issues_model.CanMarkConversation(issue, ctx.Doer); err != nil {
				ctx.ServerError("CanMarkConversation", err)
				return
			}
		}

		prUnit, err := repo.GetUnit(ctx, unit.TypePullRequests)
		if err != nil {
			ctx.ServerError("GetUnit", err)
			return
		}
		prConfig := prUnit.PullRequestsConfig()

		var mergeStyle repo_model.MergeStyle
		// Check correct values and select default
		if ms, ok := ctx.Data["MergeStyle"].(repo_model.MergeStyle); !ok ||
			!prConfig.IsMergeStyleAllowed(ms) {
			defaultMergeStyle := prConfig.GetDefaultMergeStyle()
			if prConfig.IsMergeStyleAllowed(defaultMergeStyle) && !ok {
				mergeStyle = defaultMergeStyle
			} else if prConfig.AllowMerge {
				mergeStyle = repo_model.MergeStyleMerge
			} else if prConfig.AllowRebase {
				mergeStyle = repo_model.MergeStyleRebase
			} else if prConfig.AllowRebaseMerge {
				mergeStyle = repo_model.MergeStyleRebaseMerge
			} else if prConfig.AllowSquash {
				mergeStyle = repo_model.MergeStyleSquash
			} else if prConfig.AllowManualMerge {
				mergeStyle = repo_model.MergeStyleManuallyMerged
			}
		}

		ctx.Data["MergeStyle"] = mergeStyle

		defaultMergeMessage, defaultMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, mergeStyle)
		if err != nil {
			ctx.ServerError("GetDefaultMergeMessage", err)
			return
		}
		ctx.Data["DefaultMergeMessage"] = defaultMergeMessage
		ctx.Data["DefaultMergeBody"] = defaultMergeBody

		defaultSquashMergeMessage, defaultSquashMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, repo_model.MergeStyleSquash)
		if err != nil {
			ctx.ServerError("GetDefaultSquashMergeMessage", err)
			return
		}
		ctx.Data["DefaultSquashMergeMessage"] = defaultSquashMergeMessage
		ctx.Data["DefaultSquashMergeBody"] = defaultSquashMergeBody

		pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pull.BaseRepoID, pull.BaseBranch)
		if err != nil {
			ctx.ServerError("LoadProtectedBranch", err)
			return
		}
		ctx.Data["ShowMergeInstructions"] = true
		if pb != nil {
			pb.Repo = pull.BaseRepo
			var showMergeInstructions bool
			if ctx.Doer != nil {
				showMergeInstructions = pb.CanUserPush(ctx, ctx.Doer)
			}
			ctx.Data["ProtectedBranch"] = pb
			ctx.Data["IsBlockedByApprovals"] = !issues_model.HasEnoughApprovals(ctx, pb, pull)
			ctx.Data["IsBlockedByRejection"] = issues_model.MergeBlockedByRejectedReview(ctx, pb, pull)
			ctx.Data["IsBlockedByOfficialReviewRequests"] = issues_model.MergeBlockedByOfficialReviewRequests(ctx, pb, pull)
			ctx.Data["IsBlockedByOutdatedBranch"] = issues_model.MergeBlockedByOutdatedBranch(pb, pull)
			ctx.Data["GrantedApprovals"] = issues_model.GetGrantedApprovalsCount(ctx, pb, pull)
			ctx.Data["RequireSigned"] = pb.RequireSignedCommits
			ctx.Data["ChangedProtectedFiles"] = pull.ChangedProtectedFiles
			ctx.Data["IsBlockedByChangedProtectedFiles"] = len(pull.ChangedProtectedFiles) != 0
			ctx.Data["ChangedProtectedFilesNum"] = len(pull.ChangedProtectedFiles)
			ctx.Data["ShowMergeInstructions"] = showMergeInstructions
		}
		ctx.Data["WillSign"] = false
		if ctx.Doer != nil {
			sign, key, _, err := asymkey_service.SignMerge(ctx, pull, ctx.Doer, pull.BaseRepo.RepoPath(), pull.BaseBranch, pull.GetGitRefName())
			ctx.Data["WillSign"] = sign
			ctx.Data["SigningKey"] = key
			if err != nil {
				if asymkey_service.IsErrWontSign(err) {
					ctx.Data["WontSignReason"] = err.(*asymkey_service.ErrWontSign).Reason
				} else {
					ctx.Data["WontSignReason"] = "error"
					log.Error("Error whilst checking if could sign pr %d in repo %s. Error: %v", pull.ID, pull.BaseRepo.FullName(), err)
				}
			}
		} else {
			ctx.Data["WontSignReason"] = "not_signed_in"
		}

		isPullBranchDeletable := canDelete &&
			pull.HeadRepo != nil &&
			git.IsBranchExist(ctx, pull.HeadRepo.RepoPath(), pull.HeadBranch) &&
			(!pull.HasMerged || ctx.Data["HeadBranchCommitID"] == ctx.Data["PullHeadCommitID"])

		if isPullBranchDeletable && pull.HasMerged {
			exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(ctx, pull.HeadRepoID, pull.HeadBranch)
			if err != nil {
				ctx.ServerError("HasUnmergedPullRequestsByHeadInfo", err)
				return
			}

			isPullBranchDeletable = !exist
		}
		ctx.Data["IsPullBranchDeletable"] = isPullBranchDeletable

		stillCanManualMerge := func() bool {
			if pull.HasMerged || issue.IsClosed || !ctx.IsSigned {
				return false
			}
			if pull.CanAutoMerge() || pull.IsWorkInProgress() || pull.IsChecking() {
				return false
			}
			if (ctx.Doer.IsAdmin || ctx.Repo.IsAdmin()) && prConfig.AllowManualMerge {
				return true
			}

			return false
		}

		ctx.Data["StillCanManualMerge"] = stillCanManualMerge()

		// Check if there is a pending pr merge
		ctx.Data["HasPendingPullRequestMerge"], ctx.Data["PendingPullRequestMerge"], err = pull_model.GetScheduledMergeByPullID(ctx, pull.ID)
		if err != nil {
			ctx.ServerError("GetScheduledMergeByPullID", err)
			return
		}
	}

	// Get Dependencies
	blockedBy, err := issue.BlockedByDependencies(ctx, db.ListOptions{})
	if err != nil {
		ctx.ServerError("BlockedByDependencies", err)
		return
	}
	ctx.Data["BlockedByDependencies"], ctx.Data["BlockedByDependenciesNotPermitted"] = checkBlockedByIssues(ctx, blockedBy)
	if ctx.Written() {
		return
	}

	blocking, err := issue.BlockingDependencies(ctx)
	if err != nil {
		ctx.ServerError("BlockingDependencies", err)
		return
	}

	ctx.Data["BlockingDependencies"], ctx.Data["BlockingByDependenciesNotPermitted"] = checkBlockedByIssues(ctx, blocking)
	if ctx.Written() {
		return
	}

	var pinAllowed bool
	if !issue.IsPinned() {
		pinAllowed, err = issues_model.IsNewPinAllowed(ctx, issue.RepoID, issue.IsPull)
		if err != nil {
			ctx.ServerError("IsNewPinAllowed", err)
			return
		}
	} else {
		pinAllowed = true
	}

	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
	ctx.Data["Issue"] = issue
	ctx.Data["Reference"] = issue.Ref
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + url.QueryEscape(ctx.Data["Link"].(string))
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.Doer.ID)
	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)
	ctx.Data["HasProjectsWritePermission"] = ctx.Repo.CanWrite(unit.TypeProjects)
	ctx.Data["IsRepoAdmin"] = ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	ctx.Data["LockReasons"] = setting.Repository.Issue.LockReasons
	ctx.Data["RefEndName"] = git.RefName(issue.Ref).ShortName()
	ctx.Data["NewPinAllowed"] = pinAllowed
	ctx.Data["PinEnabled"] = setting.Repository.Issue.MaxPinned != 0

	var hiddenCommentTypes *big.Int
	if ctx.IsSigned {
		val, err := user_model.GetUserSetting(ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes)
		if err != nil {
			ctx.ServerError("GetUserSetting", err)
			return
		}
		hiddenCommentTypes, _ = new(big.Int).SetString(val, 10) // we can safely ignore the failed conversion here
	}
	ctx.Data["ShouldShowCommentType"] = func(commentType issues_model.CommentType) bool {
		return hiddenCommentTypes == nil || hiddenCommentTypes.Bit(int(commentType)) == 0
	}

	ctx.HTML(http.StatusOK, tplIssueView)
}

func checkBlockedByIssues(ctx *context.Context, blockers []*issues_model.DependencyInfo) (canRead, notPermitted []*issues_model.DependencyInfo) {
	var lastRepoID int64
	var lastPerm access_model.Permission
	for i, blocker := range blockers {
		// Get the permissions for this repository
		perm := lastPerm
		if lastRepoID != blocker.Repository.ID {
			if blocker.Repository.ID == ctx.Repo.Repository.ID {
				perm = ctx.Repo.Permission
			} else {
				var err error
				perm, err = access_model.GetUserRepoPermission(ctx, &blocker.Repository, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}
			}
			lastRepoID = blocker.Repository.ID
		}

		// check permission
		if !perm.CanReadIssuesOrPulls(blocker.Issue.IsPull) {
			blockers[len(notPermitted)], blockers[i] = blocker, blockers[len(notPermitted)]
			notPermitted = blockers[:len(notPermitted)+1]
		}
	}
	blockers = blockers[len(notPermitted):]
	sortDependencyInfo(blockers)
	sortDependencyInfo(notPermitted)

	return blockers, notPermitted
}

func sortDependencyInfo(blockers []*issues_model.DependencyInfo) {
	sort.Slice(blockers, func(i, j int) bool {
		if blockers[i].RepoID == blockers[j].RepoID {
			return blockers[i].Issue.CreatedUnix < blockers[j].Issue.CreatedUnix
		}
		return blockers[i].RepoID < blockers[j].RepoID
	})
}

// GetActionIssue will return the issue which is used in the context.
func GetActionIssue(ctx *context.Context) *issues_model.Issue {
	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByIndex", issues_model.IsErrIssueNotExist, err)
		return nil
	}
	issue.Repo = ctx.Repo.Repository
	checkIssueRights(ctx, issue)
	if ctx.Written() {
		return nil
	}
	if err = issue.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return nil
	}
	return issue
}

func checkIssueRights(ctx *context.Context, issue *issues_model.Issue) {
	if issue.IsPull && !ctx.Repo.CanRead(unit.TypePullRequests) ||
		!issue.IsPull && !ctx.Repo.CanRead(unit.TypeIssues) {
		ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
	}
}

func getActionIssues(ctx *context.Context) issues_model.IssueList {
	commaSeparatedIssueIDs := ctx.FormString("issue_ids")
	if len(commaSeparatedIssueIDs) == 0 {
		return nil
	}
	issueIDs := make([]int64, 0, 10)
	for _, stringIssueID := range strings.Split(commaSeparatedIssueIDs, ",") {
		issueID, err := strconv.ParseInt(stringIssueID, 10, 64)
		if err != nil {
			ctx.ServerError("ParseInt", err)
			return nil
		}
		issueIDs = append(issueIDs, issueID)
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		ctx.ServerError("GetIssuesByIDs", err)
		return nil
	}
	// Check access rights for all issues
	issueUnitEnabled := ctx.Repo.CanRead(unit.TypeIssues)
	prUnitEnabled := ctx.Repo.CanRead(unit.TypePullRequests)
	for _, issue := range issues {
		if issue.RepoID != ctx.Repo.Repository.ID {
			ctx.NotFound("some issue's RepoID is incorrect", errors.New("some issue's RepoID is incorrect"))
			return nil
		}
		if issue.IsPull && !prUnitEnabled || !issue.IsPull && !issueUnitEnabled {
			ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
			return nil
		}
		if err = issue.LoadAttributes(ctx); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return nil
		}
	}
	return issues
}

// GetIssueInfo get an issue of a repository
func GetIssueInfo(ctx *context.Context) {
	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.Error(http.StatusNotFound)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err.Error())
		}
		return
	}

	if issue.IsPull {
		// Need to check if Pulls are enabled and we can read Pulls
		if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(unit.TypePullRequests) {
			ctx.Error(http.StatusNotFound)
			return
		}
	} else {
		// Need to check if Issues are enabled and we can read Issues
		if !ctx.Repo.CanRead(unit.TypeIssues) {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	ctx.JSON(http.StatusOK, convert.ToIssue(ctx, issue))
}

// UpdateIssueTitle change issue's title
func UpdateIssueTitle(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	}

	title := ctx.FormTrim("title")
	if len(title) == 0 {
		ctx.Error(http.StatusNoContent)
		return
	}

	if err := issue_service.ChangeTitle(ctx, issue, ctx.Doer, title); err != nil {
		ctx.ServerError("ChangeTitle", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"title": issue.Title,
	})
}

// UpdateIssueRef change issue's ref (branch)
func UpdateIssueRef(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) || issue.IsPull {
		ctx.Error(http.StatusForbidden)
		return
	}

	ref := ctx.FormTrim("ref")

	if err := issue_service.ChangeIssueRef(ctx, issue, ctx.Doer, ref); err != nil {
		ctx.ServerError("ChangeRef", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"ref": ref,
	})
}

// UpdateIssueContent change issue's content
func UpdateIssueContent(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != issue.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	}

	if err := issue_service.ChangeContent(issue, ctx.Doer, ctx.Req.FormValue("content")); err != nil {
		ctx.ServerError("ChangeContent", err)
		return
	}

	// when update the request doesn't intend to update attachments (eg: change checkbox state), ignore attachment updates
	if !ctx.FormBool("ignore_attachments") {
		if err := updateAttachments(ctx, issue, ctx.FormStrings("files[]")); err != nil {
			ctx.ServerError("UpdateAttachments", err)
			return
		}
	}

	content, err := markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.FormString("context"), // FIXME: <- IS THIS SAFE ?
		Metas:     ctx.Repo.Repository.ComposeMetas(),
		GitRepo:   ctx.Repo.GitRepo,
		Ctx:       ctx,
	}, issue.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":     content,
		"attachments": attachmentsHTML(ctx, issue.Attachments, issue.Content),
	})
}

// UpdateIssueDeadline updates an issue deadline
func UpdateIssueDeadline(ctx *context.Context) {
	form := web.GetForm(ctx).(*api.EditDeadlineOption)
	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err.Error())
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
			23, 59, 59, 0, time.Local)
		deadlineUnix = timeutil.TimeStamp(deadline.Unix())
	}

	if err := issues_model.UpdateIssueDeadline(issue, deadlineUnix, ctx.Doer); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateIssueDeadline", err.Error())
		return
	}

	ctx.JSON(http.StatusCreated, api.IssueDeadline{Deadline: &deadline})
}

// UpdateIssueMilestone change issue's milestone
func UpdateIssueMilestone(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	milestoneID := ctx.FormInt64("id")
	for _, issue := range issues {
		oldMilestoneID := issue.MilestoneID
		if oldMilestoneID == milestoneID {
			continue
		}
		issue.MilestoneID = milestoneID
		if err := issue_service.ChangeMilestoneAssign(issue, ctx.Doer, oldMilestoneID); err != nil {
			ctx.ServerError("ChangeMilestoneAssign", err)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"ok": true,
	})
}

// UpdateIssueAssignee change issue's or pull's assignee
func UpdateIssueAssignee(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	assigneeID := ctx.FormInt64("id")
	action := ctx.FormString("action")

	for _, issue := range issues {
		switch action {
		case "clear":
			if err := issue_service.DeleteNotPassedAssignee(ctx, issue, ctx.Doer, []*user_model.User{}); err != nil {
				ctx.ServerError("ClearAssignees", err)
				return
			}
		default:
			assignee, err := user_model.GetUserByID(ctx, assigneeID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return
			}

			valid, err := access_model.CanBeAssigned(ctx, assignee, issue.Repo, issue.IsPull)
			if err != nil {
				ctx.ServerError("canBeAssigned", err)
				return
			}
			if !valid {
				ctx.ServerError("canBeAssigned", repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: assigneeID, RepoName: issue.Repo.Name})
				return
			}

			_, _, err = issue_service.ToggleAssigneeWithNotify(ctx, issue, ctx.Doer, assigneeID)
			if err != nil {
				ctx.ServerError("ToggleAssignee", err)
				return
			}
		}
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"ok": true,
	})
}

// UpdatePullReviewRequest add or remove review request
func UpdatePullReviewRequest(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	reviewID := ctx.FormInt64("id")
	action := ctx.FormString("action")

	// TODO: Not support 'clear' now
	if action != "attach" && action != "detach" {
		ctx.Status(http.StatusForbidden)
		return
	}

	for _, issue := range issues {
		if err := issue.LoadRepo(ctx); err != nil {
			ctx.ServerError("issue.LoadRepo", err)
			return
		}

		if !issue.IsPull {
			log.Warn(
				"UpdatePullReviewRequest: refusing to add review request for non-PR issue %-v#%d",
				issue.Repo, issue.Index,
			)
			ctx.Status(http.StatusForbidden)
			return
		}
		if reviewID < 0 {
			// negative reviewIDs represent team requests
			if err := issue.Repo.LoadOwner(ctx); err != nil {
				ctx.ServerError("issue.Repo.LoadOwner", err)
				return
			}

			if !issue.Repo.Owner.IsOrganization() {
				log.Warn(
					"UpdatePullReviewRequest: refusing to add team review request for %s#%d owned by non organization UID[%d]",
					issue.Repo.FullName(), issue.Index, issue.Repo.ID,
				)
				ctx.Status(http.StatusForbidden)
				return
			}

			team, err := organization.GetTeamByID(ctx, -reviewID)
			if err != nil {
				ctx.ServerError("GetTeamByID", err)
				return
			}

			if team.OrgID != issue.Repo.OwnerID {
				log.Warn(
					"UpdatePullReviewRequest: refusing to add team review request for UID[%d] team %s to %s#%d owned by UID[%d]",
					team.OrgID, team.Name, issue.Repo.FullName(), issue.Index, issue.Repo.ID)
				ctx.Status(http.StatusForbidden)
				return
			}

			err = issue_service.IsValidTeamReviewRequest(ctx, team, ctx.Doer, action == "attach", issue)
			if err != nil {
				if issues_model.IsErrNotValidReviewRequest(err) {
					log.Warn(
						"UpdatePullReviewRequest: refusing to add invalid team review request for UID[%d] team %s to %s#%d owned by UID[%d]: Error: %v",
						team.OrgID, team.Name, issue.Repo.FullName(), issue.Index, issue.Repo.ID,
						err,
					)
					ctx.Status(http.StatusForbidden)
					return
				}
				ctx.ServerError("IsValidTeamReviewRequest", err)
				return
			}

			_, err = issue_service.TeamReviewRequest(ctx, issue, ctx.Doer, team, action == "attach")
			if err != nil {
				ctx.ServerError("TeamReviewRequest", err)
				return
			}
			continue
		}

		reviewer, err := user_model.GetUserByID(ctx, reviewID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				log.Warn(
					"UpdatePullReviewRequest: requested reviewer [%d] for %-v to %-v#%d is not exist: Error: %v",
					reviewID, issue.Repo, issue.Index,
					err,
				)
				ctx.Status(http.StatusForbidden)
				return
			}
			ctx.ServerError("GetUserByID", err)
			return
		}

		err = issue_service.IsValidReviewRequest(ctx, reviewer, ctx.Doer, action == "attach", issue, nil)
		if err != nil {
			if issues_model.IsErrNotValidReviewRequest(err) {
				log.Warn(
					"UpdatePullReviewRequest: refusing to add invalid review request for %-v to %-v#%d: Error: %v",
					reviewer, issue.Repo, issue.Index,
					err,
				)
				ctx.Status(http.StatusForbidden)
				return
			}
			ctx.ServerError("isValidReviewRequest", err)
			return
		}

		_, err = issue_service.ReviewRequest(ctx, issue, ctx.Doer, reviewer, action == "attach")
		if err != nil {
			ctx.ServerError("ReviewRequest", err)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"ok": true,
	})
}

// SearchIssues searches for issues across the repositories that the user has access to
func SearchIssues(ctx *context.Context) {
	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, err.Error())
		return
	}

	var isClosed util.OptionalBool
	switch ctx.FormString("state") {
	case "closed":
		isClosed = util.OptionalBoolTrue
	case "all":
		isClosed = util.OptionalBoolNone
	default:
		isClosed = util.OptionalBoolFalse
	}

	// find repos user can access (for issue search)
	opts := &repo_model.SearchRepoOptions{
		Private:     false,
		AllPublic:   true,
		TopicOnly:   false,
		Collaborate: util.OptionalBoolNone,
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
		opts.Collaborate = util.OptionalBoolFalse
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

	repoCond := repo_model.SearchRepositoryCondition(opts)
	repoIDs, _, err := repo_model.SearchRepositoryIDs(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchRepositoryIDs", err.Error())
		return
	}

	var issues []*issues_model.Issue
	var filteredCount int64

	keyword := ctx.FormTrim("q")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}
	var issueIDs []int64
	if len(keyword) > 0 && len(repoIDs) > 0 {
		if issueIDs, err = issue_indexer.SearchIssuesByKeyword(ctx, repoIDs, keyword); err != nil {
			ctx.Error(http.StatusInternalServerError, "SearchIssuesByKeyword", err.Error())
			return
		}
	}

	var isPull util.OptionalBool
	switch ctx.FormString("type") {
	case "pulls":
		isPull = util.OptionalBoolTrue
	case "issues":
		isPull = util.OptionalBoolFalse
	default:
		isPull = util.OptionalBoolNone
	}

	labels := ctx.FormTrim("labels")
	var includedLabelNames []string
	if len(labels) > 0 {
		includedLabelNames = strings.Split(labels, ",")
	}

	milestones := ctx.FormTrim("milestones")
	var includedMilestones []string
	if len(milestones) > 0 {
		includedMilestones = strings.Split(milestones, ",")
	}

	projectID := ctx.FormInt64("project")

	// this api is also used in UI,
	// so the default limit is set to fit UI needs
	limit := ctx.FormInt("limit")
	if limit == 0 {
		limit = setting.UI.IssuePagingNum
	} else if limit > setting.API.MaxResponseItems {
		limit = setting.API.MaxResponseItems
	}

	// Only fetch the issues if we either don't have a keyword or the search returned issues
	// This would otherwise return all issues if no issues were found by the search.
	if len(keyword) == 0 || len(issueIDs) > 0 || len(includedLabelNames) > 0 || len(includedMilestones) > 0 {
		issuesOpt := &issues_model.IssuesOptions{
			ListOptions: db.ListOptions{
				Page:     ctx.FormInt("page"),
				PageSize: limit,
			},
			RepoCond:           repoCond,
			IsClosed:           isClosed,
			IssueIDs:           issueIDs,
			IncludedLabelNames: includedLabelNames,
			IncludeMilestones:  includedMilestones,
			ProjectID:          projectID,
			SortType:           "priorityrepo",
			PriorityRepoID:     ctx.FormInt64("priority_repo_id"),
			IsPull:             isPull,
			UpdatedBeforeUnix:  before,
			UpdatedAfterUnix:   since,
		}

		ctxUserID := int64(0)
		if ctx.IsSigned {
			ctxUserID = ctx.Doer.ID
		}

		// Filter for: Created by User, Assigned to User, Mentioning User, Review of User Requested
		if ctx.FormBool("created") {
			issuesOpt.PosterID = ctxUserID
		}
		if ctx.FormBool("assigned") {
			issuesOpt.AssigneeID = ctxUserID
		}
		if ctx.FormBool("mentioned") {
			issuesOpt.MentionedID = ctxUserID
		}
		if ctx.FormBool("review_requested") {
			issuesOpt.ReviewRequestedID = ctxUserID
		}
		if ctx.FormBool("reviewed") {
			issuesOpt.ReviewedID = ctxUserID
		}

		if issues, err = issues_model.Issues(ctx, issuesOpt); err != nil {
			ctx.Error(http.StatusInternalServerError, "Issues", err.Error())
			return
		}

		issuesOpt.ListOptions = db.ListOptions{
			Page: -1,
		}
		if filteredCount, err = issues_model.CountIssues(ctx, issuesOpt); err != nil {
			ctx.Error(http.StatusInternalServerError, "CountIssues", err.Error())
			return
		}
	}

	ctx.SetTotalCountHeader(filteredCount)
	ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, issues))
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

	var isClosed util.OptionalBool
	switch ctx.FormString("state") {
	case "closed":
		isClosed = util.OptionalBoolTrue
	case "all":
		isClosed = util.OptionalBoolNone
	default:
		isClosed = util.OptionalBoolFalse
	}

	var issues []*issues_model.Issue
	var filteredCount int64

	keyword := ctx.FormTrim("q")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}
	var issueIDs []int64
	var labelIDs []int64
	if len(keyword) > 0 {
		issueIDs, err = issue_indexer.SearchIssuesByKeyword(ctx, []int64{ctx.Repo.Repository.ID}, keyword)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
	}

	if splitted := strings.Split(ctx.FormString("labels"), ","); len(splitted) > 0 {
		labelIDs, err = issues_model.GetLabelIDsInRepoByNames(ctx.Repo.Repository.ID, splitted)
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
			mile, err := issues_model.GetMilestoneByRepoIDANDName(ctx.Repo.Repository.ID, part[i])
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

	projectID := ctx.FormInt64("project")

	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
	}

	var isPull util.OptionalBool
	switch ctx.FormString("type") {
	case "pulls":
		isPull = util.OptionalBoolTrue
	case "issues":
		isPull = util.OptionalBoolFalse
	default:
		isPull = util.OptionalBoolNone
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

	// Only fetch the issues if we either don't have a keyword or the search returned issues
	// This would otherwise return all issues if no issues were found by the search.
	if len(keyword) == 0 || len(issueIDs) > 0 || len(labelIDs) > 0 {
		issuesOpt := &issues_model.IssuesOptions{
			ListOptions:       listOptions,
			RepoIDs:           []int64{ctx.Repo.Repository.ID},
			IsClosed:          isClosed,
			IssueIDs:          issueIDs,
			LabelIDs:          labelIDs,
			MilestoneIDs:      mileIDs,
			ProjectID:         projectID,
			IsPull:            isPull,
			UpdatedBeforeUnix: before,
			UpdatedAfterUnix:  since,
			PosterID:          createdByID,
			AssigneeID:        assignedByID,
			MentionedID:       mentionedByID,
		}

		if issues, err = issues_model.Issues(ctx, issuesOpt); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}

		issuesOpt.ListOptions = db.ListOptions{
			Page: -1,
		}
		if filteredCount, err = issues_model.CountIssues(ctx, issuesOpt); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
	}

	ctx.SetTotalCountHeader(filteredCount)
	ctx.JSON(http.StatusOK, convert.ToIssueList(ctx, issues))
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
			if err := issue_service.ChangeStatus(issue, ctx.Doer, "", isClosed); err != nil {
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
	ctx.JSON(http.StatusOK, map[string]any{
		"ok": true,
	})
}

// NewComment create a comment for issue
func NewComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateCommentForm)
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					issue.PosterID,
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(http.StatusForbidden)
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.Doer.IsAdmin {
		ctx.Flash.Error(ctx.Tr("repo.issues.comment_on_locked"))
		ctx.Redirect(issue.Link())
		return
	}

	var attachments []string
	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(issue.Link())
		return
	}

	var comment *issues_model.Comment
	defer func() {
		// Check if issue admin/poster changes the status of issue.
		if (ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) || (ctx.IsSigned && issue.IsPoster(ctx.Doer.ID))) &&
			(form.Status == "reopen" || form.Status == "close") &&
			!(issue.IsPull && issue.PullRequest.HasMerged) {

			// Duplication and conflict check should apply to reopen pull request.
			var pr *issues_model.PullRequest

			if form.Status == "reopen" && issue.IsPull {
				pull := issue.PullRequest
				var err error
				pr, err = issues_model.GetUnmergedPullRequest(ctx, pull.HeadRepoID, pull.BaseRepoID, pull.HeadBranch, pull.BaseBranch, pull.Flow)
				if err != nil {
					if !issues_model.IsErrPullRequestNotExist(err) {
						ctx.Flash.Error(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
						ctx.Redirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, pull.Index))
						return
					}
				}

				// Regenerate patch and test conflict.
				if pr == nil {
					issue.PullRequest.HeadCommitID = ""
					pull_service.AddToTaskQueue(issue.PullRequest)
				}

				// check whether the ref of PR <refs/pulls/pr_index/head> in base repo is consistent with the head commit of head branch in the head repo
				// get head commit of PR
				if pull.Flow == issues_model.PullRequestFlowGithub {
					prHeadRef := pull.GetGitRefName()
					if err := pull.LoadBaseRepo(ctx); err != nil {
						ctx.ServerError("Unable to load base repo", err)
						return
					}
					prHeadCommitID, err := git.GetFullCommitID(ctx, pull.BaseRepo.RepoPath(), prHeadRef)
					if err != nil {
						ctx.ServerError("Get head commit Id of pr fail", err)
						return
					}

					// get head commit of branch in the head repo
					if err := pull.LoadHeadRepo(ctx); err != nil {
						ctx.ServerError("Unable to load head repo", err)
						return
					}
					if ok := git.IsBranchExist(ctx, pull.HeadRepo.RepoPath(), pull.BaseBranch); !ok {
						// todo localize
						ctx.Flash.Error("The origin branch is delete, cannot reopen.")
						ctx.Redirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, pull.Index))
						return
					}
					headBranchRef := pull.GetGitHeadBranchRefName()
					headBranchCommitID, err := git.GetFullCommitID(ctx, pull.HeadRepo.RepoPath(), headBranchRef)
					if err != nil {
						ctx.ServerError("Get head commit Id of head branch fail", err)
						return
					}

					err = pull.LoadIssue(ctx)
					if err != nil {
						ctx.ServerError("load the issue of pull request error", err)
						return
					}

					if prHeadCommitID != headBranchCommitID {
						// force push to base repo
						err := git.Push(ctx, pull.HeadRepo.RepoPath(), git.PushOptions{
							Remote: pull.BaseRepo.RepoPath(),
							Branch: pull.HeadBranch + ":" + prHeadRef,
							Force:  true,
							Env:    repo_module.InternalPushingEnvironment(pull.Issue.Poster, pull.BaseRepo),
						})
						if err != nil {
							ctx.ServerError("force push error", err)
							return
						}
					}
				}
			}

			if pr != nil {
				ctx.Flash.Info(ctx.Tr("repo.pulls.open_unmerged_pull_exists", pr.Index))
			} else {
				isClosed := form.Status == "close"
				if err := issue_service.ChangeStatus(issue, ctx.Doer, "", isClosed); err != nil {
					log.Error("ChangeStatus: %v", err)

					if issues_model.IsErrDependenciesLeft(err) {
						if issue.IsPull {
							ctx.Flash.Error(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
							ctx.Redirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, issue.Index))
						} else {
							ctx.Flash.Error(ctx.Tr("repo.issues.dependency.issue_close_blocked"))
							ctx.Redirect(fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issue.Index))
						}
						return
					}
				} else {
					if err := stopTimerIfAvailable(ctx.Doer, issue); err != nil {
						ctx.ServerError("CreateOrStopIssueStopwatch", err)
						return
					}

					log.Trace("Issue [%d] status changed to closed: %v", issue.ID, issue.IsClosed)
				}
			}

		}

		// Redirect to comment hashtag if there is any actual content.
		typeName := "issues"
		if issue.IsPull {
			typeName = "pulls"
		}
		if comment != nil {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d#%s", ctx.Repo.RepoLink, typeName, issue.Index, comment.HashTag()))
		} else {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d", ctx.Repo.RepoLink, typeName, issue.Index))
		}
	}()

	// Fix #321: Allow empty comments, as long as we have attachments.
	if len(form.Content) == 0 && len(attachments) == 0 {
		return
	}

	comment, err := issue_service.CreateIssueComment(ctx, ctx.Doer, ctx.Repo.Repository, issue, form.Content, attachments)
	if err != nil {
		ctx.ServerError("CreateIssueComment", err)
		return
	}

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
}

// UpdateCommentContent change comment of issue's content
func UpdateCommentContent(ctx *context.Context) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", issues_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	}

	if !comment.Type.HasContentSupport() {
		ctx.Error(http.StatusNoContent)
		return
	}

	oldContent := comment.Content
	comment.Content = ctx.FormString("content")
	if len(comment.Content) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"content": "",
		})
		return
	}
	if err = issue_service.UpdateComment(ctx, comment, ctx.Doer, oldContent); err != nil {
		ctx.ServerError("UpdateComment", err)
		return
	}

	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.ServerError("LoadAttachments", err)
		return
	}

	// when the update request doesn't intend to update attachments (eg: change checkbox state), ignore attachment updates
	if !ctx.FormBool("ignore_attachments") {
		if err := updateAttachments(ctx, comment, ctx.FormStrings("files[]")); err != nil {
			ctx.ServerError("UpdateAttachments", err)
			return
		}
	}

	content, err := markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.FormString("context"), // FIXME: <- IS THIS SAFE ?
		Metas:     ctx.Repo.Repository.ComposeMetas(),
		GitRepo:   ctx.Repo.GitRepo,
		Ctx:       ctx,
	}, comment.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":     content,
		"attachments": attachmentsHTML(ctx, comment.Attachments, comment.Content),
	})
}

// DeleteComment delete comment of issue
func DeleteComment(ctx *context.Context) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", issues_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	} else if !comment.Type.HasContentSupport() {
		ctx.Error(http.StatusNoContent)
		return
	}

	if err = issue_service.DeleteComment(ctx, ctx.Doer, comment); err != nil {
		ctx.ServerError("DeleteComment", err)
		return
	}

	ctx.Status(http.StatusOK)
}

// ChangeIssueReaction create a reaction for issue
func ChangeIssueReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					issue.PosterID,
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(http.StatusForbidden)
		return
	}

	if ctx.HasError() {
		ctx.ServerError("ChangeIssueReaction", errors.New(ctx.GetErrMsg()))
		return
	}

	switch ctx.Params(":action") {
	case "react":
		reaction, err := issues_model.CreateIssueReaction(ctx.Doer.ID, issue.ID, form.Content)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) {
				ctx.ServerError("ChangeIssueReaction", err)
				return
			}
			log.Info("CreateIssueReaction: %s", err)
			break
		}
		// Reload new reactions
		issue.Reactions = nil
		if err = issue.LoadAttributes(ctx); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, reaction.ID)
	case "unreact":
		if err := issues_model.DeleteIssueReaction(ctx.Doer.ID, issue.ID, form.Content); err != nil {
			ctx.ServerError("DeleteIssueReaction", err)
			return
		}

		// Reload new reactions
		issue.Reactions = nil
		if err := issue.LoadAttributes(ctx); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue removed: %d/%d", ctx.Repo.Repository.ID, issue.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.Params(":action")), nil)
		return
	}

	if len(issue.Reactions) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.RenderToString(tplReactions, map[string]any{
		"ctxData":   ctx.Data,
		"ActionURL": fmt.Sprintf("%s/issues/%d/reactions", ctx.Repo.RepoLink, issue.Index),
		"Reactions": issue.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeIssueReaction.HTMLString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"html": html,
	})
}

// ChangeCommentReaction create a reaction for comment
func ChangeCommentReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	comment, err := issues_model.GetCommentByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", issues_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if comment.Issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					comment.Issue.PosterID,
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(http.StatusForbidden)
		return
	}

	if !comment.Type.HasContentSupport() {
		ctx.Error(http.StatusNoContent)
		return
	}

	switch ctx.Params(":action") {
	case "react":
		reaction, err := issues_model.CreateCommentReaction(ctx.Doer.ID, comment.Issue.ID, comment.ID, form.Content)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) {
				ctx.ServerError("ChangeIssueReaction", err)
				return
			}
			log.Info("CreateCommentReaction: %s", err)
			break
		}
		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment created: %d/%d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID, reaction.ID)
	case "unreact":
		if err := issues_model.DeleteCommentReaction(ctx.Doer.ID, comment.Issue.ID, comment.ID, form.Content); err != nil {
			ctx.ServerError("DeleteCommentReaction", err)
			return
		}

		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment removed: %d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.Params(":action")), nil)
		return
	}

	if len(comment.Reactions) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.RenderToString(tplReactions, map[string]any{
		"ctxData":   ctx.Data,
		"ActionURL": fmt.Sprintf("%s/comments/%d/reactions", ctx.Repo.RepoLink, comment.ID),
		"Reactions": comment.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeCommentReaction.HTMLString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"html": html,
	})
}

func addParticipant(poster *user_model.User, participants []*user_model.User) []*user_model.User {
	for _, part := range participants {
		if poster.ID == part.ID {
			return participants
		}
	}
	return append(participants, poster)
}

func filterXRefComments(ctx *context.Context, issue *issues_model.Issue) error {
	// Remove comments that the user has no permissions to see
	for i := 0; i < len(issue.Comments); {
		c := issue.Comments[i]
		if issues_model.CommentTypeIsRef(c.Type) && c.RefRepoID != issue.RepoID && c.RefRepoID != 0 {
			var err error
			// Set RefRepo for description in template
			c.RefRepo, err = repo_model.GetRepositoryByID(ctx, c.RefRepoID)
			if err != nil {
				return err
			}
			perm, err := access_model.GetUserRepoPermission(ctx, c.RefRepo, ctx.Doer)
			if err != nil {
				return err
			}
			if !perm.CanReadIssuesOrPulls(c.RefIsPull) {
				issue.Comments = append(issue.Comments[:i], issue.Comments[i+1:]...)
				continue
			}
		}
		i++
	}
	return nil
}

// GetIssueAttachments returns attachments for the issue
func GetIssueAttachments(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}
	attachments := make([]*api.Attachment, len(issue.Attachments))
	for i := 0; i < len(issue.Attachments); i++ {
		attachments[i] = convert.ToAttachment(ctx.Repo.Repository, issue.Attachments[i])
	}
	ctx.JSON(http.StatusOK, attachments)
}

// GetCommentAttachments returns attachments for the comment
func GetCommentAttachments(ctx *context.Context) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", issues_model.ErrCommentNotExist{})
		return
	}

	if !ctx.Repo.Permission.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		ctx.NotFound("CanReadIssuesOrPulls", issues_model.ErrCommentNotExist{})
		return
	}

	if !comment.Type.HasAttachmentSupport() {
		ctx.ServerError("GetCommentAttachments", fmt.Errorf("comment type %v does not support attachments", comment.Type))
		return
	}

	attachments := make([]*api.Attachment, 0)
	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.ServerError("LoadAttachments", err)
		return
	}
	for i := 0; i < len(comment.Attachments); i++ {
		attachments = append(attachments, convert.ToAttachment(ctx.Repo.Repository, comment.Attachments[i]))
	}
	ctx.JSON(http.StatusOK, attachments)
}

func updateAttachments(ctx *context.Context, item any, files []string) error {
	var attachments []*repo_model.Attachment
	switch content := item.(type) {
	case *issues_model.Issue:
		attachments = content.Attachments
	case *issues_model.Comment:
		attachments = content.Attachments
	default:
		return fmt.Errorf("unknown Type: %T", content)
	}
	for i := 0; i < len(attachments); i++ {
		if util.SliceContainsString(files, attachments[i].UUID) {
			continue
		}
		if err := repo_model.DeleteAttachment(attachments[i], true); err != nil {
			return err
		}
	}
	var err error
	if len(files) > 0 {
		switch content := item.(type) {
		case *issues_model.Issue:
			err = issues_model.UpdateIssueAttachments(content.ID, files)
		case *issues_model.Comment:
			err = content.UpdateAttachments(files)
		default:
			return fmt.Errorf("unknown Type: %T", content)
		}
		if err != nil {
			return err
		}
	}
	switch content := item.(type) {
	case *issues_model.Issue:
		content.Attachments, err = repo_model.GetAttachmentsByIssueID(ctx, content.ID)
	case *issues_model.Comment:
		content.Attachments, err = repo_model.GetAttachmentsByCommentID(ctx, content.ID)
	default:
		return fmt.Errorf("unknown Type: %T", content)
	}
	return err
}

func attachmentsHTML(ctx *context.Context, attachments []*repo_model.Attachment, content string) string {
	attachHTML, err := ctx.RenderToString(tplAttachment, map[string]any{
		"ctxData":     ctx.Data,
		"Attachments": attachments,
		"Content":     content,
	})
	if err != nil {
		ctx.ServerError("attachmentsHTML.HTMLString", err)
		return ""
	}
	return attachHTML
}

// combineLabelComments combine the nearby label comments as one.
func combineLabelComments(issue *issues_model.Issue) {
	var prev, cur *issues_model.Comment
	for i := 0; i < len(issue.Comments); i++ {
		cur = issue.Comments[i]
		if i > 0 {
			prev = issue.Comments[i-1]
		}
		if i == 0 || cur.Type != issues_model.CommentTypeLabel ||
			(prev != nil && prev.PosterID != cur.PosterID) ||
			(prev != nil && cur.CreatedUnix-prev.CreatedUnix >= 60) {
			if cur.Type == issues_model.CommentTypeLabel && cur.Label != nil {
				if cur.Content != "1" {
					cur.RemovedLabels = append(cur.RemovedLabels, cur.Label)
				} else {
					cur.AddedLabels = append(cur.AddedLabels, cur.Label)
				}
			}
			continue
		}

		if cur.Label != nil { // now cur MUST be label comment
			if prev.Type == issues_model.CommentTypeLabel { // we can combine them only prev is a label comment
				if cur.Content != "1" {
					// remove labels from the AddedLabels list if the label that was removed is already
					// in this list, and if it's not in this list, add the label to RemovedLabels
					addedAndRemoved := false
					for i, label := range prev.AddedLabels {
						if cur.Label.ID == label.ID {
							prev.AddedLabels = append(prev.AddedLabels[:i], prev.AddedLabels[i+1:]...)
							addedAndRemoved = true
							break
						}
					}
					if !addedAndRemoved {
						prev.RemovedLabels = append(prev.RemovedLabels, cur.Label)
					}
				} else {
					// remove labels from the RemovedLabels list if the label that was added is already
					// in this list, and if it's not in this list, add the label to AddedLabels
					removedAndAdded := false
					for i, label := range prev.RemovedLabels {
						if cur.Label.ID == label.ID {
							prev.RemovedLabels = append(prev.RemovedLabels[:i], prev.RemovedLabels[i+1:]...)
							removedAndAdded = true
							break
						}
					}
					if !removedAndAdded {
						prev.AddedLabels = append(prev.AddedLabels, cur.Label)
					}
				}
				prev.CreatedUnix = cur.CreatedUnix
				// remove the current comment since it has been combined to prev comment
				issue.Comments = append(issue.Comments[:i], issue.Comments[i+1:]...)
				i--
			} else { // if prev is not a label comment, start a new group
				if cur.Content != "1" {
					cur.RemovedLabels = append(cur.RemovedLabels, cur.Label)
				} else {
					cur.AddedLabels = append(cur.AddedLabels, cur.Label)
				}
			}
		}
	}
}

// get all teams that current user can mention
func handleTeamMentions(ctx *context.Context) {
	if ctx.Doer == nil || !ctx.Repo.Owner.IsOrganization() {
		return
	}

	var isAdmin bool
	var err error
	var teams []*organization.Team
	org := organization.OrgFromUser(ctx.Repo.Owner)
	// Admin has super access.
	if ctx.Doer.IsAdmin {
		isAdmin = true
	} else {
		isAdmin, err = org.IsOwnedBy(ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("IsOwnedBy", err)
			return
		}
	}

	if isAdmin {
		teams, err = org.LoadTeams()
		if err != nil {
			ctx.ServerError("LoadTeams", err)
			return
		}
	} else {
		teams, err = org.GetUserTeams(ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("GetUserTeams", err)
			return
		}
	}

	ctx.Data["MentionableTeams"] = teams
	ctx.Data["MentionableTeamsOrg"] = ctx.Repo.Owner.Name
	ctx.Data["MentionableTeamsOrgAvatar"] = ctx.Repo.Owner.AvatarLink(ctx)
}

type userSearchInfo struct {
	UserID     int64  `json:"user_id"`
	UserName   string `json:"username"`
	AvatarLink string `json:"avatar_link"`
	FullName   string `json:"full_name"`
}

type userSearchResponse struct {
	Results []*userSearchInfo `json:"results"`
}

// IssuePosters get posters for current repo's issues/pull requests
func IssuePosters(ctx *context.Context) {
	issuePosters(ctx, false)
}

func PullPosters(ctx *context.Context) {
	issuePosters(ctx, true)
}

func issuePosters(ctx *context.Context, isPullList bool) {
	repo := ctx.Repo.Repository
	search := strings.TrimSpace(ctx.FormString("q"))
	posters, err := repo_model.GetIssuePostersWithSearch(ctx, repo, isPullList, search, setting.UI.DefaultShowFullName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	if search == "" && ctx.Doer != nil {
		// the returned posters slice only contains limited number of users,
		// to make the current user (doer) can quickly filter their own issues, always add doer to the posters slice
		if !util.SliceContainsFunc(posters, func(user *user_model.User) bool { return user.ID == ctx.Doer.ID }) {
			posters = append(posters, ctx.Doer)
		}
	}

	posters = makeSelfOnTop(ctx, posters)

	resp := &userSearchResponse{}
	resp.Results = make([]*userSearchInfo, len(posters))
	for i, user := range posters {
		resp.Results[i] = &userSearchInfo{UserID: user.ID, UserName: user.Name, AvatarLink: user.AvatarLink(ctx)}
		if setting.UI.DefaultShowFullName {
			resp.Results[i].FullName = user.FullName
		}
	}
	ctx.JSON(http.StatusOK, resp)
}
