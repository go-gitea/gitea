// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	issuesModel "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation/i18n"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// GetContentHistoryOverview get overview
func GetContentHistoryOverview(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if issue == nil {
		return
	}

	lang := ctx.Locale.Language()
	editedHistoryCountMap, _ := issuesModel.QueryIssueContentHistoryEditedCountMap(ctx, issue.ID)
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"i18n": map[string]interface{}{
			"textEdited":                   i18n.Tr(lang, "repo.issues.content_history.edited"),
			"textDeleteFromHistory":        i18n.Tr(lang, "repo.issues.content_history.delete_from_history"),
			"textDeleteFromHistoryConfirm": i18n.Tr(lang, "repo.issues.content_history.delete_from_history_confirm"),
			"textOptions":                  i18n.Tr(lang, "repo.issues.content_history.options"),
		},
		"editedHistoryCountMap": editedHistoryCountMap,
	})
}

// GetContentHistoryList  get list
func GetContentHistoryList(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	commentID := ctx.FormInt64("comment_id")
	if issue == nil {
		return
	}

	items, _ := issuesModel.FetchIssueContentHistoryList(ctx, issue.ID, commentID)

	// render history list to HTML for frontend dropdown items: (name, value)
	// name is HTML of "avatar + userName + userAction + timeSince"
	// value is historyId
	lang := ctx.Locale.Language()
	var results []map[string]interface{}
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
		timeSinceText := timeutil.TimeSinceUnix(item.EditedUnix, lang)

		username := item.UserName
		if setting.UI.DefaultShowFullName && strings.TrimSpace(item.UserFullName) != "" {
			username = strings.TrimSpace(item.UserFullName)
		}

		results = append(results, map[string]interface{}{
			"name": fmt.Sprintf("<img class='ui avatar image' src='%s'><strong>%s</strong> %s %s",
				html.EscapeString(item.UserAvatarLink), html.EscapeString(username), actionText, timeSinceText),
			"value": item.HistoryID,
		})
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// canSoftDeleteContentHistory checks whether current user can soft-delete a history revision
// Admins or owners can always delete history revisions. Normal users can only delete own history revisions.
func canSoftDeleteContentHistory(ctx *context.Context, issue *models.Issue, comment *models.Comment,
	history *issuesModel.ContentHistory,
) bool {
	canSoftDelete := false
	if ctx.Repo.IsOwner() {
		canSoftDelete = true
	} else if ctx.Repo.CanWrite(unit.TypeIssues) {
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
	if issue == nil {
		return
	}

	historyID := ctx.FormInt64("history_id")
	history, prevHistory, err := issuesModel.GetIssueContentHistoryAndPrev(ctx, historyID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, map[string]interface{}{
			"message": "Can not find the content history",
		})
		return
	}

	// get the related comment if this history revision is for a comment, otherwise the history revision is for an issue.
	var comment *models.Comment
	if history.CommentID != 0 {
		var err error
		if comment, err = models.GetCommentByID(ctx, history.CommentID); err != nil {
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

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"canSoftDelete": canSoftDeleteContentHistory(ctx, issue, comment, history),
		"historyId":     historyID,
		"prevHistoryId": prevHistoryID,
		"diffHtml":      diffHTMLBuf.String(),
	})
}

// SoftDeleteContentHistory soft delete
func SoftDeleteContentHistory(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if issue == nil {
		return
	}

	commentID := ctx.FormInt64("comment_id")
	historyID := ctx.FormInt64("history_id")

	var comment *models.Comment
	var history *issuesModel.ContentHistory
	var err error
	if commentID != 0 {
		if comment, err = models.GetCommentByID(ctx, commentID); err != nil {
			log.Error("can not get comment for issue content history %v. err=%v", historyID, err)
			return
		}
	}
	if history, err = issuesModel.GetIssueContentHistoryByID(ctx, historyID); err != nil {
		log.Error("can not get issue content history %v. err=%v", historyID, err)
		return
	}

	canSoftDelete := canSoftDeleteContentHistory(ctx, issue, comment, history)
	if !canSoftDelete {
		ctx.JSON(http.StatusForbidden, map[string]interface{}{
			"message": "Can not delete the content history",
		})
		return
	}

	err = issuesModel.SoftDeleteIssueContentHistory(ctx, historyID)
	log.Debug("soft delete issue content history. issue=%d, comment=%d, history=%d", issue.ID, commentID, historyID)
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": err == nil,
	})
}
