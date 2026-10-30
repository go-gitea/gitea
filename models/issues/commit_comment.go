// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"strconv"

	"gitea.dev/models/db"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/markup/markdown"

	"xorm.io/builder"
)

// CommitComment scopes a comment of type CommentTypeCommitComment, which has no issue, to its commit
type CommitComment struct {
	CommentID int64  `xorm:"pk"`
	RepoID    int64  `xorm:"INDEX(s) NOT NULL"`
	CommitSHA string `xorm:"VARCHAR(64) INDEX(s) NOT NULL"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

func CreateCommitComment(ctx context.Context, repoID int64, commitSHA string, comment *Comment) error {
	comment.Type = CommentTypeCommitComment
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.Insert(ctx, comment); err != nil {
			return err
		}
		return db.Insert(ctx, &CommitComment{CommentID: comment.ID, RepoID: repoID, CommitSHA: commitSHA})
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
	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}
	for _, comment := range comments {
		var err error
		if comment.RenderedContent, err = markdown.RenderString(renderhelper.NewRenderContextRepoComment(ctx, repo, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(comment.ID, 10),
		}), comment.Content); err != nil {
			return nil, err
		}
	}
	return comments, nil
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

func GetCommitCommentByID(ctx context.Context, repoID int64, commitSHA string, commentID int64) (*Comment, error) {
	comment, has, err := db.Get[Comment](ctx, builder.In("id", builder.Select("comment_id").From("commit_comment").
		Where(builder.Eq{"comment_id": commentID, "repo_id": repoID, "commit_sha": commitSHA})))
	if err == nil && !has {
		err = ErrCommentNotExist{commentID, 0}
	}
	return comment, err
}

func DeleteCommitCommentsByRepoID(ctx context.Context, repoID int64) error {
	if _, err := db.GetEngine(ctx).In("id", builder.Select("comment_id").From("commit_comment").Where(builder.Eq{"repo_id": repoID})).Delete(&Comment{}); err != nil {
		return err
	}
	return db.DeleteBeans(ctx, &CommitComment{RepoID: repoID})
}
