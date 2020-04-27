// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPullReview convert a review to api format
func ToPullReview(r *models.Review, doer *models.User) (*api.PullReview, error) {

	if err := r.LoadAttributes(); err != nil {
		return nil, err
	}

	result := &api.PullReview{
		ID:        r.ID,
		Reviewer:  ToUser(r.Reviewer, doer != nil, doer.IsAdmin || doer.ID == r.ReviewerID),
		State:     api.ReviewStateUnknown,
		Body:      r.Content,
		CommitID:  r.CommitID,
		Stale:     r.Stale,
		Official:  r.Official,
		Submitted: r.CreatedUnix.AsTime(),
		PRURL:     r.Issue.HTMLURL(),
	}

	switch r.Type {
	case models.ReviewTypeApprove:
		result.State = api.ReviewStateApproved
	case models.ReviewTypeReject:
		result.State = api.ReviewStateRequestChanges
	case models.ReviewTypeComment:
		result.State = api.ReviewStateComment
	case models.ReviewTypePending:
		result.State = api.ReviewStatePending
	}

	return result, nil
}

// ToPullReviewList convert a list of review to it's api format
func ToPullReviewList(rl []*models.Review, doer *models.User) ([]*api.PullReview, error) {
	result := make([]*api.PullReview, 0, len(rl))
	for i := range rl {
		// show pending reviews only for the user who created them
		if rl[i].Type == models.ReviewTypePending && !(doer.IsAdmin || doer.ID == rl[i].ReviewerID) {
			continue
		}
		r, err := ToPullReview(rl[i], doer)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// ToPullReviewCommentList convert the CodeComments of an review to it's api format
func ToPullReviewCommentList(review *models.Review, doer *models.User) ([]*api.PullReviewComment, error) {
	if err := review.LoadAttributes(); err != nil {
		return nil, err
	}

	apiComments := make([]*api.PullReviewComment, 0, len(review.CodeComments))

	for _, lines := range review.CodeComments {
		for _, comments := range lines {
			for _, comment := range comments {
				apiComment := &api.PullReviewComment{
					ID:           comment.ID,
					Body:         comment.Content,
					Reviewer:     ToUser(review.Reviewer, doer != nil, doer.IsAdmin || doer.ID == review.ReviewerID),
					ReviewID:     review.ID,
					Created:      comment.CreatedUnix.AsTime(),
					Updated:      comment.UpdatedUnix.AsTime(),
					Path:         comment.TreePath,
					CommitID:     comment.CommitSHA,
					OrigCommitID: comment.OldRef,
					DiffHunk:     comment.Patch,
					HTMLURL:      comment.HTMLURL(),
					HTMLPullURL:  review.Issue.APIURL(),
				}

				if comment.Line < 0 {
					apiComment.OldLineNum = comment.UnsignedLine()
				} else {
					apiComment.LineNum = comment.UnsignedLine()
				}
				apiComments = append(apiComments, apiComment)
			}
		}
	}
	return apiComments, nil
}
