// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

// CreateReleaseReaction creates a reaction on a release.
func CreateReleaseReaction(ctx context.Context, doer *user_model.User, rel *repo_model.Release, content string) (*repo_model.Reaction, error) {
	if err := rel.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	publisherID := rel.PublisherID
	if publisherID == 0 {
		publisherID = rel.Repo.OwnerID
	}

	if user_model.IsUserBlockedBy(ctx, doer, publisherID, rel.Repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}

	return repo_model.CreateReaction(ctx, &repo_model.ReactionOptions{
		Type:      content,
		DoerID:    doer.ID,
		ReleaseID: rel.ID,
	})
}
