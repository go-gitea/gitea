// Copyright 2017 The Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"container/list"
	"log"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"github.com/google/git-appraise/repository"
	"github.com/google/git-appraise/review"
)

const (
	tplReviews base.TplName = "repo/review/list"
)

// MustAllowReviews check if repository enable reviews
func MustAllowReviews(ctx *context.Context) {
	if !ctx.Repo.Repository.AllowsReviews() {
		ctx.Handle(404, "MustAllowReviews", nil)
		return
	}
}

// Reviews render issues page
func Reviews(ctx *context.Context) {
	repo, err := repository.NewGitRepo(ctx.Repo.GitRepo.Path)
	if err != nil {
		ctx.Handle(500, "OpenGitRepository", err)
	}
	total := review.ListAll(repo)

	viewType := ctx.Query("type")
	sortType := ctx.Query("sort")
	selectLabels := ctx.Query("labels")
	milestoneID := ctx.QueryInt64("milestone")
	assigneeID := ctx.QueryInt64("assignee")
	isShowClosed := ctx.Query("state") == "closed"

	issues := []*models.Issue{}
	stat := models.IssueStats{OpenCount: 0, ClosedCount: 0, AllCount: 0}

	for _, review := range total {
		details, _ := review.Details()
		notes, _ := details.GetAnalysesNotes()
		for _, note := range notes {
			log.Print(note)
		}

		timestamp, _ := strconv.Atoi(review.Request.Timestamp)
		time := time.Unix(int64(timestamp), 0)

		closed := false
		if review.Submitted || review.Request.TargetRef == "" {
			closed = true
			stat.ClosedCount++
		} else {
			stat.OpenCount++
		}
		readed := false
		if (review.Resolved != nil && *review.Resolved) || review.Request.TargetRef == "" {
			readed = true
		}

		user, err := models.GetUserByEmail(review.Request.Requester)
		if err != nil {
			user = &models.User{Name: review.Request.Requester}
		}

		issue := models.Issue{
			Revision:    review.Revision,
			Title:       strings.Split(review.Request.Description, "\n\n")[0],
			Poster:      user,
			Created:     time,
			CreatedUnix: int64(timestamp),
			Labels:      []*models.Label{},
			IsClosed:    closed,
			IsRead:      readed,
			IsReview:    true,
		}

		if closed && isShowClosed {
			issues = append(issues, &issue)
		}
		if !closed && !isShowClosed {
			issues = append(issues, &issue)
		}
		stat.AllCount++
	}

	ctx.Data["Title"] = ctx.Tr("repo.reviews")
	ctx.Data["PageIsReviewList"] = true
	// ctx.Data["Page"] = 0
	ctx.Data["Issues"] = issues
	ctx.Data["Milestones"], err = models.GetMilestonesByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Handle(500, "GetAllRepoMilestones", err)
		return
	}

	// Get assignees.
	ctx.Data["Assignees"], err = ctx.Repo.Repository.GetAssignees()
	if err != nil {
		ctx.Handle(500, "GetAssignees", err)
		return
	}
	ctx.Data["SelectLabels"] = com.StrTo(selectLabels).MustInt64()
	ctx.Data["IssueStats"] = stat
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["IsShowClosed"] = isShowClosed
	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}
	ctx.HTML(200, tplIssues)
}

func prepareReviewInfo(ctx *context.Context) (*review.Summary, *review.Review, *models.Issue) {
	ctx.Data["PageIsReviewList"] = true

	repo, err := repository.NewGitRepo(ctx.Repo.GitRepo.Path)
	if err != nil {
		ctx.Handle(500, "OpenGitRepository", err)
	}
	reviewSummary, err := review.GetSummary(repo, ctx.Params(":index"))
	if err != nil {
		ctx.Handle(500, "GetReviewSummary", err)
	}
	reviewDetails, err := reviewSummary.Details()
	if err != nil {
		ctx.Handle(500, "GetReview", err)
	}

	issueUser, _ := models.GetUserByEmail(reviewDetails.Request.Requester)
	issueCreatedTimestamp, _ := strconv.Atoi(reviewDetails.Request.Timestamp)
	issueCreated := time.Unix(int64(issueCreatedTimestamp), 0)

	closed := false
	if reviewDetails.Submitted || reviewDetails.Request.TargetRef == "" {
		closed = true
	}
	readed := false
	if (reviewDetails.Resolved != nil && *reviewDetails.Resolved) || reviewDetails.Request.TargetRef == "" {
		readed = true
	}

	issue := &models.Issue{
		ID:          0,
		Index:       0,
		Revision:    reviewDetails.Revision,
		Title:       strings.Split(reviewDetails.Request.Description, "\n\n")[0],
		Content:     reviewDetails.Request.Description,
		Poster:      issueUser,
		Created:     issueCreated,
		Labels:      []*models.Label{},
		IsClosed:    closed,
		IsRead:      readed,
		IsReview:    true,
		NumComments: len(reviewDetails.Comments),
		Comments:    []*models.Comment{},
		PullRequest: &models.PullRequest{
			HasMerged: reviewDetails.Submitted,
		},
	}

	issue.RenderedContent = string(markdown.Render([]byte(issue.Content), ctx.Repo.RepoLink,
		ctx.Repo.Repository.ComposeMetas()))

	ctx.Data["HasMerged"] = reviewDetails.Submitted

	reviewCommits, err := repo.ListCommitsBetween(reviewDetails.Request.TargetRef, reviewDetails.Request.ReviewRef)
	if err != nil {
		ctx.Handle(500, "ListCommitsBetween", err)
	}

	startCommitID, err := reviewDetails.GetBaseCommit()
	if err != nil {
		ctx.Handle(500, "GetBaseCommit", err)
	}

	ctx.Data["HeadTarget"] = reviewDetails.Request.ReviewRef
	ctx.Data["BaseTarget"] = reviewDetails.Request.TargetRef
	ctx.Data["Title"] = strings.Split(reviewDetails.Request.Description, "\n\n")[0]
	ctx.Data["Issue"] = issue
	ctx.Data["IsIssueOwner"] = ctx.Repo.IsWriter() || (ctx.IsSigned && issue.IsPoster(ctx.User.ID))

	ctx.Data["NumCommits"], err = ctx.Repo.GitRepo.CommitsCountBetween(startCommitID, reviewCommits[len(reviewCommits)-1])
	if err != nil {
		ctx.Handle(500, "Repo.GitRepo.CommitsCountBetween", err)
	}
	ctx.Data["NumFiles"], err = ctx.Repo.GitRepo.FilesCountBetween(startCommitID, reviewCommits[len(reviewCommits)-1])
	if err != nil {
		ctx.Handle(500, "Repo.GitRepo.FilesCountBetween", err)
	}

	return reviewSummary, reviewDetails, issue
}

// ViewReview render issue view page
func ViewReview(ctx *context.Context) {
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireDropzone"] = true

	_, reviewDetails, issue := prepareReviewInfo(ctx)

	MustAllowReviews(ctx)
	if ctx.Written() {
		return
	}

	ctx.Data["PageIsPullConversation"] = true
	//
	// repo := ctx.Repo.Repository
	//
	// if issue.PullRequest.HasMerged {
	// 	ctx.Data["DisableStatusChange"] = issue.PullRequest.HasMerged
	// 	PrepareMergedViewPullInfo(ctx, issue)
	// } else {
	// 	PrepareViewPullInfo(ctx, issue)
	// }
	// if ctx.Written() {
	// 	return
	// }

	// // Metas.
	// // Check labels.
	// labelIDMark := make(map[int64]bool)
	// for i := range issue.Labels {
	// 	labelIDMark[issue.Labels[i].ID] = true
	// }
	// labels, err := models.GetLabelsByRepoID(repo.ID, "")
	// if err != nil {
	// 	ctx.Handle(500, "GetLabelsByRepoID", err)
	// 	return
	// }
	// hasSelected := false
	// for i := range labels {
	// 	if labelIDMark[labels[i].ID] {
	// 		labels[i].IsChecked = true
	// 		hasSelected = true
	// 	}
	// }
	// ctx.Data["HasSelectedLabel"] = hasSelected
	// ctx.Data["Labels"] = labels
	//
	// // Check milestone and assignee.
	// if ctx.Repo.IsWriter() {
	// 	RetrieveRepoMilestonesAndAssignees(ctx, repo)
	// 	if ctx.Written() {
	// 		return
	// 	}
	// }

	// if ctx.IsSigned {
	// 	// Update issue-user.
	// 	if err = issue.ReadBy(ctx.User.ID); err != nil {
	// 		ctx.Handle(500, "ReadBy", err)
	// 		return
	// 	}
	// }
	//
	// var (
	// 	tag          models.CommentTag
	// 	ok           bool
	// 	marked       = make(map[int64]models.CommentTag)
	// var comment *models.Comment
	participants := make([]*models.User, 1, 10)
	// )
	//
	// // Render comments and and fetch participants.
	participants[0] = issue.Poster
	for _, c := range reviewDetails.Comments {
		log.Print(c.Comment, c.Hash)
		reviewComment := models.Comment{}

		commentUser, _ := models.GetUserByEmail(c.Comment.Author)
		reviewComment.Poster = commentUser

		reviewComment.Content = c.Comment.Description

		reviewComment.RenderedContent = string(markdown.Render([]byte(reviewComment.Content), ctx.Repo.RepoLink,
			ctx.Repo.Repository.ComposeMetas()))

		commentCreatedTimestamp, _ := strconv.Atoi(reviewDetails.Request.Timestamp)
		commentCreated := time.Unix(int64(commentCreatedTimestamp), 0)
		reviewComment.Created = commentCreated

		//
		// 		// Check tag.
		// 		tag, ok = marked[comment.PosterID]
		// 		if ok {
		// 			comment.ShowTag = tag
		// 			continue
		// 		}
		//
		if ctx.Repo.Repository.IsOwnedBy(reviewComment.Poster.ID) ||
			(ctx.Repo.Repository.Owner.IsOrganization() && ctx.Repo.Repository.Owner.IsOwnedBy(reviewComment.Poster.ID)) {
			reviewComment.ShowTag = models.CommentTagOwner
		} else if reviewComment.Poster.IsWriterOfRepo(ctx.Repo.Repository) {
			reviewComment.ShowTag = models.CommentTagWriter
		} else if reviewComment.Poster.ID == issue.Poster.ID {
			reviewComment.ShowTag = models.CommentTagPoster
		}
		//
		// 		marked[comment.PosterID] = comment.ShowTag
		//
		isAdded := false
		for j := range participants {
			if reviewComment.Poster.ID == participants[j].ID {
				isAdded = true
				break
			}
		}
		if !isAdded && !issue.IsPoster(reviewComment.Poster.ID) {
			participants = append(participants, reviewComment.Poster)
		}
		// }
		//
		// pull := issue.PullRequest
		// canDelete := false

		// if ctx.IsSigned && pull.HeadBranch != "master" {
		//
		// 	if err := pull.GetHeadRepo(); err != nil {
		// 		log.Error(4, "GetHeadRepo: %v", err)
		// 	} else if ctx.User.IsWriterOfRepo(pull.HeadRepo) {
		// 		canDelete = true
		// 		deleteBranchURL := pull.HeadRepo.Link() + "/branches/" + pull.HeadBranch + "/delete"
		// 		ctx.Data["DeleteBranchLink"] = fmt.Sprintf("%s?commit=%s&redirect_to=%s", deleteBranchURL, pull.MergedCommitID, ctx.Data["Link"])
		//
		// 	}
		issue.Comments = append(issue.Comments, &reviewComment)
	}

	// ctx.Data["IsPullBranchDeletable"] = canDelete && git.IsBranchExist(pull.HeadRepo.RepoPath(), pull.HeadBranch)
	//
	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + ctx.Data["Link"].(string)
	ctx.HTML(200, tplIssueView)
}

// ViewReviewCommits show commits for a pull request
func ViewReviewCommits(ctx *context.Context) {
	ctx.Data["PageIsPullCommits"] = true

	repo, err := repository.NewGitRepo(ctx.Repo.GitRepo.Path)
	if err != nil {
		ctx.Handle(500, "OpenGitRepository", err)
	}

	_, reviewDetails, _ := prepareReviewInfo(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name

	reviewCommits, err := repo.ListCommitsBetween(reviewDetails.Request.TargetRef, reviewDetails.Request.ReviewRef)
	if err != nil {
		ctx.Handle(500, "GetReviewCommits", err)
	}

	// Reverse list
	for i, j := 0, len(reviewCommits)-1; i < j; i, j = i+1, j-1 {
		reviewCommits[i], reviewCommits[j] = reviewCommits[j], reviewCommits[i]
	}

	commits := list.New()
	for _, commitHash := range reviewCommits {
		commit, err := ctx.Repo.GitRepo.GetCommit(commitHash)
		if err != nil {
			ctx.Handle(500, "GetReviewCommit", err)
		}
		commits.PushBack(commit)
	}

	commits = models.ValidateCommitsWithEmails(commits)
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = commits.Len()

	ctx.HTML(200, tplPullCommits)
}

// ViewReviewFiles render pull request changed files list page
func ViewReviewFiles(ctx *context.Context) {
	ctx.Data["PageIsPullFiles"] = true

	_, reviewDetails, _ := prepareReviewInfo(ctx)

	if ctx.Written() {
		return
	}

	repo, err := repository.NewGitRepo(ctx.Repo.GitRepo.Path)
	if err != nil {
		ctx.Handle(500, "OpenGitRepository", err)
	}
	reviewCommits, err := repo.ListCommitsBetween(reviewDetails.Request.TargetRef, reviewDetails.Request.ReviewRef)
	if err != nil {
		ctx.Handle(500, "ListCommitsBetween", err)
	}

	startCommitID, err := reviewDetails.GetBaseCommit()
	if err != nil {
		ctx.Handle(500, "GetBaseCommit", err)
	}

	endCommitID := reviewCommits[len(reviewCommits)-1]
	diff, err := models.GetDiffRange(ctx.Repo.GitRepo.Path,
		startCommitID, endCommitID, setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles)
	if err != nil {
		ctx.Handle(500, "GetDiffRange", err)
		return
	}
	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diff.NumFiles() == 0

	commit, err := ctx.Repo.GitRepo.GetCommit(endCommitID)
	if err != nil {
		ctx.Handle(500, "GetCommit", err)
		return
	}

	headTarget := path.Join(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["IsImageFile"] = commit.IsImageFile
	ctx.Data["SourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", endCommitID)
	ctx.Data["BeforeSourcePath"] = setting.AppSubURL + "/" + path.Join(headTarget, "src", startCommitID)
	ctx.Data["RawPath"] = setting.AppSubURL + "/" + path.Join(headTarget, "raw", endCommitID)
	ctx.Data["RequireHighlightJS"] = true

	ctx.HTML(200, tplPullFiles)
}
