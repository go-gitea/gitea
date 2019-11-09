// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

// CreateReview creates a new review based on opts
func CreateReview(opts models.CreateReviewOptions) (*models.Review, error) {
	review, err := models.CreateReview(opts)
	if err != nil {
		return nil, err
	}

	notification.NotifyPullRequestReview(review.Issue.PullRequest, review, nil)

	return review, nil
}

// UpdateReview updates a review
func UpdateReview(review *models.Review) error {
	err := models.UpdateReview(review)
	if err != nil {
		return err
	}

	notification.NotifyPullRequestReview(review.Issue.PullRequest, review, nil)

	return nil
}
