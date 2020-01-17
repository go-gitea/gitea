// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	comment_service "code.gitea.io/gitea/services/comments"
	pull_service "code.gitea.io/gitea/services/pull"
)

// CreateCodeComment will create a code comment including an pending review if required
func CreateCodeComment(ctx *context.Context, form auth.CodeCommentForm) {
	issue := GetActionIssue(ctx)
	if !issue.IsPull {
		return
	}
	if ctx.Written() {
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files", ctx.Repo.RepoLink, issue.Index))
		return
	}
	var comment *models.Comment
	defer func() {
		if comment != nil {
			ctx.Redirect(comment.HTMLURL())
		} else {
			ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files", ctx.Repo.RepoLink, issue.Index))
		}
	}()
	signedLine := form.Line
	if form.Side == "previous" {
		signedLine *= -1
	}

	review := new(models.Review)
	if form.IsReview {
		var err error
		// Check if the user has already a pending review for this issue
		if review, err = models.GetCurrentReview(ctx.User, issue); err != nil {
			if !models.IsErrReviewNotExist(err) {
				ctx.ServerError("CreateCodeComment", err)
				return
			}
			// No pending review exists
			// Create a new pending review for this issue & user
			if review, err = pull_service.CreateReview(models.CreateReviewOptions{
				Type:     models.ReviewTypePending,
				Reviewer: ctx.User,
				Issue:    issue,
			}); err != nil {
				ctx.ServerError("CreateCodeComment", err)
				return
			}

		}
	}
	if review.ID == 0 {
		review.ID = form.Reply
	}
	//FIXME check if line, commit and treepath exist
	comment, err := comment_service.CreateCodeComment(
		ctx.User,
		issue.Repo,
		issue,
		form.Content,
		form.TreePath,
		signedLine,
		review.ID,
	)
	if err != nil {
		ctx.ServerError("CreateCodeComment", err)
		return
	}
	// Send no notification if comment is pending
	if !form.IsReview || form.Reply != 0 {
		notification.NotifyCreateIssueComment(ctx.User, issue.Repo, issue, comment)
	}

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(ctx *context.Context, form auth.SubmitReviewForm) {
	issue := GetActionIssue(ctx)
	if !issue.IsPull {
		return
	}
	if ctx.Written() {
		return
	}
	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files", ctx.Repo.RepoLink, issue.Index))
		return
	}
	var review *models.Review
	var err error

	reviewType := form.ReviewType()

	switch reviewType {
	case models.ReviewTypeUnknown:
		ctx.ServerError("GetCurrentReview", fmt.Errorf("unknown ReviewType: %s", form.Type))
		return

	// can not approve/reject your own PR
	case models.ReviewTypeApprove, models.ReviewTypeReject:
		if issue.IsPoster(ctx.User.ID) {
			var translated string

			if reviewType == models.ReviewTypeApprove {
				translated = ctx.Tr("repo.issues.review.self.approval")
			} else {
				translated = ctx.Tr("repo.issues.review.self.rejection")
			}

			ctx.Flash.Error(translated)
			ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files", ctx.Repo.RepoLink, issue.Index))
			return
		}
	}

	review, err = models.GetCurrentReview(ctx.User, issue)
	if err == nil {
		review.Issue = issue
		if errl := review.LoadCodeComments(); errl != nil {
			ctx.ServerError("LoadCodeComments", err)
			return
		}
	}

	if ((err == nil && len(review.CodeComments) == 0) ||
		(err != nil && models.IsErrReviewNotExist(err))) &&
		form.HasEmptyContent() {
		ctx.Flash.Error(ctx.Tr("repo.issues.review.content.empty"))
		ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files", ctx.Repo.RepoLink, issue.Index))
		return
	}

	if err != nil {
		if !models.IsErrReviewNotExist(err) {
			ctx.ServerError("GetCurrentReview", err)
			return
		}
		// No current review. Create a new one!
		if review, err = models.CreateReview(models.CreateReviewOptions{
			Type:     reviewType,
			Issue:    issue,
			Reviewer: ctx.User,
			Content:  form.Content,
		}); err != nil {
			ctx.ServerError("CreateReview", err)
			return
		}
	} else {
		review.Content = form.Content
		review.Type = reviewType
		if err = models.UpdateReview(review); err != nil {
			ctx.ServerError("UpdateReview", err)
			return
		}
	}

	// Hotfix 1.10.0: make sure the review exists before creating the head comment
	if err = review.Publish(); err != nil {
		ctx.ServerError("Publish", err)
		return
	}
	comm, err := models.CreateComment(&models.CreateCommentOptions{
		Type:     models.CommentTypeReview,
		Doer:     ctx.User,
		Content:  review.Content,
		Issue:    issue,
		Repo:     issue.Repo,
		ReviewID: review.ID,
	})
	if err != nil || comm == nil {
		ctx.ServerError("CreateComment", err)
		return
	}

	pr, err := issue.GetPullRequest()
	if err != nil {
		ctx.ServerError("GetPullRequest", err)
		return
	}
	notification.NotifyPullRequestReview(pr, review, comm)

	ctx.Redirect(fmt.Sprintf("%s/pulls/%d#%s", ctx.Repo.RepoLink, issue.Index, comm.HashTag()))
}
