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
	//FIXME check if line and treepath exist
	var err error
	comment, err = models.CreateCodeComment(
		ctx.User,
		issue.Repo,
		issue,
		form.CommitSHA,
		form.Content,
		form.TreePath,
		signedLine,
	)
	if err != nil {
		ctx.ServerError("CreateCodeComment", err)
		return
	}

	if form.IsReview {
		review, err := models.GetPendingReviewByReviewer(ctx.User, issue)
		if err != nil {
			ctx.ServerError("CreateCodeComment", err)
			return
		}
		if review == nil {
			if review, err = models.CreatePendingReview(ctx.User, issue); err != nil {
				ctx.ServerError("CreateCodeComment", err)
				return
			}
		}
		comment.Review = review
		comment.ReviewID = review.ID
		if err = models.UpdateComment(comment); err != nil {
			ctx.ServerError("CreateCodeComment", err)
			return
		}
	} else {
		notification.Service.NotifyIssue(issue, ctx.User.ID)
	}

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
}
