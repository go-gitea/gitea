// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

func mirrorSyncAction(opType models.ActionType, repo *models.Repository, refName string, data []byte) error {
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

	defer func() {
		go models.HookQueue.Add(repo.ID)
	}()

	return nil
}

// MirrorSyncPushActionOptions mirror synchronization action options.
type MirrorSyncPushActionOptions struct {
	RefName     string
	OldCommitID string
	NewCommitID string
	Commits     *models.PushCommits
}

// MirrorSyncPushAction adds new action for mirror synchronization of pushed commits.
func MirrorSyncPushAction(repo *models.Repository, opts MirrorSyncPushActionOptions) error {
	if len(opts.Commits.Commits) > setting.UI.FeedMaxCommitNum {
		opts.Commits.Commits = opts.Commits.Commits[:setting.UI.FeedMaxCommitNum]
	}

	apiCommits, err := opts.Commits.ToAPIPayloadCommits(repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		return err
	}

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

	return mirrorSyncAction(models.ActionMirrorSyncPush, repo, opts.RefName, data)
}

// MirrorSyncCreateAction adds new action for mirror synchronization of new reference.
func MirrorSyncCreateAction(repo *models.Repository, refName string) error {
	return mirrorSyncAction(models.ActionMirrorSyncCreate, repo, refName, nil)
}

// MirrorSyncDeleteAction adds new action for mirror synchronization of delete reference.
func MirrorSyncDeleteAction(repo *models.Repository, refName string) error {
	return mirrorSyncAction(models.ActionMirrorSyncDelete, repo, refName, nil)
}
