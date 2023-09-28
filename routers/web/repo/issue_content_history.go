// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"html"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/avatars"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// GetContentHistoryOverview get overview
func GetContentHistoryOverview(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	editedHistoryCountMap, _ := issues_model.QueryIssueContentHistoryEditedCountMap(ctx, issue.ID)
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
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	commentID := ctx.FormInt64("comment_id")
	items, _ := issues_model.FetchIssueContentHistoryList(ctx, issue.ID, commentID)

	// render history list to HTML for frontend dropdown items: (name, value)
	// name is HTML of "avatar + userName + userAction + timeSince"
	// value is historyId
	var results []map[string]any
	for _, item := range items {
		var actionText string
		if item.IsDeleted {
			actionTextDeleted := ctx.Locale.Tr("repo.issues.content_history.deleted")
			actionText = "<i data-history-is-deleted='1'>" + actionTextDeleted + "</i>"
		} else if item.IsFirstCreated {
			actionText = ctx.Locale.Tr("repo.issues.content_history.created")
		} else {
			actionText = ctx.Locale.Tr("repo.issues.content_history.edited")
		}

		username := item.UserName
		if setting.UI.DefaultShowFullName && strings.TrimSpace(item.UserFullName) != "" {
			username = strings.TrimSpace(item.UserFullName)
		}

		src := html.EscapeString(item.UserAvatarLink)
		class := avatars.DefaultAvatarClass + " gt-mr-3"
		name := html.EscapeString(username)
		avatarHTML := string(templates.AvatarHTML(src, 28, class, username))
		timeSinceText := string(timeutil.TimeSinceUnix(item.EditedUnix, ctx.Locale))

		results = append(results, map[string]any{
			"name":  avatarHTML + "<strong>" + name + "</strong> " + actionText + " " + timeSinceText,
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
	// CanWrite means the doer can manage the issue/PR list
	if ctx.Repo.IsOwner() || ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		canSoftDelete = true
	} else {
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

// GetContentHistoryDetail get detail
func GetContentHistoryDetail(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	historyID := ctx.FormInt64("history_id")
	history, prevHistory, err := issues_model.GetIssueContentHistoryAndPrev(ctx, historyID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, map[string]any{
			"message": "Can not find the content history",
		})
		return
	}

	// get the related comment if this history revision is for a comment, otherwise the history revision is for an issue.
	var comment *issues_model.Comment
	if history.CommentID != 0 {
		var err error
		if comment, err = issues_model.GetCommentByID(ctx, history.CommentID); err != nil {
			log.Error("can not get comment for issue content history %v. err=%v", historyID, err)
			return
		}
	}

	// get the previous history revision (if exists)
	var prevHistoryID int64
	var prevHistoryContentText string
	if prevHistory != nil {
		prevHistoryID = prevHistory.ID
		prevHistoryContentText = prevHistory.ContentText
	}

	// compare the current history revision with the previous one
	dmp := diffmatchpatch.New()
	// `checklines=false` makes better diff result
	diff := dmp.DiffMain(prevHistoryContentText, history.ContentText, false)
	diff = dmp.DiffCleanupEfficiency(diff)

	// use chroma to render the diff html
	diffHTMLBuf := bytes.Buffer{}
	diffHTMLBuf.WriteString("<pre class='chroma' style='tab-size: 4'>")
	for _, it := range diff {
		if it.Type == diffmatchpatch.DiffInsert {
			diffHTMLBuf.WriteString("<span class='gi'>")
			diffHTMLBuf.WriteString(html.EscapeString(it.Text))
			diffHTMLBuf.WriteString("</span>")
		} else if it.Type == diffmatchpatch.DiffDelete {
			diffHTMLBuf.WriteString("<span class='gd'>")
			diffHTMLBuf.WriteString(html.EscapeString(it.Text))
			diffHTMLBuf.WriteString("</span>")
		} else {
			diffHTMLBuf.WriteString(html.EscapeString(it.Text))
		}
	}
	diffHTMLBuf.WriteString("</pre>")

	ctx.JSON(http.StatusOK, map[string]any{
		"canSoftDelete": canSoftDeleteContentHistory(ctx, issue, comment, history),
		"historyId":     historyID,
		"prevHistoryId": prevHistoryID,
		"diffHtml":      diffHTMLBuf.String(),
	})
}

// SoftDeleteContentHistory soft delete
func SoftDeleteContentHistory(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	commentID := ctx.FormInt64("comment_id")
	historyID := ctx.FormInt64("history_id")

	var comment *issues_model.Comment
	var history *issues_model.ContentHistory
	var err error
	if commentID != 0 {
		if comment, err = issues_model.GetCommentByID(ctx, commentID); err != nil {
			log.Error("can not get comment for issue content history %v. err=%v", historyID, err)
			return
		}
	}
	if history, err = issues_model.GetIssueContentHistoryByID(ctx, historyID); err != nil {
		log.Error("can not get issue content history %v. err=%v", historyID, err)
		return
	}

	canSoftDelete := canSoftDeleteContentHistory(ctx, issue, comment, history)
	if !canSoftDelete {
		ctx.JSON(http.StatusForbidden, map[string]any{
			"message": "Can not delete the content history",
		})
		return
	}

	err = issues_model.SoftDeleteIssueContentHistory(ctx, historyID)
	log.Debug("soft delete issue content history. issue=%d, comment=%d, history=%d", issue.ID, commentID, historyID)
	ctx.JSON(http.StatusOK, map[string]any{
		"ok": err == nil,
	})
}
