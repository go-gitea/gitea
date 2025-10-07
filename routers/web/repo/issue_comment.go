// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
)

func reopenPullWithComment(ctx *context.Context, issue *issues_model.Issue, content string, attachments []string) *issues_model.Comment {
	pull := issue.PullRequest

	// get head commit of branch in the head repo
	if err := pull.LoadHeadRepo(ctx); err != nil {
		ctx.ServerError("Unable to load head repo", err)
		return nil
	}

	// check whether the ref of PR <refs/pulls/pr_index/head> in base repo is consistent with the head commit of head branch in the head repo
	// get head commit of PR
	if pull.Flow == issues_model.PullRequestFlowGithub {
		prHeadRef := pull.GetGitHeadRefName()
		if err := pull.LoadBaseRepo(ctx); err != nil {
			ctx.ServerError("Unable to load base repo", err)
			return nil
		}
		prHeadCommitID, err := git.GetFullCommitID(ctx, pull.BaseRepo.RepoPath(), prHeadRef)
		if err != nil {
			ctx.ServerError("Get head commit Id of pr fail", err)
			return nil
		}

		if ok := gitrepo.IsBranchExist(ctx, pull.HeadRepo, pull.BaseBranch); !ok {
			// todo localize
			ctx.JSONError("The origin branch is delete, cannot reopen.")
			return nil
		}
		headBranchRef := git.RefNameFromBranch(pull.HeadBranch)
		headBranchCommitID, err := git.GetFullCommitID(ctx, pull.HeadRepo.RepoPath(), headBranchRef.String())
		if err != nil {
			ctx.ServerError("Get head commit Id of head branch fail", err)
			return nil
		}

		err = pull.LoadIssue(ctx)
		if err != nil {
			ctx.ServerError("load the issue of pull request error", err)
			return nil
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
				return nil
			}
		}
	}

	branchExist, err := git_model.IsBranchExist(ctx, pull.HeadRepo.ID, pull.HeadBranch)
	if err != nil {
		ctx.ServerError("IsBranchExist", err)
		return nil
	}
	if !branchExist {
		ctx.JSONError(ctx.Tr("repo.pulls.head_branch_not_exist"))
		return nil
	}

	// check if an opened pull request exists with the same head branch and base branch
	pr, err := issues_model.GetUnmergedPullRequest(ctx, pull.HeadRepoID, pull.BaseRepoID, pull.HeadBranch, pull.BaseBranch, pull.Flow)
	if err != nil {
		if !issues_model.IsErrPullRequestNotExist(err) {
			ctx.JSONError(err.Error())
			return nil
		}
	}
	if pr != nil {
		ctx.Flash.Info(ctx.Tr("repo.pulls.open_unmerged_pull_exists", pr.Index))
		return nil
	}

	createdComment, err := issue_service.ReopenIssueWithComment(ctx, issue, ctx.Doer, "", content, attachments)
	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSONError(ctx.Tr("repo.issues.comment.blocked_user"))
		} else {
			ctx.ServerError("ReopenIssue", err)
		}
		return nil
	}

	// check whether the ref of PR <refs/pulls/pr_index/head> in base repo is consistent with the head commit of head branch in the head repo
	// get head commit of PR
	if pull.Flow == issues_model.PullRequestFlowGithub {
		prHeadRef := pull.GetGitHeadRefName()
		if err := pull.LoadBaseRepo(ctx); err != nil {
			ctx.ServerError("Unable to load base repo", err)
			return nil
		}
		prHeadCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(prHeadRef)
		if err != nil {
			ctx.ServerError("Get head commit Id of pr fail", err)
			return nil
		}

		headBranchCommitID, err := gitrepo.GetBranchCommitID(ctx, pull.HeadRepo, pull.HeadBranch)
		if err != nil {
			ctx.ServerError("Get head commit Id of head branch fail", err)
			return nil
		}

		if err = pull.LoadIssue(ctx); err != nil {
			ctx.ServerError("load the issue of pull request error", err)
			return nil
		}

		// if the head commit ID of the PR is different from the head branch
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
				return nil
			}
		}

		// Regenerate patch and test conflict.
		pull.HeadCommitID = ""
		pull_service.StartPullRequestCheckImmediately(ctx, pull)
	}
	return createdComment
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

		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.Doer.IsAdmin {
		ctx.JSONError(ctx.Tr("repo.issues.comment_on_locked"))
		return
	}

	if form.Content == "" {
		ctx.JSONError(ctx.Tr("repo.issues.comment.empty_content"))
		return
	}

	var attachments []string
	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	var createdComment *issues_model.Comment
	var err error

	switch form.Status {
	case "reopen":
		if !issue.IsClosed {
			ctx.JSONError(ctx.Tr("repo.issues.not_closed"))
			return
		}
		if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) &&
			!issue.IsPoster(ctx.Doer.ID) &&
			!ctx.Doer.IsAdmin {
			ctx.JSONError(ctx.Tr("repo.issues.reopen_not_allowed"))
			return
		}

		if issue.IsPull {
			createdComment = reopenPullWithComment(ctx, issue, form.Content, attachments)
		} else {
			createdComment, err = issue_service.ReopenIssueWithComment(ctx, issue, ctx.Doer, "", form.Content, attachments)
			if err != nil {
				if errors.Is(err, user_model.ErrBlockedUser) {
					ctx.JSONError(ctx.Tr("repo.issues.comment.blocked_user"))
				} else {
					ctx.ServerError("ReopenIssue", err)
				}
				return
			}
		}
		if ctx.Written() {
			return
		}
	case "close":
		if issue.IsClosed {
			ctx.JSONError(ctx.Tr("repo.issues.already_closed"))
			return
		}

		if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) &&
			!issue.IsPoster(ctx.Doer.ID) &&
			!ctx.Doer.IsAdmin {
			ctx.JSONError(ctx.Tr("repo.issues.close_not_allowed"))
			return
		}

		createdComment, err = issue_service.CloseIssueWithComment(ctx, issue, ctx.Doer, "", form.Content, attachments)
	default:
		if len(form.Content) == 0 && len(attachments) == 0 {
			ctx.JSONError(ctx.Tr("repo.issues.comment.empty_content"))
			return
		}

		createdComment, err = issue_service.CreateIssueComment(ctx, ctx.Doer, ctx.Repo.Repository, issue, form.Content, attachments)
	}

	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSONError(ctx.Tr("repo.issues.comment.blocked_user"))
		} else {
			ctx.ServerError("CreateIssueComment", err)
		}
		return
	}

	// Redirect to comment hashtag if there is any actual content.
	typeName := util.Iif(issue.IsPull, "pulls", "issues")

	if createdComment != nil {
		log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, createdComment.ID)
		ctx.JSONRedirect(fmt.Sprintf("%s/%s/%d#%s", ctx.Repo.RepoLink, typeName, issue.Index, createdComment.HashTag()))
	} else {
		ctx.JSONRedirect(fmt.Sprintf("%s/%s/%d", ctx.Repo.RepoLink, typeName, issue.Index))
	}
}

// UpdateCommentContent change comment of issue's content
func UpdateCommentContent(ctx *context.Context) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(issues_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if !comment.Type.HasContentSupport() {
		ctx.HTTPError(http.StatusNoContent)
		return
	}

	newContent := ctx.FormString("content")
	contentVersion := ctx.FormInt("content_version")
	if contentVersion != comment.ContentVersion {
		ctx.JSONError(ctx.Tr("repo.comments.edit.already_changed"))
		return
	}

	if newContent != comment.Content {
		// allow to save empty content
		oldContent := comment.Content
		comment.Content = newContent

		if err = issue_service.UpdateComment(ctx, comment, contentVersion, ctx.Doer, oldContent); err != nil {
			if errors.Is(err, user_model.ErrBlockedUser) {
				ctx.JSONError(ctx.Tr("repo.issues.comment.blocked_user"))
			} else if errors.Is(err, issues_model.ErrCommentAlreadyChanged) {
				ctx.JSONError(ctx.Tr("repo.comments.edit.already_changed"))
			} else {
				ctx.ServerError("UpdateComment", err)
			}
			return
		}
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

	var renderedContent template.HTML
	if comment.Content != "" {
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(comment.ID, 10),
		})
		renderedContent, err = markdown.RenderString(rctx, comment.Content)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
	}

	if strings.TrimSpace(string(renderedContent)) == "" {
		renderedContent = htmlutil.HTMLFormat(`<span class="no-content">%s</span>`, ctx.Tr("repo.issues.no_content"))
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":        renderedContent,
		"contentVersion": comment.ContentVersion,
		"attachments":    attachmentsHTML(ctx, comment.Attachments, comment.Content),
	})
}

// DeleteComment delete comment of issue
func DeleteComment(ctx *context.Context) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(issues_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.HTTPError(http.StatusForbidden)
		return
	} else if !comment.Type.HasContentSupport() {
		ctx.HTTPError(http.StatusNoContent)
		return
	}

	if err = issue_service.DeleteComment(ctx, ctx.Doer, comment); err != nil {
		ctx.ServerError("DeleteComment", err)
		return
	}

	ctx.Status(http.StatusOK)
}

// ChangeCommentReaction create a reaction for comment
func ChangeCommentReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(issues_model.ErrCommentNotExist{})
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

		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if !comment.Type.HasContentSupport() {
		ctx.HTTPError(http.StatusNoContent)
		return
	}

	switch ctx.PathParam("action") {
	case "react":
		reaction, err := issue_service.CreateCommentReaction(ctx, ctx.Doer, comment, form.Content)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) || errors.Is(err, user_model.ErrBlockedUser) {
				ctx.ServerError("ChangeIssueReaction", err)
				return
			}
			log.Info("CreateCommentReaction: %s", err)
			break
		}
		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(ctx, ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment created: %d/%d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID, reaction.ID)
	case "unreact":
		if err := issues_model.DeleteCommentReaction(ctx, ctx.Doer.ID, comment.Issue.ID, comment.ID, form.Content); err != nil {
			ctx.ServerError("DeleteCommentReaction", err)
			return
		}

		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(ctx, ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment removed: %d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID)
	default:
		ctx.NotFound(nil)
		return
	}

	if len(comment.Reactions) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.RenderToHTML(tplReactions, map[string]any{
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

// GetCommentAttachments returns attachments for the comment
func GetCommentAttachments(ctx *context.Context) {
	comment, err := issues_model.GetCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", issues_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", issues_model.IsErrIssueNotExist, err)
		return
	}

	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(issues_model.ErrCommentNotExist{})
		return
	}

	if !ctx.Repo.Permission.CanReadIssuesOrPulls(comment.Issue.IsPull) {
		ctx.NotFound(issues_model.ErrCommentNotExist{})
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
