// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/gitdiff"
	pull_service "code.gitea.io/gitea/services/pull"
)

const (
	tplConversation base.TplName = "repo/diff/conversation"
	tplNewComment   base.TplName = "repo/diff/new_comment"
)

// RenderNewCodeCommentForm will render the form for creating a new review comment
func RenderNewCodeCommentForm(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if !issue.IsPull {
		return
	}
	currentReview, err := models.GetCurrentReview(ctx.User, issue)
	if err != nil && !models.IsErrReviewNotExist(err) {
		ctx.ServerError("GetCurrentReview", err)
		return
	}
	ctx.Data["PageIsPullFiles"] = true
	ctx.Data["Issue"] = issue
	ctx.Data["CurrentReview"] = currentReview
	pullHeadCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(issue.PullRequest.GetGitRefName())
	if err != nil {
		ctx.ServerError("GetRefCommitID", err)
		return
	}
	ctx.Data["AfterCommitID"] = pullHeadCommitID
	ctx.HTML(http.StatusOK, tplNewComment)
}

// CreateCodeComment will create a code comment including an pending review if required
func CreateCodeComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CodeCommentForm)
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

	signedLine := form.Line
	if form.Side == "previous" {
		signedLine *= -1
	}

	comment, err := pull_service.CreateCodeComment(ctx,
		ctx.User,
		ctx.Repo.GitRepo,
		issue,
		signedLine,
		form.Content,
		form.TreePath,
		form.IsReview,
		form.Reply,
		form.LatestCommitID,
	)
	if err != nil {
		ctx.ServerError("CreateCodeComment", err)
		return
	}

	if comment == nil {
		log.Trace("Comment not created: %-v #%d[%d]", ctx.Repo.Repository, issue.Index, issue.ID)
		ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files", ctx.Repo.RepoLink, issue.Index))
		return
	}

	log.Trace("Comment created: %-v #%d[%d] Comment[%d]", ctx.Repo.Repository, issue.Index, issue.ID, comment.ID)

	if form.Origin == "diff" {
		renderConversation(ctx, comment)
		return
	}
	ctx.Redirect(comment.HTMLURL())
}

// UpdateResolveConversation add or remove an Conversation resolved mark
func UpdateResolveConversation(ctx *context.Context) {
	origin := ctx.FormString("origin")
	action := ctx.FormString("action")
	commentID := ctx.FormInt64("comment_id")

	comment, err := models.GetCommentByID(commentID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	if err = comment.LoadIssue(); err != nil {
		ctx.ServerError("comment.LoadIssue", err)
		return
	}

	var permResult bool
	if permResult, err = models.CanMarkConversation(comment.Issue, ctx.User); err != nil {
		ctx.ServerError("CanMarkConversation", err)
		return
	}
	if !permResult {
		ctx.Error(http.StatusForbidden)
		return
	}

	if !comment.Issue.IsPull {
		ctx.Error(http.StatusBadRequest)
		return
	}

	if action == "Resolve" || action == "UnResolve" {
		err = models.MarkConversation(comment, ctx.User, action == "Resolve")
		if err != nil {
			ctx.ServerError("MarkConversation", err)
			return
		}
	} else {
		ctx.Error(http.StatusBadRequest)
		return
	}

	if origin == "diff" {
		renderConversation(ctx, comment)
		return
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

func renderConversation(ctx *context.Context, comment *models.Comment) {
	comments, err := models.FetchCodeCommentsByLine(ctx, comment.Issue, ctx.User, comment.TreePath, comment.Line)
	if err != nil {
		ctx.ServerError("FetchCodeCommentsByLine", err)
		return
	}
	ctx.Data["PageIsPullFiles"] = true
	ctx.Data["comments"] = comments
	ctx.Data["CanMarkConversation"] = true
	ctx.Data["Issue"] = comment.Issue
	if err = comment.Issue.LoadPullRequest(); err != nil {
		ctx.ServerError("comment.Issue.LoadPullRequest", err)
		return
	}
	pullHeadCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(comment.Issue.PullRequest.GetGitRefName())
	if err != nil {
		ctx.ServerError("GetRefCommitID", err)
		return
	}
	ctx.Data["AfterCommitID"] = pullHeadCommitID
	ctx.HTML(http.StatusOK, tplConversation)
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.SubmitReviewForm)
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

	reviewType := form.ReviewType()
	switch reviewType {
	case models.ReviewTypeUnknown:
		ctx.ServerError("ReviewType", fmt.Errorf("unknown ReviewType: %s", form.Type))
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

	var attachments []string
	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	_, comm, err := pull_service.SubmitReview(ctx, ctx.User, ctx.Repo.GitRepo, issue, reviewType, form.Content, form.CommitID, attachments)
	if err != nil {
		if models.IsContentEmptyErr(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.review.content.empty"))
			ctx.Redirect(fmt.Sprintf("%s/pulls/%d/files", ctx.Repo.RepoLink, issue.Index))
		} else {
			ctx.ServerError("SubmitReview", err)
		}
		return
	}

	ctx.Redirect(fmt.Sprintf("%s/pulls/%d#%s", ctx.Repo.RepoLink, issue.Index, comm.HashTag()))
}

// DismissReview dismissing stale review by repo admin
func DismissReview(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.DismissReviewForm)
	comm, err := pull_service.DismissReview(ctx, form.ReviewID, form.Message, ctx.User, true)
	if err != nil {
		ctx.ServerError("pull_service.DismissReview", err)
		return
	}

	ctx.Redirect(fmt.Sprintf("%s/pulls/%d#%s", ctx.Repo.RepoLink, comm.Issue.Index, comm.HashTag()))
}

// GetUserSpecificDiff is like gitdiff.GetDiff, except that user specific data such as which files the given user has already viewed will also be set
// This function should have been a part of the services/gitdiff - package, but doing that results in an import cycle for weird reasons
func GetUserSpecificDiff(ctx *context.Context, gitRepo *git.Repository, opts *gitdiff.DiffOptions, files ...string) (*gitdiff.Diff, error) {
	diff, err := gitdiff.GetDiff(gitRepo, opts, files...)
	if err != nil || !ctx.IsSigned {
		return diff, err
	}
	pull := checkPullInfo(ctx)
	review, err := models.GetNewestReview(ctx.User.ID, pull.ID)
	if err != nil || review == nil || review.ViewedFiles == nil {
		return diff, err
	}

	// Prepation to check for each file whether it was changed since the last review
	changedFilesString, err := git.NewCommand(ctx, "diff", "--name-only", "--", fmt.Sprintf("%s..%s", review.CommitSHA, pull.PullRequest.HeadCommitID)).RunInDirTimeout(10 * 1000 * 1000 * 1000, ctx.Repo.GitRepo.Path)
	if err != nil {
		return diff, err
	}
	changedFiles := strings.Split(string(changedFilesString), "\n")

outer:
	for _, diffFile := range diff.Files {

		// Check whether the file has changed since the last review
		for _, changedFile := range changedFiles {
			diffFile.HasChangedSinceLastReview = isSameFile(diffFile, changedFile)
			if diffFile.HasChangedSinceLastReview {
				continue outer // We don't want to check if the file is viewed here as that would fold the file, which is in this case unwanted
			}
		}
		// Check whether the file has already been viewed
		for file, viewed := range review.ViewedFiles {
			if isSameFile(diffFile, file)  {
				diffFile.IsViewed = viewed
				diff.NumViewedFiles++
				break
			}
		}
	}
	ctx.PageData["numberOfFiles"] = diff.NumFiles
	ctx.PageData["numberOfViewedFiles"] = diff.NumViewedFiles

	return diff, err
}

// isSameFile returns whether the given diff file and the given file name point at the same file
func isSameFile(diffFile *gitdiff.DiffFile, file string) bool {
	return diffFile.Name == file || (diffFile.Name == "" && diffFile.OldName == file)
}
