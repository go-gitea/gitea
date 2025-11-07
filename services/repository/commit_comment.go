// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup/markdown"
)

// CreateCommitCommentOptions holds options for creating a commit comment
type CreateCommitCommentOptions struct {
	Repo        *repo_model.Repository
	Doer        *user_model.User
	CommitSHA   string
	Path        string
	Line        int64
	Content     string
	Attachments []string
}

// CreateCommitComment creates a new comment on a commit diff line
func CreateCommitComment(ctx context.Context, opts *CreateCommitCommentOptions) (*repo_model.CommitComment, error) {
	comment := &repo_model.CommitComment{
		RepoID:    opts.Repo.ID,
		CommitSHA: opts.CommitSHA,
		TreePath:  opts.Path,
		Line:      opts.Line,
		Content:   opts.Content,
		PosterID:  opts.Doer.ID,
	}

	if err := repo_model.CreateCommitComment(ctx, comment); err != nil {
		return nil, err
	}

	// Handle attachments
	if len(opts.Attachments) > 0 {
		attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, opts.Attachments)
		if err != nil {
			return nil, fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %w", opts.Attachments, err)
		}
		for i := range attachments {
			attachments[i].CommentID = comment.ID
			if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
				return nil, fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
			}
		}
		comment.Attachments = attachments
	}

	// Load poster for rendering
	if err := comment.LoadPoster(ctx); err != nil {
		return nil, err
	}

	return comment, nil
}

// RenderCommitComment renders the comment content as markdown
func RenderCommitComment(ctx context.Context, comment *repo_model.CommitComment) error {
	if err := comment.LoadRepo(ctx); err != nil {
		return err
	}

	rctx := renderhelper.NewRenderContextRepoComment(ctx, comment.Repo)
	rendered, err := markdown.RenderString(rctx, comment.Content)
	if err != nil {
		return err
	}
	comment.RenderedContent = rendered
	return nil
}

// UpdateCommitComment updates a commit comment
func UpdateCommitComment(ctx context.Context, comment *repo_model.CommitComment, contentVersion int, doer *user_model.User, oldContent string) error {
	if contentVersion != comment.ContentVersion {
		return errors.New("content version mismatch")
	}

	comment.ContentVersion++

	if err := repo_model.UpdateCommitComment(ctx, comment); err != nil {
		return err
	}

	// Re-render the comment
	return RenderCommitComment(ctx, comment)
}
