// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
)

// CommitRepoActionOptions represent options of a new commit action.
type CommitRepoActionOptions struct {
	PusherName  string
	RepoOwnerID int64
	RepoName    string
	RefFullName string
	OldCommitID string
	NewCommitID string
	Commits     *models.PushCommits
}

// CommitRepoAction adds new commit action to the repository, and prepare
// corresponding webhooks.
func CommitRepoAction(opts CommitRepoActionOptions) error {
	pusher, err := models.GetUserByName(opts.PusherName)
	if err != nil {
		return fmt.Errorf("GetUserByName [%s]: %v", opts.PusherName, err)
	}

	repo, err := models.GetRepositoryByName(opts.RepoOwnerID, opts.RepoName)
	if err != nil {
		return fmt.Errorf("GetRepositoryByName [owner_id: %d, name: %s]: %v", opts.RepoOwnerID, opts.RepoName, err)
	}

	refName := git.RefEndName(opts.RefFullName)

	// Change default branch and empty status only if pushed ref is non-empty branch.
	if repo.IsEmpty && opts.NewCommitID != git.EmptySHA && strings.HasPrefix(opts.RefFullName, git.BranchPrefix) {
		repo.DefaultBranch = refName
		repo.IsEmpty = false
		if refName != "master" {
			gitRepo, err := git.OpenRepository(repo.RepoPath())
			if err != nil {
				return err
			}
			if err := gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
				if !git.IsErrUnsupportedVersion(err) {
					return err
				}
			}
		}
	}

	// Change repository empty status and update last updated time.
	if err = models.UpdateRepository(repo, false); err != nil {
		return fmt.Errorf("UpdateRepository: %v", err)
	}

	isNewBranch := false
	opType := models.ActionCommitRepo
	// Check it's tag push or branch.
	if strings.HasPrefix(opts.RefFullName, git.TagPrefix) {
		opType = models.ActionPushTag
		if opts.NewCommitID == git.EmptySHA {
			opType = models.ActionDeleteTag
		}
		opts.Commits = &models.PushCommits{}
	} else if opts.NewCommitID == git.EmptySHA {
		opType = models.ActionDeleteBranch
		opts.Commits = &models.PushCommits{}
	} else {
		// if not the first commit, set the compare URL.
		if opts.OldCommitID == git.EmptySHA {
			isNewBranch = true
		} else {
			opts.Commits.CompareURL = repo.ComposeCompareURL(opts.OldCommitID, opts.NewCommitID)
		}

		if err = models.UpdateIssuesCommit(pusher, repo, opts.Commits.Commits, refName); err != nil {
			log.Error("updateIssuesCommit: %v", err)
		}
	}

	if len(opts.Commits.Commits) > setting.UI.FeedMaxCommitNum {
		opts.Commits.Commits = opts.Commits.Commits[:setting.UI.FeedMaxCommitNum]
	}

	data, err := json.Marshal(opts.Commits)
	if err != nil {
		return fmt.Errorf("Marshal: %v", err)
	}

	if err = models.NotifyWatchers(&models.Action{
		ActUserID: pusher.ID,
		ActUser:   pusher,
		OpType:    opType,
		Content:   string(data),
		RepoID:    repo.ID,
		Repo:      repo,
		RefName:   refName,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		return fmt.Errorf("NotifyWatchers: %v", err)
	}

	var isHookEventPush = true
	switch opType {
	case models.ActionCommitRepo: // Push
		if isNewBranch {
			notification.NotifyCreateRef(pusher, repo, "branch", opts.RefFullName)
		}

	case models.ActionDeleteBranch: // Delete Branch
		notification.NotifyDeleteRef(pusher, repo, "branch", opts.RefFullName)

	case models.ActionPushTag: // Create
		notification.NotifyCreateRef(pusher, repo, "tag", opts.RefFullName)

	case models.ActionDeleteTag: // Delete Tag
		notification.NotifyDeleteRef(pusher, repo, "tag", opts.RefFullName)
	default:
		isHookEventPush = false
	}

	if isHookEventPush {
		notification.NotifyPushCommits(pusher, repo, opts.RefFullName, opts.OldCommitID, opts.NewCommitID, opts.Commits)
	}

	return nil
}
