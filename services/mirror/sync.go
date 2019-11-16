// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
)

func syncAction(opType models.ActionType, repo *models.Repository, refName string, data []byte) error {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(),
		OpType:    opType,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refName,
		Content:   string(data),
	}); err != nil {
		return fmt.Errorf("notifyWatchers: %v", err)
	}

	return nil
}

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

	opts.Commits.CompareURL = repo.ComposeCompareURL(opts.OldCommitID, opts.NewCommitID)

	notification.NotifyPushCommits(repo.MustOwner(), repo, opts.RefName, opts.OldCommitID, opts.NewCommitID, opts.Commits)

	data, err := json.Marshal(opts.Commits)
	if err != nil {
		return err
	}

	return syncAction(models.ActionMirrorSyncPush, repo, opts.RefName, data)
}

// SyncCreateAction adds new action for mirror synchronization of new reference.
func SyncCreateAction(repo *models.Repository, refName string) error {
	return syncAction(models.ActionMirrorSyncCreate, repo, refName, nil)
}

// SyncDeleteAction adds new action for mirror synchronization of delete reference.
func SyncDeleteAction(repo *models.Repository, refName string) error {
	return syncAction(models.ActionMirrorSyncDelete, repo, refName, nil)
}
