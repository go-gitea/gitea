// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
)

// CreateIssueReaction creates a reaction on an issue.
func CreateIssueReaction(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, content string) (*issues_model.Reaction, error) {
	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if user_model.IsUserBlockedBy(ctx, doer, issue.PosterID, issue.Repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}

	return issues_model.CreateReaction(ctx, &issues_model.ReactionOptions{
		Type:    content,
		DoerID:  doer.ID,
		IssueID: issue.ID,
	})
}

// CreateCommentReaction creates a reaction on a comment.
func CreateCommentReaction(ctx context.Context, doer *user_model.User, comment *issues_model.Comment, content string) (*issues_model.Reaction, error) {
	if err := comment.LoadIssue(ctx); err != nil {
		return nil, err
	}

	if err := comment.Issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if user_model.IsUserBlockedBy(ctx, doer, comment.Issue.PosterID, comment.Issue.Repo.OwnerID, comment.PosterID) {
		return nil, user_model.ErrBlockedUser
	}

	return issues_model.CreateReaction(ctx, &issues_model.ReactionOptions{
		Type:      content,
		DoerID:    doer.ID,
		IssueID:   comment.Issue.ID,
		CommentID: comment.ID,
	})
}
