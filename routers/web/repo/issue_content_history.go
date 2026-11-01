// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"
	"net/http"
	"strings"

	"gitea.dev/models/avatars"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	"gitea.dev/modules/git"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/log"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	issue_service "gitea.dev/services/issue"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type contentHistoryScope struct {
	issue     *issues_model.Issue
	comment   *issues_model.Comment
	commitSHA string
}

func getContentHistoryScope(ctx *context.Context) *contentHistoryScope {
	if ctx.PathParam("sha") == "" {
		issue := GetActionIssue(ctx)
		if ctx.Written() {
			return nil
		}
		return &contentHistoryScope{issue: issue}
	}
	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, ctx.PathParam("sha"))
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommit", err)
		}
		return nil
	}
	return &contentHistoryScope{commitSHA: commit.ID.String()}
}

func (scope *contentHistoryScope) issueID() int64 {
	if scope.issue != nil {
		return scope.issue.ID
	}
	return 0
}

func (scope *contentHistoryScope) loadComment(ctx *context.Context, commentID int64) error {
	if commentID == 0 && scope.issue != nil {
		return nil
	}
	comment, err := issues_model.GetCommentByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment.IssueID != scope.issueID() || scope.issue == nil && !comment.IsCommitComment() {
		return issues_model.ErrCommentNotExist{ID: commentID}
	}
	if scope.issue == nil {
		if err := comment.LoadCommitComment(ctx); err != nil {
			return err
		}
		if comment.CommitComment.RepoID != ctx.Repo.Repository.ID || comment.CommitComment.CommitSHA != scope.commitSHA {
			return issues_model.ErrCommentNotExist{ID: commentID}
		}
	}
	scope.comment = comment
	return nil
}

// GetContentHistoryOverview get overview
func GetContentHistoryOverview(ctx *context.Context) {
	scope := getContentHistoryScope(ctx)
	if scope == nil {
		return
	}

	var editedHistoryCountMap map[int64]int
	var err error
	if scope.issue != nil {
		editedHistoryCountMap, err = issues_model.QueryIssueContentHistoryEditedCountMap(ctx, scope.issue.ID)
	} else {
		editedHistoryCountMap, err = issues_model.QueryCommitContentHistoryEditedCountMap(ctx, ctx.Repo.Repository.ID, scope.commitSHA)
	}
	if err != nil {
		ctx.ServerError("QueryContentHistoryEditedCountMap", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"i18n": map[string]any{
			"textEdited":                   ctx.Tr("repo.issues.content_history.edited"),
			"textDeleteFromHistory":        ctx.Tr("repo.issues.content_history.delete_from_history"),
			"textDeleteFromHistoryConfirm": ctx.Tr("repo.issues.content_history.delete_from_history_confirm"),
			"textOptions":                  ctx.Tr("repo.issues.content_history.options"),
		},
		"editedHistoryCountMap": editedHistoryCountMap,
	})
}

// GetContentHistoryList  get list
func GetContentHistoryList(ctx *context.Context) {
	scope := getContentHistoryScope(ctx)
	if scope == nil {
		return
	}

	commentID := ctx.FormInt64("comment_id")
	if err := scope.loadComment(ctx, commentID); err != nil {
		ctx.NotFound(err)
		return
	}
	items, err := issues_model.FetchIssueContentHistoryList(ctx, scope.issueID(), commentID)
	if err != nil {
		ctx.ServerError("FetchIssueContentHistoryList", err)
		return
	}

	// render history list to HTML for frontend dropdown items: (name, value)
	// name is HTML of "avatar + userName + userAction + timeSince"
	// value is historyId
	var results []map[string]any
	for _, item := range items {
		var actionHTML template.HTML
		if item.IsDeleted {
			actionHTML = htmlutil.HTMLFormat(`<i data-history-is-deleted="1">%s</i>`, ctx.Locale.TrString("repo.issues.content_history.deleted"))
		} else if item.IsFirstCreated {
			actionHTML = ctx.Locale.Tr("repo.issues.content_history.created")
		} else {
			actionHTML = ctx.Locale.Tr("repo.issues.content_history.edited")
		}

		var fullNameHTML template.HTML
		userName, fullName := item.UserName, strings.TrimSpace(item.UserFullName)
		if fullName != "" {
			fullNameHTML = htmlutil.HTMLFormat(` (<span class="tw-inline-flex tw-max-w-[160px]"><span class="gt-ellipsis">%s</span></span>)`, fullName)
		}

		avatarHTML := templates.AvatarHTML(item.UserAvatarLink, 24, avatars.DefaultAvatarClass+" tw-mr-2", userName)
		timeSinceHTML := templates.TimeSince(item.EditedUnix)
		results = append(results, map[string]any{
			"name":  htmlutil.HTMLFormat("%s <strong>%s</strong>%s %s %s", avatarHTML, userName, fullNameHTML, actionHTML, timeSinceHTML),
			"value": item.HistoryID,
		})
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"results": results,
	})
}

// canSoftDeleteContentHistory checks whether current user can soft-delete a history revision
// Admins or owners can always delete history revisions. Normal users can only delete own history revisions.
func canSoftDeleteContentHistory(ctx *context.Context, issue *issues_model.Issue, comment *issues_model.Comment,
	history *issues_model.ContentHistory,
) (canSoftDelete bool) {
	if issue == nil {
		return ctx.Doer != nil && (ctx.Doer.ID == history.PosterID || issue_service.CanSelfChangeOrAccessComment(ctx.Doer, ctx.Repo.Permission, comment, perm.AccessModeWrite))
	}
	// CanWrite means the doer can manage the issue/PR list
	if ctx.Repo.Permission.IsOwner() || ctx.Repo.Permission.CanWriteIssuesOrPulls(issue.IsPull) {
		canSoftDelete = true
	} else if ctx.Doer != nil {
		// for read-only users, they could still post issues or comments,
		// they should be able to delete the history related to their own issue/comment, a case is:
		// 1. the user posts some sensitive data
		// 2. then the repo owner edits the post but didn't remove the sensitive data
		// 3. the poster wants to delete the edited history revision
		if comment == nil {
			// the issue poster or the history poster can soft-delete
			canSoftDelete = ctx.Doer.ID == issue.PosterID || ctx.Doer.ID == history.PosterID
			canSoftDelete = canSoftDelete && (history.IssueID == issue.ID)
		} else {
			// the comment poster or the history poster can soft-delete
			canSoftDelete = ctx.Doer.ID == comment.PosterID || ctx.Doer.ID == history.PosterID
			canSoftDelete = canSoftDelete && (history.IssueID == issue.ID)
			canSoftDelete = canSoftDelete && (history.CommentID == comment.ID)
		}
	}
	return canSoftDelete
}

func diffContentHistory(oldContent, newContent string) template.HTML {
	// compare the current history revision with the previous one
	dmp := diffmatchpatch.New()
	// `checklines=false` makes better diff result
	diff := dmp.DiffMain(util.NormalizeStringEOL(oldContent), util.NormalizeStringEOL(newContent), false)
	diff = dmp.DiffCleanupEfficiency(diff)

	// use chroma to render the diff html
	buf := &htmlutil.HTMLBuilder{}
	buf.WriteHTML(`<pre class="chroma">`)
	for _, it := range diff {
		switch it.Type {
		case diffmatchpatch.DiffInsert:
			buf.WriteHTML(`<span class="gi">`)
			buf.WriteString(it.Text)
			buf.WriteHTML("</span>")
		case diffmatchpatch.DiffDelete:
			buf.WriteHTML(`<span class="gd">`)
			buf.WriteString(it.Text)
			buf.WriteHTML("</span>")
		default:
			buf.WriteString(it.Text)
		}
	}
	buf.WriteHTML("</pre>")
	return buf.HTMLString()
}

// GetContentHistoryDetail get detail
func GetContentHistoryDetail(ctx *context.Context) {
	scope := getContentHistoryScope(ctx)
	if scope == nil {
		return
	}

	historyID := ctx.FormInt64("history_id")
	history, prevHistory, err := issues_model.GetIssueContentHistoryAndPrev(ctx, scope.issueID(), historyID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, map[string]any{
			"message": "Can not find the content history",
		})
		return
	}

	// get the related comment if this history revision is for a comment, otherwise the history revision is for an issue.
	if err := scope.loadComment(ctx, history.CommentID); err != nil {
		ctx.NotFound(err)
		return
	}

	// get the previous history revision (if exists)
	var prevHistoryID int64
	var prevHistoryContentText string
	if prevHistory != nil {
		prevHistoryID = prevHistory.ID
		prevHistoryContentText = prevHistory.ContentText
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"canSoftDelete": canSoftDeleteContentHistory(ctx, scope.issue, scope.comment, history),
		"historyId":     historyID,
		"prevHistoryId": prevHistoryID,
		"diffHtml":      diffContentHistory(prevHistoryContentText, history.ContentText),
	})
}

// SoftDeleteContentHistory soft delete
func SoftDeleteContentHistory(ctx *context.Context) {
	scope := getContentHistoryScope(ctx)
	if scope == nil {
		return
	}
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}

	commentID := ctx.FormInt64("comment_id")
	historyID := ctx.FormInt64("history_id")

	history, err := issues_model.GetIssueContentHistoryByID(ctx, historyID)
	if err != nil {
		ctx.NotFound(err)
		return
	}
	if history.IssueID != scope.issueID() || history.CommentID != commentID {
		ctx.NotFound(issues_model.ErrCommentNotExist{})
		return
	}
	if err := scope.loadComment(ctx, commentID); err != nil {
		ctx.NotFound(err)
		return
	}

	if !canSoftDeleteContentHistory(ctx, scope.issue, scope.comment, history) {
		ctx.JSON(http.StatusForbidden, map[string]any{
			"message": "Can not delete the content history",
		})
		return
	}

	err = issues_model.SoftDeleteIssueContentHistory(ctx, historyID)
	log.Debug("soft delete content history. issue=%d, comment=%d, history=%d", scope.issueID(), commentID, historyID)
	ctx.JSON(http.StatusOK, map[string]any{
		"ok": err == nil,
	})
}
