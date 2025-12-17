// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"
)

// RenderNewCommitCodeCommentForm renders the form for creating a new commit code comment
func RenderNewCommitCodeCommentForm(ctx *context.Context) {
	commitSHA := ctx.PathParam("sha")

	ctx.Data["PageIsCommitDiff"] = true
	ctx.Data["AfterCommitID"] = commitSHA
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	ctx.HTML(http.StatusOK, tplNewComment)
}

// CreateCommitComment creates a new comment on a commit diff line
func CreateCommitComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CodeCommentForm)

	if ctx.Written() {
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(false) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if form.Content == "" {
		log.Warn("Empty comment content")
		ctx.HTTPError(http.StatusBadRequest, "EmptyCommentContent")
		return
	}

	signedLine := form.Line
	if form.Side == "previous" {
		signedLine *= -1
	}

	var attachments []string
	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	_, err := repo_service.CreateCommitComment(ctx, &repo_service.CreateCommitCommentOptions{
		Repo:        ctx.Repo.Repository,
		Doer:        ctx.Doer,
		CommitSHA:   form.CommitSHA,
		Path:        form.TreePath,
		Line:        signedLine,
		Content:     form.Content,
		Attachments: attachments,
	})
	if err != nil {
		ctx.ServerError("CreateCommitComment", err)
		return
	}

	// Fetch all comments for this line to show the full conversation
	allComments, err := repo_model.FindCommitComments(ctx, repo_model.FindCommitCommentsOptions{
		RepoID:    ctx.Repo.Repository.ID,
		CommitSHA: form.CommitSHA,
		Path:      form.TreePath,
		Line:      signedLine,
	})
	if err != nil {
		ctx.ServerError("FindCommitComments", err)
		return
	}

	// Load and render all comments
	issueComments := make([]*issues_model.Comment, 0, len(allComments))
	for _, cc := range allComments {
		if err := cc.LoadPoster(ctx); err != nil {
			ctx.ServerError("LoadPoster", err)
			return
		}
		if err := cc.LoadAttachments(ctx); err != nil {
			ctx.ServerError("LoadAttachments", err)
			return
		}
		if err := repo_service.RenderCommitComment(ctx, cc); err != nil {
			ctx.ServerError("RenderCommitComment", err)
			return
		}
		// Load reactions for this comment
		reactions, _, err := issues_model.FindCommentReactions(ctx, 0, cc.ID)
		if err != nil {
			ctx.ServerError("FindCommentReactions", err)
			return
		}
		if _, err := reactions.LoadUsers(ctx, ctx.Repo.Repository); err != nil {
			ctx.ServerError("LoadUsers", err)
			return
		}
		cc.Reactions = reactions
		issueComments = append(issueComments, convertCommitCommentToIssueComment(cc))
	}

	// Prepare data for template
	ctx.Data["comments"] = issueComments
	ctx.Data["SignedUserID"] = ctx.Doer.ID
	ctx.Data["SignedUser"] = ctx.Doer
	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}
	ctx.Data["PageIsCommitDiff"] = true
	ctx.Data["AfterCommitID"] = form.CommitSHA

	ctx.HTML(http.StatusOK, tplDiffConversation)
}

// LoadCommitComments loads comments for a commit diff
func LoadCommitComments(ctx *context.Context) {
	commitSHA := ctx.PathParam("sha")
	if commitSHA == "" {
		ctx.HTTPError(http.StatusBadRequest, "Missing commit SHA")
		return
	}

	comments, err := repo_model.FindCommitComments(ctx, repo_model.FindCommitCommentsOptions{
		RepoID:    ctx.Repo.Repository.ID,
		CommitSHA: commitSHA,
	})
	if err != nil {
		ctx.ServerError("FindCommitComments", err)
		return
	}

	// Load posters, attachments, and render comments
	for _, comment := range comments {
		if err := comment.LoadPoster(ctx); err != nil {
			ctx.ServerError("LoadPoster", err)
			return
		}
		if err := comment.LoadAttachments(ctx); err != nil {
			ctx.ServerError("LoadAttachments", err)
			return
		}
		if err := repo_service.RenderCommitComment(ctx, comment); err != nil {
			ctx.ServerError("RenderCommitComment", err)
			return
		}
		// Load reactions for this comment
		reactions, _, err := issues_model.FindCommentReactions(ctx, 0, comment.ID)
		if err != nil {
			ctx.ServerError("FindCommentReactions", err)
			return
		}
		if _, err := reactions.LoadUsers(ctx, ctx.Repo.Repository); err != nil {
			ctx.ServerError("LoadUsers", err)
			return
		}
		comment.Reactions = reactions
	}

	// Group comments by file and line
	commentMap := make(map[string]map[string][]*repo_model.CommitComment)
	for _, comment := range comments {
		if commentMap[comment.TreePath] == nil {
			commentMap[comment.TreePath] = make(map[string][]*repo_model.CommitComment)
		}
		key := comment.DiffSide() + "_" + strconv.FormatUint(comment.UnsignedLine(), 10)
		commentMap[comment.TreePath][key] = append(commentMap[comment.TreePath][key], comment)
	}

	ctx.Data["CommitComments"] = commentMap
	ctx.Data["SignedUserID"] = ctx.Doer.ID
	ctx.Data["SignedUser"] = ctx.Doer
	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}
	ctx.Data["IsCommitComment"] = true
	ctx.Data["AfterCommitID"] = commitSHA

	ctx.JSON(http.StatusOK, map[string]any{
		"ok":       true,
		"comments": commentMap,
	})
}

// UpdateCommitCommentContent updates the content of a commit comment
func UpdateCommitCommentContent(ctx *context.Context) {
	comment, err := repo_model.GetCommitCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if repo_model.IsErrCommitCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommitCommentByID", err)
		}
		return
	}

	if comment.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(errors.New("repo ID mismatch"))
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(false)) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	newContent := ctx.FormString("content")
	contentVersion := ctx.FormInt("content_version")
	if contentVersion != comment.ContentVersion {
		ctx.JSONError(ctx.Tr("repo.comments.edit.already_changed"))
		return
	}

	if newContent != comment.Content {
		oldContent := comment.Content
		comment.Content = newContent

		if err = repo_service.UpdateCommitComment(ctx, comment, contentVersion, ctx.Doer, oldContent); err != nil {
			ctx.ServerError("UpdateCommitComment", err)
			return
		}
	}

	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.ServerError("LoadAttachments", err)
		return
	}

	// when the update request doesn't intend to update attachments (eg: change checkbox state), ignore attachment updates
	if !ctx.FormBool("ignore_attachments") {
		if err := updateCommitCommentAttachments(ctx, comment, ctx.FormStrings("files[]")); err != nil {
			ctx.ServerError("UpdateAttachments", err)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":        string(comment.RenderedContent),
		"contentVersion": comment.ContentVersion,
		"attachments":    renderCommitCommentAttachments(ctx, comment.Attachments, comment.Content),
	})
}

// updateCommitCommentAttachments updates attachments for a commit comment
func updateCommitCommentAttachments(ctx *context.Context, comment *repo_model.CommitComment, uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}

	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, uuids)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", uuids, err)
	}

	for i := range attachments {
		attachments[i].CommentID = comment.ID
		if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
		}
	}

	comment.Attachments = attachments
	return nil
}

// convertCommitCommentToIssueComment converts a single CommitComment to Comment for template compatibility
func convertCommitCommentToIssueComment(cc *repo_model.CommitComment) *issues_model.Comment {
	var reactions issues_model.ReactionList
	if cc.Reactions != nil {
		if r, ok := cc.Reactions.(issues_model.ReactionList); ok {
			reactions = r
		}
	}
	return &issues_model.Comment{
		ID:               cc.ID,
		PosterID:         cc.PosterID,
		Poster:           cc.Poster,
		OriginalAuthor:   cc.OriginalAuthor,
		OriginalAuthorID: cc.OriginalAuthorID,
		TreePath:         cc.TreePath,
		Line:             cc.Line,
		Content:          cc.Content,
		ContentVersion:   cc.ContentVersion,
		RenderedContent:  cc.RenderedContent,
		CreatedUnix:      cc.CreatedUnix,
		UpdatedUnix:      cc.UpdatedUnix,
		Reactions:        reactions,
		Attachments:      cc.Attachments,
	}
}

// DeleteCommitComment deletes a commit comment
func DeleteCommitComment(ctx *context.Context) {
	comment, err := repo_model.GetCommitCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if repo_model.IsErrCommitCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommitCommentByID", err)
		}
		return
	}

	if comment.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(errors.New("repo ID mismatch"))
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(false)) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if err = repo_model.DeleteCommitComment(ctx, comment); err != nil {
		ctx.ServerError("DeleteCommitComment", err)
		return
	}

	ctx.Status(http.StatusOK)
}

// ChangeCommitCommentReaction creates or removes a reaction for a commit comment
func ChangeCommitCommentReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	comment, err := repo_model.GetCommitCommentByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if repo_model.IsErrCommitCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommitCommentByID", err)
		}
		return
	}

	if comment.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(errors.New("repo ID mismatch"))
		return
	}

	if !ctx.IsSigned {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	switch ctx.PathParam("action") {
	case "react":
		// Create reaction using IssueID=0 for commit comments
		reaction, err := issues_model.CreateReaction(ctx, &issues_model.ReactionOptions{
			Type:      form.Content,
			DoerID:    ctx.Doer.ID,
			IssueID:   0, // Use 0 for commit comments
			CommentID: comment.ID,
		})
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) {
				ctx.ServerError("ChangeCommitCommentReaction", err)
				return
			}
			log.Info("CreateReaction: %s", err)
			break
		}
		log.Trace("Reaction for commit comment created: %d/%d/%d", ctx.Repo.Repository.ID, comment.ID, reaction.ID)
	case "unreact":
		if err := issues_model.DeleteCommentReaction(ctx, ctx.Doer.ID, 0, comment.ID, form.Content); err != nil {
			ctx.ServerError("DeleteCommentReaction", err)
			return
		}
		log.Trace("Reaction for commit comment removed: %d/%d", ctx.Repo.Repository.ID, comment.ID)
	default:
		ctx.NotFound(nil)
		return
	}

	// Reload reactions
	reactions, _, err := issues_model.FindCommentReactions(ctx, 0, comment.ID)
	if err != nil {
		log.Info("FindCommentReactions: %s", err)
	}

	// Load reaction users
	if _, err := reactions.LoadUsers(ctx, ctx.Repo.Repository); err != nil {
		log.Info("LoadUsers: %s", err)
	}

	if len(reactions) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.RenderToHTML(tplReactions, map[string]any{
		"ActionURL": fmt.Sprintf("%s/commit-comments/%d/reactions", ctx.Repo.RepoLink, comment.ID),
		"Reactions": reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeCommitCommentReaction.HTMLString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"html": html,
	})
}

// renderCommitCommentAttachments renders attachments HTML for commit comments
func renderCommitCommentAttachments(ctx *context.Context, attachments []*repo_model.Attachment, content string) template.HTML {
	attachHTML, err := ctx.RenderToHTML(templates.TplName("repo/issue/view_content/attachments"), map[string]any{
		"ctxData":     ctx.Data,
		"Attachments": attachments,
		"Content":     content,
	})
	if err != nil {
		ctx.ServerError("renderCommitCommentAttachments.RenderToHTML", err)
		return ""
	}
	return attachHTML
}
