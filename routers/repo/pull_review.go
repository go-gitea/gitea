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
			ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files#%s", ctx.Repo.RepoLink, issue.Index, comment.HashTag()))
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
			if review, err = models.CreateReview(models.CreateReviewOptions{
				Type:     models.ReviewTypePending,
				Reviewer: ctx.User,
				Issue:    issue,
			}); err != nil {
				ctx.ServerError("CreateCodeComment", err)
				return
			}
		}
	}

	//FIXME check if line, commit and treepath exist
	var err error
	comment, err = models.CreateCodeComment(
		ctx.User,
		issue.Repo,
		issue,
		form.CommitSHA,
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
	if !form.IsReview {
		notification.Service.NotifyIssue(issue, ctx.User.ID)
	}

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
}
