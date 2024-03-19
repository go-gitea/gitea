// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
)

// UpdateAttachments update attachments by UUIDs for the comment
func UpdateAttachments(ctx context.Context, c *Comment, uuids []string) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, uuids)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", uuids, err)
	}
	for i := 0; i < len(attachments); i++ {
		attachments[i].IssueID = c.IssueID
		attachments[i].CommentID = c.ID
		if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
		}
	}
	return committer.Commit()
}

// ClearCommentAttaches clear attachments' comment information by UUIDs for the comment
func ClearCommentAttaches(ctx context.Context, comment *Comment, uuids []string) error {
	_, err := db.GetEngine(ctx).
		Where("issue_id = ?", comment.IssueID).
		And("comment_id = ?", comment.ID).
		In("uuid", uuids).
		Cols("issue_id", "comment_id").Update(&repo_model.Attachment{})
	return err
}
