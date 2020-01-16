// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

// Update ToDo wip ...
func Update(pull *models.PullRequest, doer *models.User, message string) (err error) {
	//use merge functions but switch repo's and branches
	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	if err = pr.LoadHeadRepo(); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return fmt.Errorf("LoadHeadRepo: %v", err)
	} else if err = pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return fmt.Errorf("LoadBaseRepo: %v", err)
	}

	if err := rawMerge(pr, doer, models.MergeStyleMerge, message); err != nil {
		return err
	}

	//notification.NotifyPullRequestUpdated(pr, doer)
	//trigger hooks and co ..

	return nil
}

// IsUserAllowedToUpdate check if user is allowed to update PR with given permissions and branch protections
func IsUserAllowedToUpdate(pull *models.PullRequest, p models.Permission, user *models.User) (bool, error) {
	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}
	return IsUserAllowedToMerge(pr, p, user)
}
