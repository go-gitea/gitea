// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"
	notify_service "code.gitea.io/gitea/services/notify"
)

var notEnoughLines = regexp.MustCompile(`fatal: file .* has only \d+ lines?`)

// ErrDismissRequestOnClosedPR represents an error when an user tries to dismiss a review associated to a closed or merged PR.
type ErrDismissRequestOnClosedPR struct{}

// IsErrDismissRequestOnClosedPR checks if an error is an ErrDismissRequestOnClosedPR.
func IsErrDismissRequestOnClosedPR(err error) bool {
	_, ok := err.(ErrDismissRequestOnClosedPR)
	return ok
}

func (err ErrDismissRequestOnClosedPR) Error() string {
	return "can't dismiss a review associated to a closed or merged PR"
}

func (err ErrDismissRequestOnClosedPR) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrSubmitReviewOnClosedPR represents an error when an user tries to submit an approve or reject review associated to a closed or merged PR.
var ErrSubmitReviewOnClosedPR = errors.New("can't submit review for a closed or merged PR")

// checkInvalidation checks if the line of code comment got changed by another commit.
// If the line got changed the comment is going to be invalidated.
func checkInvalidation(ctx context.Context, c *issues_model.Comment, repo *git.Repository, branch string) error {
	// FIXME differentiate between previous and proposed line
	commit, err := repo.LineBlame(branch, repo.Path, c.TreePath, uint(c.UnsignedLine()))
	if err != nil && (strings.Contains(err.Error(), "fatal: no such path") || notEnoughLines.MatchString(err.Error())) {
		c.Invalidated = true
		return issues_model.UpdateCommentInvalidate(ctx, c)
	}
	if err != nil {
		return err
	}
	if c.CommitSHA != "" && c.CommitSHA != commit.ID.String() {
		c.Invalidated = true
		return issues_model.UpdateCommentInvalidate(ctx, c)
	}
	return nil
}

// InvalidateCodeComments will lookup the prs for code comments which got invalidated by change
func InvalidateCodeComments(ctx context.Context, prs issues_model.PullRequestList, doer *user_model.User, repo *git.Repository, branch string) error {
	if len(prs) == 0 {
		return nil
	}
	issueIDs := prs.GetIssueIDs()

	codeComments, err := db.Find[issues_model.Comment](ctx, issues_model.FindCommentsOptions{
		ListOptions: db.ListOptionsAll,
		Type:        issues_model.CommentTypeCode,
		Invalidated: optional.Some(false),
		IssueIDs:    issueIDs,
	})
	if err != nil {
		return fmt.Errorf("find code comments: %v", err)
	}
	for _, comment := range codeComments {
		if err := checkInvalidation(ctx, comment, repo, branch); err != nil {
			return err
		}
	}
	return nil
}

// CreateCodeComment creates a comment on the code line
func CreateCodeComment(ctx context.Context, doer *user_model.User, gitRepo *git.Repository,
	issue *issues_model.Issue, line int64, beforeCommitID, afterCommitID, content, treePath string,
	pendingReview bool, replyReviewID int64, attachments []string,
) (*issues_model.Comment, error) {
	var (
		existsReview bool
		err          error
	)

	if gitRepo == nil {
		var closer io.Closer
		gitRepo, closer, err = gitrepo.RepositoryFromContextOrOpen(ctx, issue.Repo)
		if err != nil {
			return nil, fmt.Errorf("RepositoryFromContextOrOpen: %w", err)
		}
		defer closer.Close()
	}

	if err := issue.LoadPullRequest(ctx); err != nil {
		return nil, fmt.Errorf("LoadPullRequest: %w", err)
	}

	headCommitID, err := gitRepo.GetRefCommitID(issue.PullRequest.GetGitHeadRefName())
	if err != nil {
		return nil, fmt.Errorf("GetRefCommitID[%s]: %w", issue.PullRequest.GetGitHeadRefName(), err)
	}
	prCommitIDs, err := git.CommitIDsBetween(ctx, gitRepo.Path, issue.PullRequest.MergeBase, headCommitID)
	if err != nil {
		return nil, fmt.Errorf("CommitIDsBetween[%s, %s]: %w", beforeCommitID, afterCommitID, err)
	}

	if beforeCommitID == "" || beforeCommitID == issue.PullRequest.MergeBase {
		beforeCommitID = issue.PullRequest.MergeBase
	} else if !slices.Contains(prCommitIDs, beforeCommitID) { // beforeCommitID must be one of the pull request commits
		return nil, fmt.Errorf("beforeCommitID[%s] is not a valid pull request commit", beforeCommitID)
	}

	if afterCommitID == "" || afterCommitID == headCommitID {
		afterCommitID = headCommitID
	} else if !slices.Contains(prCommitIDs, afterCommitID) { // afterCommitID must be one of the pull request commits
		return nil, fmt.Errorf("afterCommitID[%s] is not a valid pull request commit", afterCommitID)
	}

	// CreateCodeComment() is used for:
	// - Single comments
	// - Comments that are part of a review
	// - Comments that reply to an existing review

	if !pendingReview && replyReviewID != 0 {
		// It's not part of a review; maybe a reply to a review comment or a single comment.
		// Check if there are reviews for that line already; if there are, this is a reply
		if existsReview, err = issues_model.ReviewExists(ctx, issue, treePath, line); err != nil {
			return nil, err
		}
	}

	// Comments that are replies don't require a review header to show up in the issue view
	if !pendingReview && existsReview {
		if err = issue.LoadRepo(ctx); err != nil {
			return nil, err
		}

		comment, err := createCodeComment(ctx,
			doer,
			issue.Repo,
			gitRepo,
			issue,
			beforeCommitID,
			afterCommitID,
			content,
			treePath,
			line,
			replyReviewID,
			attachments,
		)
		if err != nil {
			return nil, err
		}

		mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comment.Content)
		if err != nil {
			return nil, err
		}

		notify_service.CreateIssueComment(ctx, doer, issue.Repo, issue, comment, mentions)

		return comment, nil
	}

	review, err := issues_model.GetCurrentReview(ctx, doer, issue)
	if err != nil {
		if !issues_model.IsErrReviewNotExist(err) {
			return nil, err
		}

		if review, err = issues_model.CreateReview(ctx, issues_model.CreateReviewOptions{
			Type:     issues_model.ReviewTypePending,
			Reviewer: doer,
			Issue:    issue,
			Official: false,
			CommitID: afterCommitID,
		}); err != nil {
			return nil, err
		}
	}

	comment, err := createCodeComment(ctx,
		doer,
		issue.Repo,
		gitRepo,
		issue,
		beforeCommitID,
		afterCommitID,
		content,
		treePath,
		line,
		review.ID,
		attachments,
	)
	if err != nil {
		return nil, err
	}

	if !pendingReview && !existsReview {
		// Submit the review we've just created so the comment shows up in the issue view
		if _, _, err = SubmitReview(ctx, doer, gitRepo, issue, issues_model.ReviewTypeComment, "", afterCommitID, nil); err != nil {
			return nil, err
		}
	}

	// NOTICE: if it's a pending review the notifications will not be fired until user submit review.

	return comment, nil
}

func patchCacheKey(issueID int64, beforeCommitID, afterCommitID, treePath string, line int64) string {
	// The key is used to cache the patch for a specific line in a review comment.
	// It is composed of the issue ID, commit IDs, tree path and line number.
	return fmt.Sprintf("review-line-patch-%d-%s-%s-%s-%d", issueID, beforeCommitID, afterCommitID, treePath, line)
}

// createCodeComment creates a plain code comment at the specified line / path
func createCodeComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository,
	issue *issues_model.Issue, beforeCommitID, afterCommitID, content, treePath string, line, reviewID int64, attachments []string,
) (*issues_model.Comment, error) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		return nil, fmt.Errorf("LoadPullRequest: %w", err)
	}
	pr := issue.PullRequest
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, fmt.Errorf("LoadBaseRepo: %w", err)
	}

	patch, err := cache.GetString(patchCacheKey(issue.ID, beforeCommitID, afterCommitID, treePath, line), func() (string, error) {
		reader, writer := io.Pipe()
		defer func() {
			_ = reader.Close()
			_ = writer.Close()
		}()
		go func() {
			if err := git.GetRepoRawDiffForFile(gitRepo, beforeCommitID, afterCommitID, git.RawDiffNormal, treePath, writer); err != nil {
				_ = writer.CloseWithError(fmt.Errorf("GetRawDiffForLine[%s, %s, %s, %s]: %w", gitRepo.Path, beforeCommitID, afterCommitID, treePath, err))
				return
			}
			_ = writer.Close()
		}()

		return git.CutDiffAroundLine(reader, int64((&issues_model.Comment{Line: line}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines)
	})
	if err != nil {
		return nil, fmt.Errorf("GetPatch failed: %w", err)
	}

	return db.WithTx2(ctx, func(ctx context.Context) (*issues_model.Comment, error) {
		comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
			Type:           issues_model.CommentTypeCode,
			Doer:           doer,
			Repo:           repo,
			Issue:          issue,
			Content:        content,
			LineNum:        line,
			TreePath:       treePath,
			BeforeCommitID: beforeCommitID,
			CommitSHA:      afterCommitID,
			ReviewID:       reviewID,
			Patch:          patch,
			Invalidated:    false,
			Attachments:    attachments,
		})
		if err != nil {
			return nil, err
		}

		// The line commit ID Must be referenced in the git repository, because the branch maybe rebased or force-pushed.
		// If the review commit is GC, the position can not be calculated dynamically.
		if err := git.UpdateRef(ctx, pr.BaseRepo.RepoPath(), issues_model.GetCodeCommentRefName(pr.Index, comment.ID, "before"), beforeCommitID); err != nil {
			log.Error("Unable to update ref before_commitid in base repository for PR[%d] Error: %v", pr.ID, err)
			return nil, err
		}
		if err := git.UpdateRef(ctx, pr.BaseRepo.RepoPath(), issues_model.GetCodeCommentRefName(pr.Index, comment.ID, "after"), afterCommitID); err != nil {
			log.Error("Unable to update ref after_commitid in base repository for PR[%d] Error: %v", pr.ID, err)
			return nil, err
		}

		return comment, nil
	})
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(ctx context.Context, doer *user_model.User, gitRepo *git.Repository, issue *issues_model.Issue, reviewType issues_model.ReviewType, content, commitID string, attachmentUUIDs []string) (*issues_model.Review, *issues_model.Comment, error) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		return nil, nil, err
	}

	pr := issue.PullRequest
	var stale bool
	if reviewType != issues_model.ReviewTypeApprove && reviewType != issues_model.ReviewTypeReject {
		stale = false
	} else {
		if issue.IsClosed {
			return nil, nil, ErrSubmitReviewOnClosedPR
		}

		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitHeadRefName())
		if err != nil {
			return nil, nil, err
		}

		if headCommitID == commitID {
			stale = false
		} else {
			stale, err = checkIfPRContentChanged(ctx, pr, commitID, headCommitID)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	review, comm, err := issues_model.SubmitReview(ctx, doer, issue, reviewType, content, commitID, stale, attachmentUUIDs)
	if err != nil {
		return nil, nil, err
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comm.Content)
	if err != nil {
		return nil, nil, err
	}

	notify_service.PullRequestReview(ctx, pr, review, comm, mentions)

	for _, fileComments := range review.CodeComments {
		for _, codeComment := range fileComments {
			mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, codeComment.Content)
			if err != nil {
				return nil, nil, err
			}
			notify_service.PullRequestCodeComment(ctx, pr, codeComment, mentions)
		}
	}

	return review, comm, nil
}

// DismissApprovalReviews dismiss all approval reviews because of new commits
func DismissApprovalReviews(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest) error {
	reviews, err := issues_model.FindReviews(ctx, issues_model.FindReviewOptions{
		ListOptions: db.ListOptionsAll,
		IssueID:     pull.IssueID,
		Types:       []issues_model.ReviewType{issues_model.ReviewTypeApprove},
		Dismissed:   optional.Some(false),
	})
	if err != nil {
		return err
	}

	if err := reviews.LoadIssues(ctx); err != nil {
		return err
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, review := range reviews {
			if err := issues_model.DismissReview(ctx, review, true); err != nil {
				return err
			}

			comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
				Doer:     doer,
				Content:  "New commits pushed, approval review dismissed automatically according to repository settings",
				Type:     issues_model.CommentTypeDismissReview,
				ReviewID: review.ID,
				Issue:    review.Issue,
				Repo:     review.Issue.Repo,
			})
			if err != nil {
				return err
			}

			comment.Review = review
			comment.Poster = doer
			comment.Issue = review.Issue

			notify_service.PullReviewDismiss(ctx, doer, review, comment)
		}
		return nil
	})
}

// DismissReview dismissing stale review by repo admin
func DismissReview(ctx context.Context, reviewID, repoID int64, message string, doer *user_model.User, isDismiss, dismissPriors bool) (comment *issues_model.Comment, err error) {
	review, err := issues_model.GetReviewByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	if review.Type != issues_model.ReviewTypeApprove && review.Type != issues_model.ReviewTypeReject {
		return nil, errors.New("not need to dismiss this review because it's type is not Approve or change request")
	}

	// load data for notify
	if err := review.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	// Check if the review's repoID is the one we're currently expecting.
	if review.Issue.RepoID != repoID {
		return nil, errors.New("reviews's repository is not the same as the one we expect")
	}

	issue := review.Issue

	if issue.IsClosed {
		return nil, ErrDismissRequestOnClosedPR{}
	}

	if issue.IsPull {
		if err := issue.LoadPullRequest(ctx); err != nil {
			return nil, err
		}
		if issue.PullRequest.HasMerged {
			return nil, ErrDismissRequestOnClosedPR{}
		}
	}

	if err := issues_model.DismissReview(ctx, review, isDismiss); err != nil {
		return nil, err
	}

	if dismissPriors {
		reviews, err := issues_model.FindReviews(ctx, issues_model.FindReviewOptions{
			IssueID:    review.IssueID,
			ReviewerID: review.ReviewerID,
			Dismissed:  optional.Some(false),
		})
		if err != nil {
			return nil, err
		}
		for _, oldReview := range reviews {
			if err = issues_model.DismissReview(ctx, oldReview, true); err != nil {
				return nil, err
			}
		}
	}

	if !isDismiss {
		return nil, nil
	}

	if err := review.Issue.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	comment, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Doer:     doer,
		Content:  message,
		Type:     issues_model.CommentTypeDismissReview,
		ReviewID: review.ID,
		Issue:    review.Issue,
		Repo:     review.Issue.Repo,
	})
	if err != nil {
		return nil, err
	}

	comment.Review = review
	comment.Poster = doer
	comment.Issue = review.Issue

	notify_service.PullReviewDismiss(ctx, doer, review, comment)

	return comment, nil
}

// ReCalculateLineNumber recalculates the line number based on the hunks of the diff.
// left side is the commit the comment was created on, right side is the commit the comment is displayed on.
// If the returned line number is zero, it should not be displayed.
func ReCalculateLineNumber(hunks []*git.HunkInfo, leftLine int64) int64 {
	if len(hunks) == 0 || leftLine == 0 {
		return leftLine
	}

	isLeft := leftLine < 0
	absLine := leftLine
	if isLeft {
		absLine = -leftLine
	}
	newLine := absLine

	for _, hunk := range hunks {
		if absLine < hunk.LeftLine {
			// The line is before the hunk, so we can ignore it
			continue
		} else if hunk.LeftLine <= absLine && absLine < hunk.LeftLine+hunk.LeftHunk {
			// The line is within the hunk, that means the line is deleted from the current commit
			// So that we don't need to display this line
			return 0
		}
		// The line is after the hunk, so we can add the right hunk size
		newLine += hunk.RightHunk - hunk.LeftHunk
	}
	return util.Iif(isLeft, -newLine, newLine)
}

// FetchCodeCommentsByLine fetches the code comments for a given commit, treePath and line number of a pull request.
func FetchCodeCommentsByLine(ctx context.Context, gitRepo *git.Repository, repo *repo_model.Repository, issueID int64, currentUser *user_model.User, beforeCommitID, afterCommitID, treePath string, line int64, showOutdatedComments bool) (issues_model.CommentList, error) {
	opts := issues_model.FindCommentsOptions{
		Type:     issues_model.CommentTypeCode,
		IssueID:  issueID,
		TreePath: treePath,
	}
	// load all the comments on this file and then filter them by line number
	// we cannot use the line number in the options because some comments's line number may have changed
	comments, err := issues_model.FindCodeComments(ctx, opts, repo, currentUser, nil, showOutdatedComments)
	if err != nil {
		return nil, fmt.Errorf("FindCodeComments: %w", err)
	}
	if len(comments) == 0 {
		return nil, nil
	}
	afterCommit, err := gitRepo.GetCommit(afterCommitID)
	if err != nil {
		return nil, fmt.Errorf("GetCommit[%s]: %w", afterCommitID, err)
	}
	n := 0
	hunksCache := make(map[string][]*git.HunkInfo)
	for _, comment := range comments {
		// Code comment should always have a commit SHA, if not, we need to set it based on the line number
		if comment.BeforeCommitID == "" {
			comment.BeforeCommitID = beforeCommitID
		}
		if comment.CommitSHA == "" {
			comment.CommitSHA = afterCommitID
		}

		dstCommitID := beforeCommitID
		commentCommitID := comment.BeforeCommitID
		if comment.Line > 0 {
			dstCommitID = afterCommitID
			commentCommitID = comment.CommitSHA
		}

		if commentCommitID != dstCommitID {
			// If the comment is not for the current commit, we need to recalculate the line number
			hunks, ok := hunksCache[commentCommitID+".."+dstCommitID]
			if !ok {
				hunks, err = git.GetAffectedHunksForTwoCommitsSpecialFile(ctx, repo.RepoPath(), commentCommitID, dstCommitID, treePath)
				if err != nil {
					return nil, fmt.Errorf("GetAffectedHunksForTwoCommitsSpecialFile[%s, %s, %s]: %w", repo.FullName(), commentCommitID, dstCommitID, err)
				}
				hunksCache[commentCommitID+".."+dstCommitID] = hunks
			}
			comment.Line = ReCalculateLineNumber(hunks, comment.Line)
		}

		if comment.Line == line {
			commentAfterCommit, err := gitRepo.GetCommit(comment.CommitSHA)
			if err != nil {
				return nil, fmt.Errorf("GetCommit[%s]: %w", comment.CommitSHA, err)
			}

			// If the comment is not the first one or the comment created before the current commit
			if n > 0 || comment.CommitSHA == afterCommitID ||
				commentAfterCommit.Committer.When.Before(afterCommit.Committer.When) {
				comments[n] = comment
				n++
			}
		}
	}
	return comments[:n], nil
}

// LoadCodeComments loads comments into each line, so that the comments can be displayed in the diff view.
// the comments' line number is recalculated based on the hunks of the diff.
func LoadCodeComments(ctx context.Context, gitRepo *git.Repository, repo *repo_model.Repository,
	diff *gitdiff.Diff, issueID int64, currentUser *user_model.User,
	beforeCommit, afterCommit *git.Commit, showOutdatedComments bool,
) error {
	if beforeCommit == nil || afterCommit == nil {
		return errors.New("startCommit and endCommit cannot be nil")
	}

	allComments, err := issues_model.FetchCodeComments(ctx, repo, issueID, currentUser, showOutdatedComments)
	if err != nil {
		return err
	}

	for _, file := range diff.Files {
		if fileComments, ok := allComments[file.Name]; ok {
			lineComments := make(map[int64][]*issues_model.Comment)
			hunksCache := make(map[string][]*git.HunkInfo)
			// filecomments should be sorted by created time, so that the latest comments are at the end
			for _, comment := range fileComments {
				if comment.BeforeCommitID == "" {
					comment.BeforeCommitID = beforeCommit.ID.String()
				}
				if comment.CommitSHA == "" {
					comment.CommitSHA = afterCommit.ID.String()
				}

				dstCommitID := beforeCommit.ID.String()
				commentCommitID := comment.BeforeCommitID
				if comment.Line > 0 {
					dstCommitID = afterCommit.ID.String()
					commentCommitID = comment.CommitSHA
				}

				if commentCommitID != dstCommitID {
					// If the comment is not for the current commit, we need to recalculate the line number
					hunks, ok := hunksCache[commentCommitID+".."+dstCommitID]
					if !ok {
						hunks, err = git.GetAffectedHunksForTwoCommitsSpecialFile(ctx, repo.RepoPath(), commentCommitID, dstCommitID, file.Name)
						if err != nil {
							return fmt.Errorf("GetAffectedHunksForTwoCommitsSpecialFile[%s, %s, %s]: %w", repo.FullName(), commentCommitID, dstCommitID, err)
						}
						hunksCache[commentCommitID+".."+dstCommitID] = hunks
					}
					comment.Line = ReCalculateLineNumber(hunks, comment.Line)
				}

				if comment.Line != 0 {
					commentAfterCommit, err := gitRepo.GetCommit(comment.CommitSHA)
					if err != nil {
						return fmt.Errorf("GetCommit[%s]: %w", comment.CommitSHA, err)
					}
					// If the comment is not the first one or the comment created before the current commit
					if lineComments[comment.Line] != nil || comment.CommitSHA == afterCommit.ID.String() ||
						commentAfterCommit.Committer.When.Before(afterCommit.Committer.When) {
						lineComments[comment.Line] = append(lineComments[comment.Line], comment)
					}
				}
			}

			for _, section := range file.Sections {
				for _, line := range section.Lines {
					if comments, ok := lineComments[int64(line.LeftIdx*-1)]; ok {
						line.Comments = append(line.Comments, comments...)
					}
					if comments, ok := lineComments[int64(line.RightIdx)]; ok {
						line.Comments = append(line.Comments, comments...)
					}
					sort.SliceStable(line.Comments, func(i, j int) bool {
						return line.Comments[i].CreatedUnix < line.Comments[j].CreatedUnix
					})
				}
			}
		}
	}
	return nil
}
