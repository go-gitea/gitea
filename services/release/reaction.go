// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"
	"slices"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
)

// GitHub doesn't allow negative reactions on releases
func isNegativeReaction(reaction string) bool {
	return reaction == "-1" || reaction == "confused"
}

func AllowedReleaseReactions() []string {
	return slices.DeleteFunc(slices.Clone(setting.UI.Reactions), isNegativeReaction)
}

func CreateReleaseReaction(ctx context.Context, doer *user_model.User, rel *repo_model.Release, content string) (*issues_model.Reaction, error) {
	if isNegativeReaction(content) {
		return nil, issues_model.ErrForbiddenIssueReaction{Reaction: content}
	}

	if err := rel.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if user_model.IsUserBlockedBy(ctx, doer, rel.PublisherID, rel.Repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}

	return issues_model.CreateReaction(ctx, &issues_model.ReactionOptions{
		Type:      content,
		DoerID:    doer.ID,
		ReleaseID: rel.ID,
	})
}
