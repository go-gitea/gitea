// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// CreateReview creates a new review based on opts
func CreateReview(opts models.CreateReviewOptions) (*models.Review, error) {
	review, err := models.CreateReview(opts)
	if err != nil {
		return nil, err
	}

	return review, reviewHook(review)
}

// UpdateReview updates a review
func UpdateReview(review *models.Review) error {
	err := models.UpdateReview(review)
	if err != nil {
		return err
	}

	return reviewHook(review)
}

func reviewHook(review *models.Review) error {
	var reviewHookType models.HookEventType

	switch review.Type {
	case models.ReviewTypeApprove:
		reviewHookType = models.HookEventPullRequestApproved
	case models.ReviewTypeComment:
		reviewHookType = models.HookEventPullRequestComment
	case models.ReviewTypeReject:
		reviewHookType = models.HookEventPullRequestRejected
	default:
		// unsupported review webhook type here
		return nil
	}

	pr := review.Issue.PullRequest

	if err := pr.LoadIssue(); err != nil {
		return err
	}

	mode, err := models.AccessLevel(review.Issue.Poster, review.Issue.Repo)
	if err != nil {
		return err
	}

	if err := models.PrepareWebhooks(review.Issue.Repo, reviewHookType, &api.PullRequestPayload{
		Action:      api.HookIssueSynchronized,
		Index:       review.Issue.Index,
		PullRequest: pr.APIFormat(),
		Repository:  review.Issue.Repo.APIFormat(mode),
		Sender:      review.Reviewer.APIFormat(),
	}); err != nil {
		return err
	}
	go models.HookQueue.Add(review.Issue.Repo.ID)

	return nil
}
