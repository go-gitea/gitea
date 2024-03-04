// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/timeutil"
	notify_service "code.gitea.io/gitea/services/notify"
)

// CreateRefComment creates a commit reference comment to issue.
func CreateRefComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, content, commitSHA string) error {
	if len(commitSHA) == 0 {
		return fmt.Errorf("cannot create reference with empty commit SHA")
	}

	// Check if same reference from same commit has already existed.
	has, err := db.GetEngine(ctx).Get(&issues_model.Comment{
		Type:      issues_model.CommentTypeCommitRef,
		IssueID:   issue.ID,
		CommitSHA: commitSHA,
	})
	if err != nil {
		return fmt.Errorf("check reference comment: %w", err)
	} else if has {
		return nil
	}

	_, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:      issues_model.CommentTypeCommitRef,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		CommitSHA: commitSHA,
		Content:   content,
	})
	return err
}

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, content string, attachments []string) (*issues_model.Comment, error) {
	comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:        issues_model.CommentTypeComment,
		Doer:        doer,
		Repo:        repo,
		Issue:       issue,
		Content:     content,
		Attachments: attachments,
	})
	if err != nil {
		return nil, err
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comment.Content)
	if err != nil {
		return nil, err
	}

	notify_service.CreateIssueComment(ctx, doer, repo, issue, comment, mentions)

	return comment, nil
}

func getDiffAttaches(oldAttaches, newAttaches container.Set[string]) (addedAttaches, delAttaches container.Set[string]) {
	for attach := range newAttaches {
		if !oldAttaches.Contains(attach) {
			addedAttaches.Add(attach)
		}
	}
	for attach := range oldAttaches {
		if !newAttaches.Contains(attach) {
			delAttaches.Add(attach)
		}
	}
	return
}

// UpdateComment updates information of comment.
func UpdateComment(ctx context.Context, c *issues_model.Comment, doer *user_model.User, oldContent string, attachments []string, ignoreUpdateAttachments bool) error {
	needsContentHistory := c.Content != oldContent && c.Type.HasContentSupport()
	if needsContentHistory {
		hasContentHistory, err := issues_model.HasIssueContentHistory(ctx, c.IssueID, c.ID)
		if err != nil {
			return err
		}
		if !hasContentHistory {
			if err = issues_model.SaveIssueContentHistory(ctx, c.PosterID, c.IssueID, c.ID,
				c.CreatedUnix, oldContent, true); err != nil {
				return err
			}
		}
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := issues_model.UpdateComment(ctx, c, doer); err != nil {
			return err
		}

		if !ignoreUpdateAttachments {
			oldAttaches, err := markdown.ParseAttachments(oldContent)
			if err != nil {
				return err
			}

			newAttaches, err := markdown.ParseAttachments(c.Content)
			if err != nil {
				return err
			}

			addedAttaches, delAttaches := getDiffAttaches(oldAttaches, newAttaches)
			for _, attach := range addedAttaches.Values() {
				attachments = append(attachments, attach)
			}

			if len(delAttaches) > 0 {
				if err := issues_model.ClearCommentAttaches(ctx, c, delAttaches.Values()); err != nil {
					return err
				}
			}

			// when the update request doesn't intend to update attachments (eg: change checkbox state), ignore attachment updates
			if err := updateAttachments(ctx, c, attachments); err != nil {
				return err
			}
		}

		if needsContentHistory {
			err := issues_model.SaveIssueContentHistory(ctx, doer.ID, c.IssueID, c.ID, timeutil.TimeStampNow(), c.Content, false)
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	notify_service.UpdateComment(ctx, doer, c, oldContent)

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(ctx context.Context, doer *user_model.User, comment *issues_model.Comment) error {
	err := db.WithTx(ctx, func(ctx context.Context) error {
		return issues_model.DeleteComment(ctx, comment)
	})
	if err != nil {
		return err
	}

	notify_service.DeleteComment(ctx, doer, comment)

	return nil
}
