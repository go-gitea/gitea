// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/builder"
)

// ConversationStats summarizes the resolvable code review conversations of a pull request.
type ConversationStats struct {
	Total      int
	Resolved   int
	NextOpenID int64 // id of the root comment of the first unresolved conversation, 0 if none
}

// Unresolved returns the number of conversations that are still open.
func (s *ConversationStats) Unresolved() int {
	return s.Total - s.Resolved
}

// GetConversationStats counts the code review conversations of an issue.
// A conversation is identified by file, line and outdated state (replies inherit it from the thread on proposed-side lines only), its first comment is the root and carries the resolved mark.
// Outdated conversations still count (they remain visible and resolvable in the timeline), comments of pending (unpublished) reviews are ignored.
func GetConversationStats(ctx context.Context, issueID int64) (*ConversationStats, error) {
	e := db.GetEngine(ctx)
	pendingReviews := builder.Select("id").From("review").
		Where(builder.Eq{"issue_id": issueID, "type": ReviewTypePending})
	comments := make([]*Comment, 0, 16)
	if err := e.Cols("id", "issue_id", "tree_path", "line", "invalidated", "resolve_doer_id").
		Where(builder.Eq{"issue_id": issueID, "type": CommentTypeCode}).
		And(builder.NotIn("review_id", pendingReviews)).
		Asc("created_unix", "id").
		Find(&comments); err != nil {
		return nil, err
	}

	type key struct {
		path        string
		line        int64
		invalidated bool
	}
	stats := &ConversationStats{}
	seen := make(map[key]struct{}, len(comments))
	for _, c := range comments {
		if c.TreePath != "" && c.Line != 0 {
			k := key{c.TreePath, c.Line, c.Invalidated && c.Line > 0}
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
		}
		stats.Total++
		if c.ResolveDoerID != 0 {
			stats.Resolved++
		} else if stats.NextOpenID == 0 {
			stats.NextOpenID = c.ID
		}
	}
	return stats, nil
}
