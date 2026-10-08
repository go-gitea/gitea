// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"

	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/log"
)

// creatorCanManageWebhook reports whether the user who configured the webhook may still manage it,
// so that revoking someone's access also stops the deliveries they set up.
func creatorCanManageWebhook(ctx context.Context, w *webhook_model.Webhook, repo *repo_model.Repository) (bool, error) {
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
		if repo == nil || repo.ID != w.RepoID {
			if repo, err = repo_model.GetRepositoryByID(ctx, w.RepoID); err != nil {
				return false, err
			}
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

// filterRevokedWebhooks deactivates and drops the webhooks whose creator lost the right to manage them.
func filterRevokedWebhooks(ctx context.Context, ws []*webhook_model.Webhook, repo *repo_model.Repository) ([]*webhook_model.Webhook, error) {
	kept := ws[:0]
	for _, w := range ws {
		ok, err := creatorCanManageWebhook(ctx, w, repo)
		if err != nil {
			return nil, fmt.Errorf("filterRevokedWebhooks: webhook %d: %w", w.ID, err)
		}
		if ok {
			kept = append(kept, w)
			continue
		}
		log.Info("Deactivating webhook %d: its creator %d can no longer manage it", w.ID, w.CreatedByID)
		if err := webhook_model.DeactivateWebhook(ctx, w.ID); err != nil {
			return nil, fmt.Errorf("filterRevokedWebhooks: deactivate webhook %d: %w", w.ID, err)
		}
	}
	return kept, nil
}
