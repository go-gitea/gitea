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

	return &internal.IndexerData{
		ID:       issue.ID,
		RepoID:   issue.RepoID,
		Title:    issue.Title,
		Content:  issue.Content,
		Comments: comments,
	}, true, nil
}
