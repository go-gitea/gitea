// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
)

type (
	Reaction             = repo_model.Reaction
	ReactionList         = repo_model.ReactionList
	ReactionOptions      = repo_model.ReactionOptions
	FindReactionsOptions = repo_model.FindReactionsOptions
)

type ErrForbiddenIssueReaction = repo_model.ErrForbiddenReaction

func IsErrForbiddenIssueReaction(err error) bool {
	return repo_model.IsErrForbiddenReaction(err)
}

type ErrReactionAlreadyExist = repo_model.ErrReactionAlreadyExist

func IsErrReactionAlreadyExist(err error) bool {
	return repo_model.IsErrReactionAlreadyExist(err)
}

// FindCommentReactions returns a ReactionList of all reactions from an comment
func FindCommentReactions(ctx context.Context, issueID, commentID int64) (ReactionList, int64, error) {
	return repo_model.FindReactions(ctx, repo_model.FindReactionsOptions{
		IssueID:   issueID,
		CommentID: commentID,
	})
}

// FindIssueReactions returns a ReactionList of all reactions from an issue
func FindIssueReactions(ctx context.Context, issueID int64, listOptions db.ListOptions) (ReactionList, int64, error) {
	return repo_model.FindReactions(ctx, repo_model.FindReactionsOptions{
		ListOptions: listOptions,
		IssueID:     issueID,
		CommentID:   -1,
	})
}

// FindReactions returns a ReactionList of all reactions from an issue or a comment
func FindReactions(ctx context.Context, opts FindReactionsOptions) (ReactionList, int64, error) {
	return repo_model.FindReactions(ctx, opts)
}

// CreateReaction creates reaction for issue or comment.
func CreateReaction(ctx context.Context, opts *ReactionOptions) (*Reaction, error) {
	return repo_model.CreateReaction(ctx, opts)
}

// DeleteReaction deletes reaction for issue or comment.
func DeleteReaction(ctx context.Context, opts *ReactionOptions) error {
	return repo_model.DeleteReaction(ctx, opts)
}

// DeleteIssueReaction deletes a reaction on issue.
func DeleteIssueReaction(ctx context.Context, doerID, issueID int64, content string) error {
	return repo_model.DeleteReaction(ctx, &repo_model.ReactionOptions{
		Type:      content,
		DoerID:    doerID,
		IssueID:   issueID,
		CommentID: -1,
	})
}

// DeleteCommentReaction deletes a reaction on comment.
func DeleteCommentReaction(ctx context.Context, doerID, issueID, commentID int64, content string) error {
	return repo_model.DeleteReaction(ctx, &repo_model.ReactionOptions{
		Type:      content,
		DoerID:    doerID,
		IssueID:   issueID,
		CommentID: commentID,
	})
}
