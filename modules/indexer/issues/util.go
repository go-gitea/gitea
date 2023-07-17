// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

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

	// TBC: MentionIDs ReviewedIDs ReviewRequestedIDs SubscriberIDs

	return &internal.IndexerData{
		ID:                 issue.ID,
		RepoID:             issue.RepoID,
		IsPublic:           !issue.Repo.IsPrivate,
		Title:              issue.Title,
		Content:            issue.Content,
		Comments:           comments,
		IsPull:             issue.IsPull,
		IsClosed:           issue.IsClosed,
		Labels:             labels,
		NoLabel:            len(labels) == 0,
		MilestoneID:        issue.MilestoneID,
		ProjectID:          issue.Project.ID,
		ProjectBoardID:     issue.ProjectBoardID(),
		PosterID:           issue.PosterID,
		AssigneeID:         issue.AssigneeID,
		MentionIDs:         nil,
		ReviewedIDs:        nil,
		ReviewRequestedIDs: nil,
		SubscriberIDs:      nil,
		UpdatedUnix:        issue.UpdatedUnix,
		CreatedUnix:        issue.CreatedUnix,
		DeadlineUnix:       issue.DeadlineUnix,
		CommentCount:       int64(len(issue.Comments)),
	}, true, nil
}
