// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
)

// getIssueIndexerData returns the indexer data of an issue and a bool value indicating whether the issue exists.
func getIssueIndexerData(ctx context.Context, issueID int64) (*internal.IndexerData, bool, error) {
	issue, err := issue_model.GetIssueByID(ctx, issueID)
	if err != nil {
		if issue_model.IsErrIssueNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// FIXME: what if users want to search for a review comment of a pull request?
	//        The comment type is CommentTypeCode or CommentTypeReview.
	//        But LoadDiscussComments only loads CommentTypeComment.
	if err := issue.LoadDiscussComments(ctx); err != nil {
		return nil, false, err
	}

	comments := make([]string, 0, len(issue.Comments))
	for _, comment := range issue.Comments {
		if comment.Content != "" {
			// what ever the comment type is, index the content if it is not empty.
			comments = append(comments, comment.Content)
		}
	}

	if err := issue.LoadAttributes(ctx); err != nil {
		return nil, false, err
	}

	labels := make([]int64, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.ID)
	}

	mentionIDs, err := issue_model.GetIssueMentionIDs(ctx, issueID)
	if err != nil {
		return nil, false, err
	}

	var (
		reviewedIDs        []int64
		reviewRequestedIDs []int64
	)
	{
		reviews, err := issue_model.FindReviews(ctx, issue_model.FindReviewOptions{
			ListOptions: db.ListOptions{
				ListAll: true,
			},
			IssueID:      issueID,
			OfficialOnly: false,
		})
		if err != nil {
			return nil, false, err
		}

		reviewedIDsSet := make(container.Set[int64], len(reviews))
		reviewRequestedIDsSet := make(container.Set[int64], len(reviews))
		for _, review := range reviews {
			if review.Type == issue_model.ReviewTypeRequest {
				reviewRequestedIDsSet.Add(review.ReviewerID)
			} else {
				reviewedIDsSet.Add(review.ReviewerID)
			}
		}
		reviewedIDs = reviewedIDsSet.Values()
		reviewRequestedIDs = reviewRequestedIDsSet.Values()
	}

	subscriberIDs, err := issue_model.GetIssueWatchersIDs(ctx, issue.ID, true)
	if err != nil {
		return nil, false, err
	}

	var projectID int64
	if issue.Project != nil {
		projectID = issue.Project.ID
	}

	return &internal.IndexerData{
		ID:                 issue.ID,
		RepoID:             issue.RepoID,
		IsPublic:           !issue.Repo.IsPrivate,
		Title:              issue.Title,
		Content:            issue.Content,
		Comments:           comments,
		IsPull:             issue.IsPull,
		IsClosed:           issue.IsClosed,
		LabelIDs:           labels,
		NoLabel:            len(labels) == 0,
		MilestoneID:        issue.MilestoneID,
		ProjectID:          projectID,
		ProjectBoardID:     issue.ProjectBoardID(ctx),
		PosterID:           issue.PosterID,
		AssigneeID:         issue.AssigneeID,
		MentionIDs:         mentionIDs,
		ReviewedIDs:        reviewedIDs,
		ReviewRequestedIDs: reviewRequestedIDs,
		SubscriberIDs:      subscriberIDs,
		UpdatedUnix:        issue.UpdatedUnix,
		CreatedUnix:        issue.CreatedUnix,
		DeadlineUnix:       issue.DeadlineUnix,
		CommentCount:       int64(len(issue.Comments)),
	}, true, nil
}

func updateRepoIndexer(ctx context.Context, repoID int64) error {
	ids, err := issue_model.GetIssueIDsByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("issue_model.GetIssueIDsByRepoID: %w", err)
	}
	for _, id := range ids {
		if err := updateIssueIndexer(id); err != nil {
			return err
		}
	}
	return nil
}

func updateIssueIndexer(issueID int64) error {
	return pushIssueIndexerQueue(&IndexerMetadata{ID: issueID})
}

func deleteRepoIssueIndexer(ctx context.Context, repoID int64) error {
	var ids []int64
	ids, err := issue_model.GetIssueIDsByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("issue_model.GetIssueIDsByRepoID: %w", err)
	}

	if len(ids) == 0 {
		return nil
	}
	return pushIssueIndexerQueue(&IndexerMetadata{
		IDs:      ids,
		IsDelete: true,
	})
}

func pushIssueIndexerQueue(data *IndexerMetadata) error {
	if issueIndexerQueue == nil {
		// Some unit tests will trigger indexing, but the queue is not initialized.
		// It's OK to ignore it, but log a warning message in case it's not a unit test.
		log.Warn("Trying to push %+v to issue indexer queue, but the queue is not initialized, it's OK if it's a unit test", data)
		return nil
	}

	err := issueIndexerQueue.Push(data)
	if errors.Is(err, queue.ErrAlreadyInQueue) {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		log.Warn("It seems that issue indexer is slow and the queue is full. Please check the issue indexer or increase the queue size.")
	}
	return err
}
