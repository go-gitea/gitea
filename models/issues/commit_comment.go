// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
)

// CommitComment binds a Comment to a line of a commit in a repository. The content lives in the
// comment table, so rendering, deletion and future attachment support keep working as before.
type CommitComment struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX(repo_commit) NOT NULL"`
	CommitSHA string `xorm:"INDEX(repo_commit) NOT NULL"`
	CommentID int64  `xorm:"INDEX UNIQUE NOT NULL"`
}

func (*CommitComment) TableName() string {
	return "commit_comment"
}

type CreateCommitCommentOptions struct {
	Doer      *user_model.User
	RepoID    int64
	CommitSHA string // full sha of the commit the comment is attached to
	TreePath  string
	Line      int64 // 1-based line number in the diff hunk
	Content   string
}

// CreateCommitComment creates the comment and its commit binding in one transaction, a comment
// without a binding is deleted as orphaned content by other cleanup routines.
func CreateCommitComment(ctx context.Context, opts *CreateCommitCommentOptions) (*Comment, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*Comment, error) {
		comment := &Comment{
			Type:        CommentTypeCommitComment,
			PosterID:    opts.Doer.ID,
			Poster:      opts.Doer,
			CommitSHA:   opts.CommitSHA,
			TreePath:    opts.TreePath,
			Line:        opts.Line,
			Content:     opts.Content,
			CreatedUnix: timeutil.TimeStampNow(),
		}
		if err := db.Insert(ctx, comment); err != nil {
			return nil, err
		}

		if err := db.Insert(ctx, &CommitComment{
			RepoID:    opts.RepoID,
			CommitSHA: opts.CommitSHA,
			CommentID: comment.ID,
		}); err != nil {
			return nil, err
		}
		return comment, nil
	})
}

// FindCommitCommentsByCommitSHA returns the comments of a commit ordered by creation order. repoID
// is required so a comment can never leak into another repository that happens to contain the sha.
func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) (CommentList, error) {
	e := db.GetEngine(ctx)
	commentIDs := make([]int64, 0, 10)
	if err := e.Table("commit_comment").
		Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		Asc("id").
		Cols("comment_id").
		Find(&commentIDs); err != nil {
		return nil, err
	}
	if len(commentIDs) == 0 {
		return nil, nil
	}

	comments := make(CommentList, 0, len(commentIDs))
	if err := e.Where("type = ?", CommentTypeCommitComment).
		In("id", commentIDs).
		Asc("id").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}
	return comments, nil
}

// GetCommitCommentByID returns a commit comment, only if it belongs to the given repository.
func GetCommitCommentByID(ctx context.Context, repoID, commentID int64) (*Comment, error) {
	e := db.GetEngine(ctx)
	has, err := e.Where("repo_id = ? AND comment_id = ?", repoID, commentID).Exist(new(CommitComment))
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{ID: commentID}
	}

	comment := &Comment{}
	has, err = e.ID(commentID).Get(comment)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{ID: commentID}
	}
	return comment, nil
}

// DeleteCommitComment deletes a commit comment and its binding.
func DeleteCommitComment(ctx context.Context, comment *Comment) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.DeleteByBean(ctx, &CommitComment{CommentID: comment.ID}); err != nil {
			return err
		}
		return DeleteComment(ctx, comment)
	})
}

// DeleteCommitCommentsByRepoID removes all commit comments of a repository, they are not reachable
// through an issue so the regular issue cleanup does not catch them.
func DeleteCommitCommentsByRepoID(ctx context.Context, repoID int64) error {
	e := db.GetEngine(ctx)
	commentIDs := make([]int64, 0, 10)
	if err := e.Table("commit_comment").
		Where("repo_id = ?", repoID).
		Cols("comment_id").
		Find(&commentIDs); err != nil {
		return err
	}
	if len(commentIDs) == 0 {
		return nil
	}

	if _, err := e.Where("repo_id = ?", repoID).Delete(new(CommitComment)); err != nil {
		return err
	}
	_, err := e.In("id", commentIDs).Delete(new(Comment))
	return err
}
