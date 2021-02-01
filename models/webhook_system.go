// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "fmt"

// GetDefaultWebhooks returns all admin-default webhooks.
func GetDefaultWebhooks() ([]*Webhook, error) {
	return getDefaultWebhooks(x)
}

func getDefaultWebhooks(e Engine) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, e.
		Where("repo_id=? AND org_id=? AND is_system_webhook=?", 0, 0, false).
		Find(&webhooks)
}

// GetSystemOrDefaultWebhook returns admin system or default webhook by given ID.
func GetSystemOrDefaultWebhook(id int64) (*Webhook, error) {
	webhook := &Webhook{ID: id}
	has, err := x.
		Where("repo_id=? AND org_id=?", 0, 0).
		Get(webhook)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{id}
	}
	return webhook, nil
}

// GetSystemWebhooks returns all admin system webhooks.
func GetSystemWebhooks() ([]*Webhook, error) {
	return getSystemWebhooks(x)
}

func getSystemWebhooks(e Engine) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, e.
		Where("repo_id=? AND org_id=? AND is_system_webhook=?", 0, 0, true).
		Find(&webhooks)
}

// DeleteDefaultSystemWebhook deletes an admin-configured default or system webhook (where Org and Repo ID both 0)
func DeleteDefaultSystemWebhook(id int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	count, err := sess.
		Where("repo_id=? AND org_id=?", 0, 0).
		Delete(&Webhook{ID: id})
	if err != nil {
		return err
	} else if count == 0 {
		return ErrWebhookNotExist{ID: id}
	}

	if _, err := sess.Delete(&HookTask{HookID: id}); err != nil {
		return err
	}

	return sess.Commit()
}

// copyDefaultWebhooksToRepo creates copies of the default webhooks in a new repo
func copyDefaultWebhooksToRepo(e Engine, repoID int64) error {
	ws, err := getDefaultWebhooks(e)
	if err != nil {
		return fmt.Errorf("GetDefaultWebhooks: %v", err)
	}

	for _, w := range ws {
		w.ID = 0
		w.RepoID = repoID
		if err := createWebhook(e, w); err != nil {
			return fmt.Errorf("CreateWebhook: %v", err)
		}
	}
	return nil
}
