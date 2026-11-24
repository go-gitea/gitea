// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	"gitea.dev/services/gitdiff"
	notify_service "gitea.dev/services/notify"
)

func isErrBlameInvalidated(err error) bool {
	return gitcmd.IsStderr(err, gitcmd.StderrNoSuchPath, gitcmd.StderrFileNoEnoughLines, gitcmd.StderrBadObject)
}

// ErrDismissRequestOnClosedPR represents an error when a user tries to dismiss a review associated to a closed or merged PR.
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

// ErrSubmitReviewOnClosedPR represents an error when a user tries to submit an approve or reject review associated to a closed or merged PR.
var ErrSubmitReviewOnClosedPR = errors.New("can't submit review for a closed or merged PR")

type codeCommentDiff struct {
	before, after, path string
}

func getCodeCommentDiff(ctx context.Context, diffCache *cache.EphemeralCache, gitRepo *git.Repository, diff codeCommentDiff) ([]git.FileDiffHunk, error) {
	return cache.GetWithEphemeralCache(ctx, diffCache, "codeCommentDiff", diff, func(ctx context.Context, key codeCommentDiff) ([]git.FileDiffHunk, error) {
		return git.GetFileDiffHunks(ctx, gitRepo, key.before, key.after, key.path)
	})
}

func codeCommentLineChanged(ctx context.Context, comment *issues_model.Comment, repo *repo_model.Repository, branch string) (bool, error) {
	// FIXME differentiate between previous and proposed line
	commitID, err := git.LineBlameCommitID(ctx, repo, branch, comment.TreePath, uint(comment.UnsignedLine()))
	if isErrBlameInvalidated(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return comment.CommitSHA != "" && comment.CommitSHA != commitID, nil
}

func codeCommentRangeChanged(ctx context.Context, diffCache *cache.EphemeralCache, gitRepo *git.Repository, comment *issues_model.Comment, mergeBase, head string) (bool, error) {
	hunks, err := getCodeCommentDiff(ctx, diffCache, gitRepo, codeCommentDiff{before: comment.CodeCommitID(), after: head, path: comment.TreePath})
	if errors.Is(err, util.ErrNotExist) {
		return true, nil
	} else if err != nil || len(hunks) == 0 {
		return false, err
	}

	start, end := int64(comment.UnsignedCodeStartLine()), int64(comment.UnsignedLine())
	if comment.CodeStartLine() < 0 && comment.Line > 0 {
		start, err = mixedCodeCommentRangeStart(ctx, diffCache, gitRepo, comment, mergeBase)
		if errors.Is(err, util.ErrNotExist) {
			return true, nil
		} else if err != nil {
			return false, err
		}
	}
	return slices.ContainsFunc(hunks, func(hunk git.FileDiffHunk) bool {
		return hunk.OldStart <= end && hunk.OldStart+max(hunk.OldCount, 1) > start
	}), nil
}

func mixedCodeCommentRangeStart(ctx context.Context, diffCache *cache.EphemeralCache, gitRepo *git.Repository, comment *issues_model.Comment, mergeBase string) (int64, error) {
	oldLine := -comment.CodeStartLine()
	if _, patch, ok := strings.Cut(comment.Patch, "\n@@ "); ok {
		header, _, _ := strings.Cut(patch, "\n")
		leftStart, _, rightStart, _ := git.ParseDiffHunkString("@@ " + header)
		if int64(leftStart) == oldLine {
			return int64(rightStart), nil
		}
	}
	if mergeBase != "" {
		anchorBase, err := git.MergeBase(ctx, gitRepo, mergeBase, comment.CodeCommitID())
		if err != nil {
			return 0, err
		}
		mergeBase = anchorBase
	}
	hunks, err := getCodeCommentDiff(ctx, diffCache, gitRepo, codeCommentDiff{before: mergeBase, after: comment.CodeCommitID(), path: comment.TreePath})
	if err != nil {
		return 0, err
	}
	var offset int64
	for _, hunk := range hunks {
		if oldLine < hunk.OldStart {
			break
		}
		if oldLine < hunk.OldStart+hunk.OldCount {
			return hunk.NewStart, nil
		}
		offset = hunk.NewStart + hunk.NewCount - hunk.OldStart - hunk.OldCount
	}
	return oldLine + offset, nil
}

// InvalidateCodeComments will look up the prs for code comments which got invalidated by change
func InvalidateCodeComments(ctx context.Context, prs issues_model.PullRequestList, repo *repo_model.Repository, branch string) error {
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
	if len(codeComments) == 0 {
		return nil
	}
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return err
	}
	defer closer.Close()
	head, err := gitRepo.GetRefCommitID(ctx, branch)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return err
	}
	mergeBases := make(map[int64]string, len(prs))
	for _, pr := range prs {
		mergeBases[pr.IssueID] = pr.MergeBase
	}
	diffCache := cache.NewEphemeralCache()
	for _, comment := range codeComments {
		var changed bool
		var err error
		if head == "" {
			changed = true
		} else if comment.CodeCommitID() != "" && comment.CodeStartLine() != comment.Line {
			changed, err = codeCommentRangeChanged(ctx, diffCache, gitRepo, comment, mergeBases[comment.IssueID], head)
		} else {
			changed, err = codeCommentLineChanged(ctx, comment, repo, head)
		}
		if err != nil {
			return err
		}
		if changed {
			comment.Invalidated = true
			if err := issues_model.UpdateCommentInvalidate(ctx, comment); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateCodeComment creates a code comment in a review on the code line:
// When pendingReview=false:
// * no replyReviewID: create a new review and add the comment to it, and submit the review
// * replyReviewID is set: add the comment to the existing submitted review
// When pendingReview=true: get current pending review or create a new one, and add the comment to it
func CreateCodeComment(ctx context.Context, doer *user_model.User, gitRepo *git.Repository, issue *issues_model.Issue, pendingReview bool, latestCommitID string, opts issues_model.CreateCommentOptions) (_ *issues_model.Comment, err error) {
	if err := checkReviewerNotBlocked(ctx, doer, issue); err != nil {
		return nil, err
	}
	if opts.CodeStartLine == opts.LineNum {
		opts.CodeStartLine = 0
	}
	if err := ValidateCodeCommentRange(opts.CodeStartLine, opts.LineNum); err != nil {
		return nil, err
	}
	opts.Type, opts.Doer, opts.Repo, opts.Issue = issues_model.CommentTypeCode, doer, issue.Repo, issue
	replyReviewID := opts.ReviewID

	replyToSubmittedReview := !pendingReview && replyReviewID != 0
	if replyToSubmittedReview {
		// It's not part of a review; maybe a reply to a review comment or a single comment.
		// Check if there are reviews for that line already; if there are, this is a reply
		if replyToSubmittedReview, err = issues_model.ReviewExists(ctx, issue, opts.TreePath, opts.LineNum); err != nil {
			return nil, err
		}
	}

	// Comments that are replies don't require a review header to show up in the issue view
	// Add the code comment to existing submitted review
	if replyToSubmittedReview {
		replyReview, err := issues_model.GetReviewByID(ctx, replyReviewID)
		if err != nil {
			return nil, err
		} else if replyReview.IssueID != issue.ID || (replyReview.Type == issues_model.ReviewTypePending && replyReview.ReviewerID != doer.ID) {
			return nil, issues_model.ErrReviewNotExist{ID: replyReviewID}
		}

		comment, err := createCodeComment(ctx, &opts, replyReview, latestCommitID)
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

	// If no existing submitted review, create a pending review and its comment in a single transaction
	// to avoid failed code comment creation leaving a pending review without any comments
	comment, err := db.WithTx2(ctx, func(ctx context.Context) (*issues_model.Comment, error) {
		curReview, err := issues_model.GetCurrentReview(ctx, doer, issue)
		if errors.Is(err, util.ErrNotExist) {
			// create a new pending review for the code comment
			if curReview, err = issues_model.CreateReview(ctx, issues_model.CreateReviewOptions{
				Type:     issues_model.ReviewTypePending,
				Reviewer: doer,
				Issue:    issue,
				Official: false,
				CommitID: latestCommitID,
			}); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		// add the code comment to the pending review
		return createCodeComment(ctx, &opts, curReview, latestCommitID)
	})
	if err != nil {
		return nil, err
	}

	if !pendingReview {
		// Doer is creating a submitted review, so submit the pending review we've just created,
		// then the comment shows up in the issue view
		if _, _, err = SubmitReview(ctx, doer, gitRepo, issue, issues_model.ReviewTypeComment, "", latestCommitID, nil); err != nil {
			return nil, err
		}
	}

	// NOTICE: if it's a pending review the notifications will not be fired until user submit review.

	return comment, nil
}

func checkReviewerNotBlocked(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) error {
	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}
	if !user_model.IsUserBlockedBy(ctx, doer, issue.PosterID, issue.Repo.OwnerID) || access_model.IsUserRepoAdmin(ctx, issue.Repo, doer) {
		return nil
	}
	if !user_model.IsUserBlockedBy(ctx, doer, issue.Repo.OwnerID) { // the author's block must not keep official reviewers from requesting changes
		if official, err := issues_model.IsOfficialReviewer(ctx, issue, doer); err != nil || official {
			return err
		}
	}
	return util.ErrorWrapTranslatable(user_model.ErrBlockedUser, "repo.issues.comment.blocked_user")
}

func ValidateCodeCommentRange(startLine, line int64) error {
	if startLine > 0 && startLine > line || startLine < 0 && line <= 0 && startLine < line {
		return util.NewInvalidArgumentErrorf("invalid code comment line range %d to %d", startLine, line)
	}
	return nil
}

// createCodeComment creates a plain code comment at the specified line / path
func createCodeComment(ctx context.Context, opts *issues_model.CreateCommentOptions, review *issues_model.Review, latestCommitID string) (*issues_model.Comment, error) {
	opts.ReviewID = review.ID
	line, treePath := opts.LineNum, opts.TreePath
	if err := opts.Issue.LoadPullRequest(ctx); err != nil {
		return nil, fmt.Errorf("createCodeComment: load pull request: %w", err)
	}
	pr := opts.Issue.PullRequest
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, fmt.Errorf("createCodeComment: load base repository: %w", err)
	}
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, pr.BaseRepo)
	if err != nil {
		return nil, fmt.Errorf("createCodeComment: open base repository: %w", err)
	}
	defer closer.Close()

	comments, err := issues_model.FindComments(ctx, &issues_model.FindCommentsOptions{
		ReviewID:    review.ID,
		Line:        line,
		TreePath:    treePath,
		Type:        issues_model.CommentTypeCode,
		ListOptions: db.ListOptions{PageSize: 1, Page: 1},
	})
	if err != nil {
		return nil, fmt.Errorf("createCodeComment: find first comment for review %d, line %d, path %s: %w", review.ID, line, treePath, err)
	}

	prHeadRef := pr.GetGitHeadRefName()
	headRef := prHeadRef
	if len(comments) > 0 {
		first := comments[0]
		opts.CodeCommitID = first.CodeCommitID()
		if opts.CodeStartLine == 0 && first.CodeStartLine() != line {
			opts.CodeStartLine = first.CodeStartLine()
		}
		if line > 0 {
			opts.CommitSHA, opts.Invalidated = first.CommitSHA, first.Invalidated
			if cmp.Or(opts.CodeStartLine, line) == first.CodeStartLine() {
				opts.Patch = first.Patch
			}
		}
	} else if line > 0 {
		headRef = cmp.Or(review.CommitID, prHeadRef)
	}

	if line > 0 && opts.CommitSHA == "" {
		opts.CommitSHA, err = git.LineBlameCommitID(ctx, pr.BaseRepo, headRef, treePath, uint(line))
		if err != nil && headRef != prHeadRef {
			opts.CommitSHA, err = git.LineBlameCommitID(ctx, pr.BaseRepo, prHeadRef, treePath, uint(line))
		}
		if err != nil {
			log.Debug("LineBlame[%s, %s, %s, %d]: %v", prHeadRef, gitRepo.LogString(), treePath, line, err)
		}
	}

	if opts.Patch == "" {
		headCommitID, err := gitRepo.GetRefCommitID(ctx, prHeadRef)
		if err != nil {
			return nil, fmt.Errorf("createCodeComment: resolve head ref %s: %w", prHeadRef, err)
		}
		opts.CommitSHA = cmp.Or(opts.CommitSHA, headCommitID)
		if len(comments) == 0 {
			opts.CodeCommitID = util.Iif(line < 0, pr.MergeBase, headCommitID)
			if anchorRef := cmp.Or(latestCommitID, headRef); line > 0 && anchorRef != headCommitID && anchorRef != prHeadRef {
				if anchorCommitID, err := gitRepo.GetRefCommitID(ctx, anchorRef); err == nil {
					opts.CodeCommitID = anchorCommitID
				} else if !errors.Is(err, util.ErrNotExist) {
					return nil, fmt.Errorf("createCodeComment: resolve anchor %s: %w", anchorRef, err)
				}
			}
		}
		opts.Patch, err = gitdiff.GetCodeCommentPatch(ctx, gitRepo, pr.MergeBase, headCommitID, treePath, line, opts.CodeStartLine)
		if err != nil {
			return nil, err
		}
	}
	return issues_model.CreateComment(ctx, opts)
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(ctx context.Context, doer *user_model.User, gitRepo *git.Repository, issue *issues_model.Issue, reviewType issues_model.ReviewType, content, commitID string, attachmentUUIDs []string) (*issues_model.Review, *issues_model.Comment, error) {
	if err := checkReviewerNotBlocked(ctx, doer, issue); err != nil {
		return nil, nil, err
	}

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

		headCommitID, err := gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
		if err != nil {
			return nil, nil, err
		}

		if headCommitID == commitID {
			stale = false
		} else {
			stale, _, err = checkIfPRContentChanged(ctx, pr, commitID, headCommitID)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	review, comm, err := issues_model.SubmitReview(ctx, doer, issue, reviewType, content, commitID, stale, attachmentUUIDs)
	if err != nil {
		return nil, nil, err
	}
	if reviewType == issues_model.ReviewTypePending {
		return review, nil, nil
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comm.Content)
	if err != nil {
		return nil, nil, err
	}

	notify_service.PullRequestReview(ctx, pr, review, comm, mentions)

	for _, lines := range review.CodeComments {
		for _, comments := range lines {
			for _, codeComment := range comments {
				mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, codeComment.Content)
				if err != nil {
					return nil, nil, err
				}
				notify_service.PullRequestCodeComment(ctx, pr, codeComment, mentions)
			}
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
		return nil, nil //nolint:nilnil // return nil because this is not a dismiss action
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
