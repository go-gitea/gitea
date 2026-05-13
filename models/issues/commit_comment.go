// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

// CommitComment is a junction table linking a commit (repo + SHA) to
// a Comment entry. The comment content, tree_path, line, poster, etc.
// are stored in the Comment table with Type = CommentTypeCommitComment.
type CommitComment struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX NOT NULL"`
	CommitSHA string `xorm:"VARCHAR(64) INDEX NOT NULL"`
	CommentID int64  `xorm:"UNIQUE NOT NULL"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

// FileCommitComments holds commit comments for a single file,
// split by side (left = old, right = new) with int keys matching DiffLine indices.
type FileCommitComments struct {
	Left  map[int][]*Comment
	Right map[int][]*Comment
}

// CommitCommentsForDiff maps file paths to their commit comments.
type CommitCommentsForDiff map[string]*FileCommitComments

// FindCommitCommentsByCommitSHA returns all comments for a given commit in a repo.
func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) ([]*Comment, error) {
	var commentIDs []int64
	if err := db.GetEngine(ctx).Cols("comment_id").Table("commit_comment").
		Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		Find(&commentIDs); err != nil {
		return nil, err
	}

	if len(commentIDs) == 0 {
		return nil, nil
	}

	comments := make([]*Comment, 0, len(commentIDs))
	if err := db.GetEngine(ctx).
		In("id", commentIDs).
		OrderBy("created_unix ASC").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := CommentList(comments).LoadPosters(ctx); err != nil {
		return nil, err
	}

	return comments, nil
}

// FindCommitCommentsForDiff returns comments grouped by path and side for rendering in a diff view.
func FindCommitCommentsForDiff(ctx context.Context, repoID int64, commitSHA string) (CommitCommentsForDiff, error) {
	comments, err := FindCommitCommentsByCommitSHA(ctx, repoID, commitSHA)
	if err != nil {
		return nil, err
	}

	result := make(CommitCommentsForDiff)
	for _, c := range comments {
		fcc, ok := result[c.TreePath]
		if !ok {
			fcc = &FileCommitComments{
				Left:  make(map[int][]*Comment),
				Right: make(map[int][]*Comment),
			}
			result[c.TreePath] = fcc
		}
		if c.Line < 0 {
			idx := int(-c.Line)
			fcc.Left[idx] = append(fcc.Left[idx], c)
		} else {
			idx := int(c.Line)
			fcc.Right[idx] = append(fcc.Right[idx], c)
		}
	}
	return result, nil
}

// CreateCommitComment creates a Comment with type CommitComment and a
// corresponding CommitComment junction record, within a transaction.
func CreateCommitComment(ctx context.Context, repoID int64, commitSHA string, comment *Comment) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Insert(comment); err != nil {
			return err
		}

		ref := &CommitComment{
			RepoID:    repoID,
			CommitSHA: commitSHA,
			CommentID: comment.ID,
		}
		_, err := db.GetEngine(ctx).Insert(ref)
		return err
	})
}

// DeleteCommitComment deletes both the junction record and the Comment entry.
func DeleteCommitComment(ctx context.Context, commentID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("comment_id = ?", commentID).Delete(&CommitComment{}); err != nil {
			return err
		}
		_, err := db.GetEngine(ctx).ID(commentID).Delete(&Comment{})
		return err
	})
}

// GetCommitCommentByID returns a commit comment by loading the Comment entry,
// verifying it belongs to the given repository via the junction table.
func GetCommitCommentByID(ctx context.Context, repoID, commentID int64) (*Comment, error) {
	exists, err := db.GetEngine(ctx).Table("commit_comment").
		Where("repo_id = ? AND comment_id = ?", repoID, commentID).
		Exist()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, db.ErrNotExist{Resource: "CommitComment", ID: commentID}
	}

	c := &Comment{}
	has, err := db.GetEngine(ctx).ID(commentID).Get(c)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{Resource: "CommitComment", ID: commentID}
	}
	return c, nil
}
