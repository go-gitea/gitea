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
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
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
					gitRepo.Close()
					return err
				}
			}
			gitRepo.Close()
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

	defer func() {
		go models.HookQueue.Add(repo.ID)
	}()

	apiPusher := pusher.APIFormat()
	apiRepo := repo.APIFormat(models.AccessModeNone)

	var shaSum string
	var isHookEventPush = false
	switch opType {
	case models.ActionCommitRepo: // Push
		isHookEventPush = true

		if isNewBranch {
			gitRepo, err := git.OpenRepository(repo.RepoPath())
			if err != nil {
				log.Error("OpenRepository[%s]: %v", repo.RepoPath(), err)
			}

			shaSum, err = gitRepo.GetBranchCommitID(refName)
			if err != nil {
				gitRepo.Close()
				log.Error("GetBranchCommitID[%s]: %v", opts.RefFullName, err)
			}
			gitRepo.Close()
			if err = models.PrepareWebhooks(repo, models.HookEventCreate, &api.CreatePayload{
				Ref:     refName,
				Sha:     shaSum,
				RefType: "branch",
				Repo:    apiRepo,
				Sender:  apiPusher,
			}); err != nil {
				return fmt.Errorf("PrepareWebhooks: %v", err)
			}
		}

	case models.ActionDeleteBranch: // Delete Branch
		isHookEventPush = true

		if err = models.PrepareWebhooks(repo, models.HookEventDelete, &api.DeletePayload{
			Ref:        refName,
			RefType:    "branch",
			PusherType: api.PusherTypeUser,
			Repo:       apiRepo,
			Sender:     apiPusher,
		}); err != nil {
			return fmt.Errorf("PrepareWebhooks.(delete branch): %v", err)
		}

	case models.ActionPushTag: // Create
		isHookEventPush = true

		gitRepo, err := git.OpenRepository(repo.RepoPath())
		if err != nil {
			log.Error("OpenRepository[%s]: %v", repo.RepoPath(), err)
		}
		shaSum, err = gitRepo.GetTagCommitID(refName)
		if err != nil {
			gitRepo.Close()
			log.Error("GetTagCommitID[%s]: %v", opts.RefFullName, err)
		}
		gitRepo.Close()
		if err = models.PrepareWebhooks(repo, models.HookEventCreate, &api.CreatePayload{
			Ref:     refName,
			Sha:     shaSum,
			RefType: "tag",
			Repo:    apiRepo,
			Sender:  apiPusher,
		}); err != nil {
			return fmt.Errorf("PrepareWebhooks: %v", err)
		}
	case models.ActionDeleteTag: // Delete Tag
		isHookEventPush = true

		if err = models.PrepareWebhooks(repo, models.HookEventDelete, &api.DeletePayload{
			Ref:        refName,
			RefType:    "tag",
			PusherType: api.PusherTypeUser,
			Repo:       apiRepo,
			Sender:     apiPusher,
		}); err != nil {
			return fmt.Errorf("PrepareWebhooks.(delete tag): %v", err)
		}
	}

	if isHookEventPush {
		commits, err := opts.Commits.ToAPIPayloadCommits(repo.RepoPath(), repo.HTMLURL())
		if err != nil {
			return err
		}
		if err = models.PrepareWebhooks(repo, models.HookEventPush, &api.PushPayload{
			Ref:        opts.RefFullName,
			Before:     opts.OldCommitID,
			After:      opts.NewCommitID,
			CompareURL: setting.AppURL + opts.Commits.CompareURL,
			Commits:    commits,
			Repo:       apiRepo,
			Pusher:     apiPusher,
			Sender:     apiPusher,
		}); err != nil {
			return fmt.Errorf("PrepareWebhooks: %v", err)
		}
	}

	return nil
}
