// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"

	issues_model "codenest.dev/models/issues"
	repo_model "codenest.dev/models/repo"
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/setting"
)

func AllowedReleaseReactions() []string {
	return setting.UI.ReleaseReactions
}

func CreateReleaseReaction(ctx context.Context, doer *user_model.User, rel *repo_model.Release, content string) (*issues_model.Reaction, error) {
	if err := rel.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if user_model.IsUserBlockedBy(ctx, doer, rel.PublisherID, rel.Repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}

	return issues_model.CreateReaction(ctx, setting.UI.ReleaseReactionsLookup, &issues_model.ReactionOptions{
		Type:      content,
		DoerID:    doer.ID,
		ReleaseID: rel.ID,
	})
}
