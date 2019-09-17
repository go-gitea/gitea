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

	var reviewHookType models.HookEventType

	switch opts.Type {
	case models.ReviewTypeApprove:
		reviewHookType = models.HookEventPullRequestApproved
	case models.ReviewTypeComment:
		reviewHookType = models.HookEventPullRequestComment
	case models.ReviewTypeReject:
		reviewHookType = models.HookEventPullRequestRejected
	default:
		// unsupported review webhook type here
		return review, nil
	}

	pr := opts.Issue.PullRequest

	if err := pr.LoadIssue(); err != nil {
		return nil, err
	}

	mode, err := models.AccessLevel(opts.Issue.Poster, opts.Issue.Repo)
	if err != nil {
		return nil, err
	}

	if err := models.PrepareWebhooks(opts.Issue.Repo, reviewHookType, &api.PullRequestPayload{
		Action:      api.HookIssueSynchronized,
		Index:       opts.Issue.Index,
		PullRequest: pr.APIFormat(),
		Repository:  opts.Issue.Repo.APIFormat(mode),
		Sender:      opts.Reviewer.APIFormat(),
	}); err != nil {
		return nil, err
	}
	go models.HookQueue.Add(opts.Issue.Repo.ID)

	return review, nil
}
