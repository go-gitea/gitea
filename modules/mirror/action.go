// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"
)

// SyncPushActionOptions mirror synchronization action options.
type SyncPushActionOptions struct {
	RefName     string
	OldCommitID string
	NewCommitID string
	Commits     *models.PushCommits
}

// SyncPushAction adds new action for mirror synchronization of pushed commits.
func SyncPushAction(repo *models.Repository, opts SyncPushActionOptions) error {
	if len(opts.Commits.Commits) > setting.UI.FeedMaxCommitNum {
		opts.Commits.Commits = opts.Commits.Commits[:setting.UI.FeedMaxCommitNum]
	}

	apiCommits := opts.Commits.ToAPIPayloadCommits(repo.HTMLURL())

	opts.Commits.CompareURL = repo.ComposeCompareURL(opts.OldCommitID, opts.NewCommitID)
	apiPusher := repo.MustOwner().APIFormat()
	if err := models.PrepareWebhooks(repo, models.HookEventPush, &api.PushPayload{
		Ref:        opts.RefName,
		Before:     opts.OldCommitID,
		After:      opts.NewCommitID,
		CompareURL: setting.AppURL + opts.Commits.CompareURL,
		Commits:    apiCommits,
		Repo:       repo.APIFormat(models.AccessModeOwner),
		Pusher:     apiPusher,
		Sender:     apiPusher,
	}); err != nil {
		return fmt.Errorf("PrepareWebhooks: %v", err)
	}

	data, err := json.Marshal(opts.Commits)
	if err != nil {
		return err
	}

	notification.NotifyRepoMirrorSync(models.ActionMirrorSyncPush, repo, opts.RefName, data)
	return nil
}

// SyncCreateAction adds new action for mirror synchronization of new reference.
func SyncCreateAction(repo *models.Repository, refName string) error {
	notification.NotifyRepoMirrorSync(models.ActionMirrorSyncCreate, repo, refName, nil)
	return nil
}

// SyncDeleteAction adds new action for mirror synchronization of delete reference.
func SyncDeleteAction(repo *models.Repository, refName string) error {
	notification.NotifyRepoMirrorSync(models.ActionMirrorSyncDelete, repo, refName, nil)
	return nil
}
