// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
)

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

	var attachments []string
	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
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
						ctx.JSONError(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
						return
					}
				}

				// Regenerate patch and test conflict.
				if pr == nil {
					issue.PullRequest.HeadCommitID = ""
					pull_service.AddToTaskQueue(ctx, issue.PullRequest)
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
						ctx.JSONError("The origin branch is delete, cannot reopen.")
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
				if form.Status == "close" && !issue.IsClosed {
					if err := issue_service.CloseIssue(ctx, issue, ctx.Doer, ""); err != nil {
						log.Error("CloseIssue: %v", err)
						if issues_model.IsErrDependenciesLeft(err) {
							if issue.IsPull {
								ctx.JSONError(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
							} else {
								ctx.JSONError(ctx.Tr("repo.issues.dependency.issue_close_blocked"))
							}
							return
						}
					} else {
						if err := stopTimerIfAvailable(ctx, ctx.Doer, issue); err != nil {
							ctx.ServerError("stopTimerIfAvailable", err)
							return
						}
						log.Trace("Issue [%d] status changed to closed: %v", issue.ID, issue.IsClosed)
					}
				} else if form.Status == "reopen" && issue.IsClosed {
					if err := issue_service.ReopenIssue(ctx, issue, ctx.Doer, ""); err != nil {
						log.Error("ReopenIssue: %v", err)
					}
				}
			}
		}

		// Redirect to comment hashtag if there is any actual content.
		typeName := "issues"
		if issue.IsPull {
			typeName = "pulls"
		}
		if comment != nil {
			ctx.JSONRedirect(fmt.Sprintf("%s/%s/%d#%s", ctx.Repo.RepoLink, typeName, issue.Index, comment.HashTag()))
		} else {
			ctx.JSONRedirect(fmt.Sprintf("%s/%s/%d", ctx.Repo.RepoLink, typeName, issue.Index))
		}
	}()

	// Fix #321: Allow empty comments, as long as we have attachments.
	if len(form.Content) == 0 && len(attachments) == 0 {
		return
	}

	comment, err := issue_service.CreateIssueComment(ctx, ctx.Doer, ctx.Repo.Repository, issue, form.Content, attachments)
	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSONError(ctx.Tr("repo.issues.comment.blocked_user"))
		} else {
			ctx.ServerError("CreateIssueComment", err)
		}
		return
	}

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
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

	oldContent := comment.Content
	newContent := ctx.FormString("content")
	contentVersion := ctx.FormInt("content_version")

	// allow to save empty content
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
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository)
		renderedContent, err = markdown.RenderString(rctx, comment.Content)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
	} else {
		contentEmpty := fmt.Sprintf(`<span class="no-content">%s</span>`, ctx.Tr("repo.issues.no_content"))
		renderedContent = template.HTML(contentEmpty)
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
