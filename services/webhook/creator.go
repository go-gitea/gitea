// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"

	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	webhook_model "gitea.dev/models/webhook"
)

// creatorCanManageWebhook reports whether the user who configured the webhook may still manage it,
// so that revoking someone's access also stops the deliveries they set up.
func creatorCanManageWebhook(ctx context.Context, w *webhook_model.Webhook) (bool, error) {
	if w.CreatedByID == 0 {
		return true, nil // created before creators were recorded
	}
	creator, err := user_model.GetUserByID(ctx, w.CreatedByID)
	if user_model.IsErrUserNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if !creator.IsActive || creator.ProhibitLogin {
		return false, nil
	}
	if creator.IsAdmin {
		return true, nil
	}

	switch {
	case w.RepoID != 0:
		repo, err := repo_model.GetRepositoryByID(ctx, w.RepoID)
		if err != nil {
			return false, err
		}
		perm, err := access_model.GetIndividualUserRepoPermission(ctx, repo, creator)
		if err != nil {
			return false, err
		}
		return perm.IsAdmin(), nil
	case w.OwnerID != 0:
		if w.OwnerID == creator.ID {
			return true, nil
		}
		return organization.IsOrganizationOwner(ctx, w.OwnerID, creator.ID)
	default:
		return false, nil // system and default webhooks are managed by site admins only
	}
}
