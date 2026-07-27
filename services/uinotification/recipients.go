// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uinotification

import (
	"context"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
)

// filterRecipientsByRepoAccess drops users who may not read unitType in repo, or who the
// repository owner has blocked. Every notification source funnels through this so a user
// who lost access to a repository cannot keep receiving its notifications.
func filterRecipientsByRepoAccess(ctx context.Context, repo *repo_model.Repository, userIDs []int64, unitType unit.Type) ([]int64, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	users, err := user_model.GetUsersByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	allowed := make([]int64, 0, len(users))
	for _, user := range users {
		if user_model.IsUserBlockedBy(ctx, repo.Owner, user.ID) {
			continue
		}
		perm, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
		if err != nil {
			log.Error("GetIndividualUserRepoPermission [repo: %d, user: %d]: %v", repo.ID, user.ID, err)
			continue
		}
		if !perm.CanRead(unitType) {
			continue
		}
		allowed = append(allowed, user.ID)
	}
	return allowed, nil
}
