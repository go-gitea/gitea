// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"container/list"
	"encoding/json"
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/repofiles"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	pull_service "code.gitea.io/gitea/services/pull"
)

// pushQueue represents a queue to handle update pull request tests
var pushQueue queue.Queue

// handle passed PR IDs and test the PRs
func handle(data ...queue.Data) {
	for _, datum := range data {
		opts := datum.([]*repo_module.PushUpdateOptions)
		if err := pushUpdates(opts); err != nil {
			log.Error("pushUpdate failed: %v", err)
		}
	}
}

func initPushQueue() error {
	pushQueue = queue.CreateQueue("push_update", handle, []*repo_module.PushUpdateOptions{}).(queue.Queue)
	if pushQueue == nil {
		return fmt.Errorf("Unable to create push_update Queue")
	}

	go graceful.GetManager().RunWithShutdownFns(pushQueue.Run)
	return nil
}

// PushUpdate is an alias of PushUpdates for single push update options
func PushUpdate(opts *repo_module.PushUpdateOptions) error {
	return PushUpdates([]*repo_module.PushUpdateOptions{opts})
}

// PushUpdates adds a push update to push queue
func PushUpdates(opts []*repo_module.PushUpdateOptions) error {
	if len(opts) == 0 {
		return nil
	}

	for _, opt := range opts {
		if opt.IsNewRef() && opt.IsDelRef() {
			return fmt.Errorf("Old and new revisions are both %s", git.EmptySHA)
		}
	}

	return pushQueue.Push(opts)
}

// pushUpdates generates push action history feeds for push updating multiple refs
func pushUpdates(optsList []*repo_module.PushUpdateOptions) error {
	if len(optsList) == 0 {
		return nil
	}

	repo, err := models.GetRepositoryByOwnerAndName(optsList[0].RepoUserName, optsList[0].RepoName)
	if err != nil {
		return fmt.Errorf("GetRepositoryByOwnerAndName failed: %v", err)
	}

	repoPath := repo.RepoPath()
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	if err = repo.UpdateSize(models.DefaultDBContext()); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	addTags := make([]string, 0, len(optsList))
	delTags := make([]string, 0, len(optsList))
	actions := make([]*commitRepoActionOptions, 0, len(optsList))
	var pusher *models.User

	for _, opts := range optsList {
		if opts.IsNewRef() && opts.IsDelRef() {
			return fmt.Errorf("Old and new revisions are both %s", git.EmptySHA)
		}
		var commits = &repo_module.PushCommits{}
		if opts.IsTag() { // If is tag reference {
			tagName := opts.TagName()
			if opts.IsDelRef() {
				delTags = append(delTags, tagName)
			} else { // is new tag
				addTags = append(addTags, tagName)
			}
		} else if opts.IsBranch() { // If is branch reference
			if pusher == nil || pusher.ID != opts.PusherID {
				var err error
				if pusher, err = models.GetUserByID(opts.PusherID); err != nil {
					return err
				}
			}

			branch := opts.BranchName()
			if !opts.IsDelRef() {
				log.Trace("TriggerTask '%s/%s' by %s", repo.Name, branch, pusher.Name)
				go pull_service.AddTestPullRequestTask(pusher, repo.ID, branch, true, opts.OldCommitID, opts.NewCommitID)

				newCommit, err := gitRepo.GetCommit(opts.NewCommitID)
				if err != nil {
					return fmt.Errorf("gitRepo.GetCommit: %v", err)
				}

				// Push new branch.
				var l *list.List
				if opts.IsNewRef() {
					l, err = newCommit.CommitsBeforeLimit(10)
					if err != nil {
						return fmt.Errorf("newCommit.CommitsBeforeLimit: %v", err)
					}
				} else {
					l, err = newCommit.CommitsBeforeUntil(opts.OldCommitID)
					if err != nil {
						return fmt.Errorf("newCommit.CommitsBeforeUntil: %v", err)
					}

					isForce, err := repo_module.IsForcePush(opts)
					if err != nil {
						log.Error("isForcePush %s:%s failed: %v", repo.FullName(), branch, err)
					}

					if isForce {
						log.Trace("Push %s is a force push", opts.NewCommitID)

						cache.Remove(repo.GetCommitsCountCacheKey(opts.RefName(), true))
					} else {
						// TODO: increment update the commit count cache but not remove
						cache.Remove(repo.GetCommitsCountCacheKey(opts.RefName(), true))
					}
				}

				commits = repo_module.ListToPushCommits(l)

				if err = models.RemoveDeletedBranch(repo.ID, branch); err != nil {
					log.Error("models.RemoveDeletedBranch %s/%s failed: %v", repo.ID, branch, err)
				}

				// Cache for big repository
				if err := repo_module.CacheRef(repo, gitRepo, opts.RefFullName); err != nil {
					log.Error("repo_module.CacheRef %s/%s failed: %v", repo.ID, branch, err)
				}
			} else if err = pull_service.CloseBranchPulls(pusher, repo.ID, branch); err != nil {
				// close all related pulls
				log.Error("close related pull request failed: %v", err)
			}

			// Even if user delete a branch on a repository which he didn't watch, he will be watch that.
			if err = models.WatchIfAuto(opts.PusherID, repo.ID, true); err != nil {
				log.Warn("Fail to perform auto watch on user %v for repo %v: %v", opts.PusherID, repo.ID, err)
			}
		}
		actions = append(actions, &commitRepoActionOptions{
			PushUpdateOptions: *opts,
			Pusher:            pusher,
			RepoOwnerID:       repo.OwnerID,
			Commits:           commits,
		})
	}
	if err := repo_module.PushUpdateAddDeleteTags(repo, gitRepo, addTags, delTags); err != nil {
		return fmt.Errorf("PushUpdateAddDeleteTags: %v", err)
	}

	if err := commitRepoAction(repo, gitRepo, actions...); err != nil {
		return fmt.Errorf("commitRepoAction: %v", err)
	}

	return nil
}

// commitRepoActionOptions represent options of a new commit action.
type commitRepoActionOptions struct {
	repo_module.PushUpdateOptions

	Pusher      *models.User
	RepoOwnerID int64
	Commits     *repo_module.PushCommits
}

// commitRepoAction adds new commit action to the repository, and prepare
// corresponding webhooks.
func commitRepoAction(repo *models.Repository, gitRepo *git.Repository, optsList ...*commitRepoActionOptions) error {
	actions := make([]*models.Action, len(optsList))

	for i, opts := range optsList {
		if opts.Pusher == nil || opts.Pusher.Name != opts.PusherName {
			var err error
			opts.Pusher, err = models.GetUserByName(opts.PusherName)
			if err != nil {
				return fmt.Errorf("GetUserByName [%s]: %v", opts.PusherName, err)
			}
		}

		refName := git.RefEndName(opts.RefFullName)

		// Change default branch and empty status only if pushed ref is non-empty branch.
		if repo.IsEmpty && opts.IsBranch() && !opts.IsDelRef() {
			repo.DefaultBranch = refName
			repo.IsEmpty = false
			if refName != "master" {
				if err := gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
					if !git.IsErrUnsupportedVersion(err) {
						return err
					}
				}
			}
			// Update the is empty and default_branch columns
			if err := models.UpdateRepositoryCols(repo, "default_branch", "is_empty"); err != nil {
				return fmt.Errorf("UpdateRepositoryCols: %v", err)
			}
		}

		opType := models.ActionCommitRepo

		// Check it's tag push or branch.
		if opts.IsTag() {
			opType = models.ActionPushTag
			if opts.IsDelRef() {
				opType = models.ActionDeleteTag
			}
			opts.Commits = &repo_module.PushCommits{}
		} else if opts.IsDelRef() {
			opType = models.ActionDeleteBranch
			opts.Commits = &repo_module.PushCommits{}
		} else {
			// if not the first commit, set the compare URL.
			if !opts.IsNewRef() {
				opts.Commits.CompareURL = repo.ComposeCompareURL(opts.OldCommitID, opts.NewCommitID)
			}

			if err := repofiles.UpdateIssuesCommit(opts.Pusher, repo, opts.Commits.Commits, refName); err != nil {
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

		actions[i] = &models.Action{
			ActUserID: opts.Pusher.ID,
			ActUser:   opts.Pusher,
			OpType:    opType,
			Content:   string(data),
			RepoID:    repo.ID,
			Repo:      repo,
			RefName:   refName,
			IsPrivate: repo.IsPrivate,
		}

		var isHookEventPush = true
		switch opType {
		case models.ActionCommitRepo: // Push
			if opts.IsNewBranch() {
				notification.NotifyCreateRef(opts.Pusher, repo, "branch", opts.RefFullName)
			}
		case models.ActionDeleteBranch: // Delete Branch
			notification.NotifyDeleteRef(opts.Pusher, repo, "branch", opts.RefFullName)

		case models.ActionPushTag: // Create
			notification.NotifyCreateRef(opts.Pusher, repo, "tag", opts.RefFullName)

		case models.ActionDeleteTag: // Delete Tag
			notification.NotifyDeleteRef(opts.Pusher, repo, "tag", opts.RefFullName)
		default:
			isHookEventPush = false
		}

		if isHookEventPush {
			notification.NotifyPushCommits(opts.Pusher, repo, &opts.PushUpdateOptions, opts.Commits)
		}
	}

	// Change repository last updated time.
	if err := models.UpdateRepositoryUpdatedTime(repo.ID, time.Now()); err != nil {
		return fmt.Errorf("UpdateRepositoryUpdatedTime: %v", err)
	}

	if err := models.NotifyWatchers(actions...); err != nil {
		return fmt.Errorf("NotifyWatchers: %v", err)
	}
	return nil
}
