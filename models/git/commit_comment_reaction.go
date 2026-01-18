// SPDX-License-Identifier: MIT
package git

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

// CommitCommentReaction represents a reaction on a commit comment
type CommitCommentReaction struct {
	ID               int64              `xorm:"pk autoincr"`
	Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
	CommitCommentID  int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
	User             *user_model.User   `xorm:"-"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
}

func init() {
	db.RegisterModel(new(CommitCommentReaction))
}

// FindCommitCommentReactions returns reactions for a given commit comment
func FindCommitCommentReactions(ctx context.Context, commitCommentID int64) ([]*CommitCommentReaction, error) {
	reactions := make([]*CommitCommentReaction, 0, 10)
	if err := db.GetEngine(ctx).Where("commit_comment_id = ?", commitCommentID).Asc("created_unix").Find(&reactions); err != nil {
		return nil, fmt.Errorf("Find commit comment reactions: %w", err)
	}
	return reactions, nil
}

// CreateCommitCommentReaction creates a reaction for a commit comment
func CreateCommitCommentReaction(ctx context.Context, doer *user_model.User, commitCommentID int64, reactionType string) (*CommitCommentReaction, error) {
	if !setting.UI.ReactionsLookup.Contains(reactionType) {
		return nil, fmt.Errorf("'%s' is not an allowed reaction", reactionType)
	}

	reaction := &CommitCommentReaction{
		Type:            reactionType,
		UserID:          doer.ID,
		CommitCommentID: commitCommentID,
	}

	// Check if exists
	existing := CommitCommentReaction{}
	has, err := db.GetEngine(ctx).Where("commit_comment_id = ? and type = ? and user_id = ?", commitCommentID, reactionType, doer.ID).Get(&existing)
	if err != nil {
		return nil, fmt.Errorf("Find existing commit comment reaction: %w", err)
	}
	if has {
		return &existing, fmt.Errorf("reaction '%s' already exists", reactionType)
	}

	if err := db.Insert(ctx, reaction); err != nil {
		return nil, fmt.Errorf("Insert commit comment reaction: %w", err)
	}
	return reaction, nil
}

// DeleteCommitCommentReaction deletes a reaction for a commit comment
func DeleteCommitCommentReaction(ctx context.Context, doerID, commitCommentID int64, reactionType string) error {
	reaction := &CommitCommentReaction{
		Type:            reactionType,
		UserID:          doerID,
		CommitCommentID: commitCommentID,
	}

	_, err := db.GetEngine(ctx).Delete(reaction)
	return err
}
