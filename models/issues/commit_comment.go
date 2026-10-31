// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type CommitComment struct {
	CommentID int64  `xorm:"pk"`
	RepoID    int64  `xorm:"INDEX(s) NOT NULL"`
	CommitSHA string `xorm:"VARCHAR(64) INDEX(s) NOT NULL"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

func CreateCommitComment(ctx context.Context, repoID int64, commitSHA string, comment *Comment, attachmentUUIDs []string) error {
	comment.Type = CommentTypeCommitComment
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.Insert(ctx, comment); err != nil {
			return err
		}
		if err := db.Insert(ctx, &CommitComment{CommentID: comment.ID, RepoID: repoID, CommitSHA: commitSHA}); err != nil {
			return err
		}
		return UpdateCommentAttachments(ctx, comment, attachmentUUIDs)
	})
}

func findCommitComments(ctx context.Context, repo *repo_model.Repository, commitSHA string, cond builder.Cond) (CommentList, error) {
	var comments CommentList
	if err := db.GetEngine(ctx).
		Join("INNER", "commit_comment", "commit_comment.comment_id = comment.id").
		Where(builder.Eq{"commit_comment.repo_id": repo.ID, "commit_comment.commit_sha": commitSHA}.And(cond)).
		Asc("comment.created_unix", "comment.id").
		Find(&comments); err != nil {
		return nil, err
	}
	return comments, comments.loadCodeCommentContent(ctx, repo)
}

func FetchCommitCodeComments(ctx context.Context, repo *repo_model.Repository, commitSHA string, treePaths ...string) (CodeComments, error) {
	comments, err := findCommitComments(ctx, repo, commitSHA, builder.If(len(treePaths) > 0, builder.In("comment.tree_path", treePaths)))
	if err != nil {
		return nil, err
	}
	return groupCodeComments(comments), nil
}

func FetchCommitCodeCommentsByLine(ctx context.Context, repo *repo_model.Repository, commitSHA, treePath string, line int64) (CommentList, error) {
	return findCommitComments(ctx, repo, commitSHA, builder.Eq{"comment.tree_path": treePath, "comment.line": line})
}

func GetCommitCommentPosterIDs(ctx context.Context, repoID int64, commitSHA string) ([]int64, error) {
	var posterIDs []int64
	err := db.GetEngine(ctx).Table("comment").
		Join("INNER", "commit_comment", "commit_comment.comment_id = comment.id").
		Where(builder.Eq{"commit_comment.repo_id": repoID, "commit_comment.commit_sha": commitSHA}).
		Distinct("comment.poster_id").
		Find(&posterIDs)
	return posterIDs, err
}

func GetRepoCommentByID(ctx context.Context, repoID, commentID int64) (*Comment, error) {
	comment, err := GetCommentByID(ctx, commentID)
	if err != nil {
		return nil, err
	}
	commentRepoID, err := comment.getRepoID(ctx)
	if err != nil {
		return nil, err
	}
	if commentRepoID != repoID {
		return nil, ErrCommentNotExist{commentID, 0}
	}
	return comment, nil
}

func (c *Comment) getRepoID(ctx context.Context) (int64, error) {
	if !c.IsCommitComment() {
		if err := c.LoadIssue(ctx); err != nil {
			return 0, err
		}
		return c.Issue.RepoID, nil
	}
	if err := c.LoadCommitComment(ctx); err != nil {
		return 0, err
	}
	return c.CommitComment.RepoID, nil
}

func (c *Comment) LoadCommitComment(ctx context.Context) error {
	if c.CommitComment != nil {
		return nil
	}
	commitComment, has, err := db.GetByID[CommitComment](ctx, c.ID)
	if err != nil {
		return err
	}
	if !has {
		return ErrCommentNotExist{c.ID, 0}
	}
	c.CommitComment = commitComment
	return nil
}

func (c *Comment) UnitType() unit.Type {
	if c.IsCommitComment() {
		return unit.TypeCode
	}
	return util.Iif(c.Issue.IsPull, unit.TypePullRequests, unit.TypeIssues)
}

func DeleteCommitCommentsByRepoID(ctx context.Context, repoID int64) error {
	commentIDs := builder.Select("comment_id").From("commit_comment").Where(builder.Eq{"repo_id": repoID})
	for _, bean := range []any{&Reaction{}, &ContentHistory{}} {
		if _, err := db.GetEngine(ctx).In("comment_id", commentIDs).Delete(bean); err != nil {
			return err
		}
	}
	if _, err := db.GetEngine(ctx).In("id", commentIDs).Delete(&Comment{}); err != nil {
		return err
	}
	return db.DeleteBeans(ctx, &CommitComment{RepoID: repoID})
}
