// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
)

// GetSystemWebhook returns admin default webhook by given ID.
func GetAdminWebhook(ctx context.Context, id int64, isSystemWebhook bool) (*Webhook, error) {
	webhook := &Webhook{ID: id}
	has, err := db.GetEngine(ctx).
		Where("repo_id=? AND owner_id=? AND is_system_webhook=?", 0, 0, isSystemWebhook).
		Get(webhook)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{ID: id}
	}
	return webhook, nil
}

// returns all admin system or default webhooks.
// isSystemWebhook == true gives system webhooks, otherwise gives default webhooks.
// isActive filters system webhooks to those currently enabled or disabled; pass util.OptionalBoolNone to get both.
// isActive is ignored when requesting default webhooks.
func GetAdminWebhooks(ctx context.Context, isSystemWebhook bool, isActive util.OptionalBool) ([]*Webhook, error) {
	if !isSystemWebhook {
		isActive = util.OptionalBoolNone
	}
	webhooks := make([]*Webhook, 0, 5)
	if isActive.IsNone() {
		return webhooks, db.GetEngine(ctx).
			Where("repo_id=? AND owner_id=? AND is_system_webhook=?", 0, 0, isSystemWebhook).
			Find(&webhooks)
	}
	return webhooks, db.GetEngine(ctx).
		Where("repo_id=? AND owner_id=? AND is_system_webhook=? AND is_active = ?", 0, 0, isSystemWebhook, isActive.IsTrue()).
		Find(&webhooks)
}

// DeleteWebhook deletes an admin-configured default or system webhook (where Org and Repo ID both 0)
func DeleteAdminWebhook(ctx context.Context, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		count, err := db.GetEngine(ctx).
			Where("repo_id=? AND owner_id=?", 0, 0).
			Delete(&Webhook{ID: id})
		if err != nil {
			return err
		} else if count == 0 {
			return ErrWebhookNotExist{ID: id}
		}

		_, err = db.DeleteByBean(ctx, &HookTask{HookID: id})
		return err
	})
}

// CopyDefaultWebhooksToRepo creates copies of the default webhooks in a new repo
func CopyDefaultWebhooksToRepo(ctx context.Context, repoID int64) error {
	ws, err := GetAdminWebhooks(ctx, false, util.OptionalBoolNone)
	if err != nil {
		return fmt.Errorf("GetDefaultWebhooks: %v", err)
	}

	for _, w := range ws {
		w.ID = 0
		w.RepoID = repoID
		if err := CreateWebhook(ctx, w); err != nil {
			return fmt.Errorf("CreateWebhook: %v", err)
		}
	}
	return nil
}
