// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strconv"

	"gitea.dev/models/db"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/markup/markdown"

	"xorm.io/builder"
)

// CreateCommitCommentOptions defines options for creating an inline comment on a commit.
type CreateCommitCommentOptions struct {
	Doer        *user_model.User
	Repo        *repo_model.Repository
	CommitSHA   string
	Content     string
	TreePath    string
	LineNum     int64 // - previous line / + proposed line, same convention as code comments
	Patch       string
	Attachments []string // UUIDs of attachments
}

// CreateCommitComment creates an inline comment on a commit. Unlike pull request
// code comments it is not bound to an issue/review, so it is scoped by RepoID and
// CommitSHA. Uploaded attachments are bound to the created comment so that they are
// not removed by the orphaned-attachment cleanup.
func CreateCommitComment(ctx context.Context, opts *CreateCommitCommentOptions) (*Comment, error) {
	if opts.Repo == nil {
		return nil, fmt.Errorf("CreateCommitComment: repo is required")
	}
	if opts.CommitSHA == "" {
		return nil, fmt.Errorf("CreateCommitComment: commit sha is required")
	}
	return db.WithTx2(ctx, func(ctx context.Context) (*Comment, error) {
		comment := &Comment{
			Type:      CommentTypeCode,
			PosterID:  opts.Doer.ID,
			Poster:    opts.Doer,
			RepoID:    opts.Repo.ID,
			CommitSHA: opts.CommitSHA,
			Line:      opts.LineNum,
			TreePath:  opts.TreePath,
			Content:   opts.Content,
			Patch:     opts.Patch,
		}
		if err := db.Insert(ctx, comment); err != nil {
			return nil, err
		}
		if err := UpdateCommentAttachments(ctx, comment, opts.Attachments); err != nil {
			return nil, err
		}
		return comment, nil
	})
}

// commitCommentsCond builds the condition selecting inline commit comments for a repo/commit.
func commitCommentsCond(repoID int64, commitSHA string) builder.Cond {
	return builder.Eq{
		"comment.type":       CommentTypeCode,
		"comment.issue_id":   0,
		"comment.repo_id":    repoID,
		"comment.commit_sha": commitSHA,
	}
}

// FetchCommitCodeComments returns a 2d-map: ["Path"]["Line"] = comments at line for
// the given commit, with content rendered and posters/attachments/reactions loaded.
func FetchCommitCodeComments(ctx context.Context, repo *repo_model.Repository, commitSHA string, currentUser *user_model.User) (CodeComments, error) {
	pathToLineToComment := make(CodeComments)

	var comments CommentList
	if err := db.GetEngine(ctx).
		Where(commitCommentsCond(repo.ID, commitSHA)).
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments); err != nil {
		return nil, err
	}

	if len(comments) == 0 {
		return pathToLineToComment, nil
	}

	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}
	if err := comments.LoadAttachments(ctx); err != nil {
		return nil, err
	}

	for _, comment := range comments {
		if err := comment.LoadReactions(ctx, repo); err != nil {
			return nil, err
		}
		rctx := renderhelper.NewRenderContextRepoComment(ctx, repo, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(comment.ID, 10),
		})
		var err error
		if comment.RenderedContent, err = markdown.RenderString(rctx, comment.Content); err != nil {
			return nil, err
		}

		if pathToLineToComment[comment.TreePath] == nil {
			pathToLineToComment[comment.TreePath] = make(map[int64][]*Comment)
		}
		pathToLineToComment[comment.TreePath][comment.Line] = append(pathToLineToComment[comment.TreePath][comment.Line], comment)
	}
	return pathToLineToComment, nil
}

// GetCommitCommentByID returns an inline commit comment by its ID, ensuring it
// belongs to the given repository and is not bound to an issue.
func GetCommitCommentByID(ctx context.Context, repoID, commentID int64) (*Comment, error) {
	comment := &Comment{}
	has, err := db.GetEngine(ctx).
		Where(builder.Eq{
			"comment.id":       commentID,
			"comment.type":     CommentTypeCode,
			"comment.issue_id": 0,
			"comment.repo_id":  repoID,
		}).Get(comment)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrCommentNotExist{ID: commentID}
	}
	return comment, nil
}

// CountCommitComments counts the inline comments attached to a commit.
func CountCommitComments(ctx context.Context, repo *repo_model.Repository, commitSHA string) (int64, error) {
	return db.GetEngine(ctx).Where(commitCommentsCond(repo.ID, commitSHA)).Count(new(Comment))
}
