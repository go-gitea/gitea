// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
)

const dismissReviewOnReRequestMessage = "Review request re-submitted, review dismissed automatically according to repository settings"

type reviewRequestDismisser struct {
	issue                     *issues_model.Issue
	dismissOnReRequestLoaded  bool
	dismissOnReRequestEnabled bool
	reviewsLoaded             bool
	reviewsByReviewerID       map[int64][]*issues_model.Review
	dismissNotifications      []*dismissReviewNotification
}

type dismissReviewNotification struct {
	doer    *user_model.User
	review  *issues_model.Review
	comment *issues_model.Comment
}

func (d *reviewRequestDismisser) dismissReviewsForReviewerIDs(ctx context.Context, doer *user_model.User, reviewerIDs []int64) error {
	reviewerIDSet := make(map[int64]struct{}, len(reviewerIDs))
	for _, reviewerID := range reviewerIDs {
		if reviewerID > 0 {
			reviewerIDSet[reviewerID] = struct{}{}
		}
	}
	if len(reviewerIDSet) == 0 {
		return nil
	}

	enabled, err := d.loadDismissOnReRequestEnabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	if err := d.loadDismissibleReviewsByReviewerID(ctx); err != nil {
		return err
	}

	for reviewerID := range reviewerIDSet {
		reviews := d.reviewsByReviewerID[reviewerID]
		for _, review := range reviews {
			if review.Dismissed {
				continue
			}

			if err := issues_model.DismissReview(ctx, review, true); err != nil {
				return err
			}

			comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
				Doer:     doer,
				Content:  dismissReviewOnReRequestMessage,
				Type:     issues_model.CommentTypeDismissReview,
				ReviewID: review.ID,
				Issue:    review.Issue,
				Repo:     review.Issue.Repo,
			})
			if err != nil {
				return err
			}

			review.Dismissed = true
			comment.Review = review
			comment.Poster = doer
			comment.Issue = review.Issue

			d.dismissNotifications = append(d.dismissNotifications, &dismissReviewNotification{
				doer:    doer,
				review:  review,
				comment: comment,
			})
		}
	}

	return nil
}

func (d *reviewRequestDismisser) loadDismissOnReRequestEnabled(ctx context.Context) (bool, error) {
	if d.dismissOnReRequestLoaded {
		return d.dismissOnReRequestEnabled, nil
	}
	d.dismissOnReRequestLoaded = true

	if d.issue == nil || !d.issue.IsPull {
		d.dismissOnReRequestEnabled = false
		return false, nil
	}

	if d.issue.PullRequest == nil {
		if err := d.issue.LoadPullRequest(ctx); err != nil {
			return false, err
		}
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, d.issue.PullRequest.BaseRepoID, d.issue.PullRequest.BaseBranch)
	if err != nil {
		return false, err
	}

	d.dismissOnReRequestEnabled = pb != nil && pb.DismissApprovalsOnReRequest
	return d.dismissOnReRequestEnabled, nil
}

func (d *reviewRequestDismisser) loadDismissibleReviewsByReviewerID(ctx context.Context) error {
	if d.reviewsLoaded {
		return nil
	}
	d.reviewsLoaded = true

	reviews, err := issues_model.FindReviews(ctx, issues_model.FindReviewOptions{
		ListOptions: db.ListOptionsAll,
		IssueID:     d.issue.ID,
		Types: []issues_model.ReviewType{
			issues_model.ReviewTypeApprove,
			issues_model.ReviewTypeReject,
		},
		Dismissed: optional.Some(false),
	})
	if err != nil {
		return err
	}

	if len(reviews) == 0 {
		return nil
	}

	if err := reviews.LoadIssues(ctx); err != nil {
		return err
	}

	for _, review := range reviews {
		d.reviewsByReviewerID[review.ReviewerID] = append(d.reviewsByReviewerID[review.ReviewerID], review)
	}

	return nil
}
