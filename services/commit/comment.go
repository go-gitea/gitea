// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package commit hosts services for things that are anchored to a commit
// rather than to an issue or pull request — most notably, inline / general
// comments on a commit ("commit comments"), wired to the rest of the comment
// infrastructure via a synthetic carrier Issue (see
// models/issues/commit_comment.go for the data model rationale).
package commit

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
)

// CreateCommitCommentOptions captures everything needed to post a single
// comment on a commit. Empty TreePath / zero Line means "general comment on
// the commit as a whole"; non-empty TreePath together with a non-zero Line
// means "inline comment anchored to a specific diff line".
type CreateCommitCommentOptions struct {
	Doer        *user_model.User
	Repo        *repo_model.Repository
	CommitSHA   string
	Content     string
	TreePath    string   // file path within the commit, "" for a general commit comment
	Line        int64    // negative for old-side, positive for new-side, 0 for general
	Attachments []string // attachment UUIDs already uploaded by the user
}

// CreateCommitComment posts a comment on a commit, lazily creating the
// synthetic carrier Issue on first use. Attachments are bound to the new
// comment via the existing UpdateCommentAttachments path (`issue_id` +
// `comment_id` set on each attachment row), which keeps them safe from
// `DeleteOrphanedAttachments` and from the `doctor checkStorage` orphan
// scan.
//
// The existence of the commit is verified up front, so a typo in the SHA
// can't materialize an unreachable carrier Issue.
//
// CommentType selection:
//   - line == 0 && treePath == "" → CommentTypeComment (general)
//   - otherwise                   → CommentTypeCode (inline / line)
//
// Both types already handle attachments inside issues_model.CreateComment ›
// updateCommentInfos, so we don't need to invoke UpdateCommentAttachments
// directly here.
func CreateCommitComment(ctx context.Context, opts CreateCommitCommentOptions) (*issues_model.Comment, error) {
	if opts.Doer == nil {
		return nil, errors.New("nil doer")
	}
	if opts.Repo == nil {
		return nil, errors.New("nil repo")
	}
	if opts.CommitSHA == "" {
		return nil, errors.New("empty commit SHA")
	}
	if opts.Content == "" && len(opts.Attachments) == 0 {
		return nil, errors.New("comment must have content or at least one attachment")
	}

	if user_model.IsUserBlockedBy(ctx, opts.Doer, opts.Repo.OwnerID) {
		if isAdmin, _ := access_model.IsUserRepoAdmin(ctx, opts.Repo, opts.Doer); !isAdmin {
			return nil, user_model.ErrBlockedUser
		}
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, opts.Repo)
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}
	defer closer.Close()

	commit, err := gitRepo.GetCommit(opts.CommitSHA)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, ErrCommitNotFound{SHA: opts.CommitSHA}
		}
		return nil, fmt.Errorf("get commit %s: %w", opts.CommitSHA, err)
	}
	// Normalise to the full 40-char SHA so multiple short-SHA aliases for the
	// same commit collapse onto a single carrier Issue.
	fullSHA := commit.ID.String()

	commentType := issues_model.CommentTypeComment
	if opts.Line != 0 || opts.TreePath != "" {
		commentType = issues_model.CommentTypeCode
	}

	var comment *issues_model.Comment
	err = db.WithTx(ctx, func(ctx context.Context) error {
		carrier, err := issues_model.GetOrCreateCommitCommentIssue(ctx, opts.Repo, fullSHA, opts.Doer)
		if err != nil {
			return fmt.Errorf("get-or-create carrier issue: %w", err)
		}

		c, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
			Type:        commentType,
			Doer:        opts.Doer,
			Repo:        opts.Repo,
			Issue:       carrier,
			CommitSHA:   fullSHA,
			TreePath:    opts.TreePath,
			LineNum:     opts.Line,
			Content:     opts.Content,
			Attachments: opts.Attachments,
		})
		if err != nil {
			return fmt.Errorf("create comment: %w", err)
		}
		comment = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// ListCommitComments returns every comment posted on a given commit, ordered
// by creation time. Returns an empty slice (not nil) when no comments exist
// — callers should treat both as equivalent.
func ListCommitComments(ctx context.Context, repo *repo_model.Repository, commitSHA string) ([]*issues_model.Comment, error) {
	if repo == nil {
		return nil, errors.New("nil repo")
	}
	if commitSHA == "" {
		return nil, errors.New("empty commit SHA")
	}

	carrier, err := issues_model.GetCommitCommentIssue(ctx, repo.ID, commitSHA)
	if err != nil {
		if errors.Is(err, issues_model.ErrCommitCommentIssueNotFound) {
			return []*issues_model.Comment{}, nil
		}
		return nil, err
	}

	comments, err := issues_model.FindComments(ctx, &issues_model.FindCommentsOptions{
		IssueID: carrier.ID,
		Type:    issues_model.CommentTypeUndefined,
	})
	if err != nil {
		return nil, err
	}
	// FindComments returns issues_model.CommentList (= []*Comment); convert
	// explicitly so the return type stays portable for callers that don't
	// pull in the issues_model package's named alias.
	return []*issues_model.Comment(comments), nil
}

// ErrCommitNotFound is returned by CreateCommitComment when the SHA the
// caller supplied doesn't resolve to a commit in the repo. Distinct error
// type so HTTP handlers can map it to 404 rather than 500.
type ErrCommitNotFound struct {
	SHA string
}

func (e ErrCommitNotFound) Error() string {
	return fmt.Sprintf("commit %q not found", e.SHA)
}
