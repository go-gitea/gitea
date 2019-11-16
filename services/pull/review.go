// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"bytes"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/gitdiff"
)

// CreateCodeComment creates a comment on the code line
func CreateCodeComment(doer *models.User, issue *models.Issue, line int64, content string, treePath string, isReview bool, replyReviewID int64) (*models.Comment, error) {
	// It's not a review, maybe a reply to a review comment or a single comment.
	if !isReview {
		if err := issue.LoadRepo(); err != nil {
			return nil, err
		}

		comment, err := createCodeComment(
			doer,
			issue.Repo,
			issue,
			content,
			treePath,
			line,
			replyReviewID,
		)
		if err != nil {
			return nil, err
		}

		notification.NotifyCreateIssueComment(doer, issue.Repo, issue, comment)

		return comment, nil
	}

	review, err := models.GetCurrentReview(doer, issue)
	if err != nil {
		if !models.IsErrReviewNotExist(err) {
			return nil, err
		}

		review, err = models.CreateReview(models.CreateReviewOptions{
			Type:     models.ReviewTypePending,
			Reviewer: doer,
			Issue:    issue,
		})
		if err != nil {
			return nil, err
		}
	}

	comment, err := createCodeComment(
		doer,
		issue.Repo,
		issue,
		content,
		treePath,
		line,
		review.ID,
	)
	if err != nil {
		return nil, err
	}

	// NOTICE: it's a pending review, so the notifications will not be fired until user submit review.

	return comment, nil
}

// createCodeComment creates a plain code comment at the specified line / path
func createCodeComment(doer *models.User, repo *models.Repository, issue *models.Issue, content, treePath string, line, reviewID int64) (*models.Comment, error) {
	var commitID, patch string
	if err := issue.LoadPullRequest(); err != nil {
		return nil, fmt.Errorf("GetPullRequestByIssueID: %v", err)
	}
	pr := issue.PullRequest
	if err := pr.GetBaseRepo(); err != nil {
		return nil, fmt.Errorf("GetHeadRepo: %v", err)
	}
	gitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	// FIXME validate treePath
	// Get latest commit referencing the commented line
	// No need for get commit for base branch changes
	if line > 0 {
		commit, err := gitRepo.LineBlame(pr.GetGitRefName(), gitRepo.Path, treePath, uint(line))
		if err == nil {
			commitID = commit.ID.String()
		} else if !strings.Contains(err.Error(), "exit status 128 - fatal: no such path") {
			return nil, fmt.Errorf("LineBlame[%s, %s, %s, %d]: %v", pr.GetGitRefName(), gitRepo.Path, treePath, line, err)
		}
	}

	// Only fetch diff if comment is review comment
	if reviewID != 0 {
		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return nil, fmt.Errorf("GetRefCommitID[%s]: %v", pr.GetGitRefName(), err)
		}
		patchBuf := new(bytes.Buffer)
		if err := gitdiff.GetRawDiffForFile(gitRepo.Path, pr.MergeBase, headCommitID, gitdiff.RawDiffNormal, treePath, patchBuf); err != nil {
			return nil, fmt.Errorf("GetRawDiffForLine[%s, %s, %s, %s]: %v", err, gitRepo.Path, pr.MergeBase, headCommitID, treePath)
		}
		patch = gitdiff.CutDiffAroundLine(patchBuf, int64((&models.Comment{Line: line}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines)
	}
	return models.CreateComment(&models.CreateCommentOptions{
		Type:      models.CommentTypeCode,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		Content:   content,
		LineNum:   line,
		TreePath:  treePath,
		CommitSHA: commitID,
		ReviewID:  reviewID,
		Patch:     patch,
		NoAction:  true,
	})
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(doer *models.User, issue *models.Issue, reviewType models.ReviewType, content string) (*models.Review, *models.Comment, error) {
	review, comm, err := models.SubmitReview(doer, issue, reviewType, content)
	if err != nil {
		return nil, nil, err
	}

	pr, err := issue.GetPullRequest()
	if err != nil {
		return nil, nil, err
	}
	notification.NotifyPullRequestReview(pr, review, comm)

	return review, comm, nil
}
