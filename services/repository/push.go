// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"container/list"
	"encoding/json"
	"fmt"
	"strings"
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

// PushUpdateOptions defines the push update options
type PushUpdateOptions struct {
	PusherID     int64
	PusherName   string
	RepoUserName string
	RepoName     string
	RefFullName  string
	OldCommitID  string
	NewCommitID  string
}

// IsNewRef return true if it's a first-time push to a branch, tag or etc.
func (opts PushUpdateOptions) IsNewRef() bool {
	return opts.OldCommitID == git.EmptySHA
}

// IsDelRef return true if it's a deletion to a branch or tag
func (opts PushUpdateOptions) IsDelRef() bool {
	return opts.NewCommitID == git.EmptySHA
}

// IsUpdateRef return true if it's an update operation
func (opts PushUpdateOptions) IsUpdateRef() bool {
	return !opts.IsNewRef() && !opts.IsDelRef()
}

// IsTag return true if it's an operation to a tag
func (opts PushUpdateOptions) IsTag() bool {
	return strings.HasPrefix(opts.RefFullName, git.TagPrefix)
}

// IsNewTag return true if it's a creation to a tag
func (opts PushUpdateOptions) IsNewTag() bool {
	return opts.IsTag() && opts.IsNewRef()
}

// IsDelTag return true if it's a deletion to a tag
func (opts PushUpdateOptions) IsDelTag() bool {
	return opts.IsTag() && opts.IsDelRef()
}

// IsBranch return true if it's a push to branch
func (opts PushUpdateOptions) IsBranch() bool {
	return strings.HasPrefix(opts.RefFullName, git.BranchPrefix)
}

// IsNewBranch return true if it's the first-time push to a branch
func (opts PushUpdateOptions) IsNewBranch() bool {
	return opts.IsBranch() && opts.IsNewRef()
}

// IsUpdateBranch return true if it's not the first push to a branch
func (opts PushUpdateOptions) IsUpdateBranch() bool {
	return opts.IsBranch() && opts.IsUpdateRef()
}

// IsDelBranch return true if it's a deletion to a branch
func (opts PushUpdateOptions) IsDelBranch() bool {
	return opts.IsBranch() && opts.IsDelRef()
}

// TagName returns simple tag name if it's an operation to a tag
func (opts PushUpdateOptions) TagName() string {
	return opts.RefFullName[len(git.TagPrefix):]
}

// BranchName returns simple branch name if it's an operation to branch
func (opts PushUpdateOptions) BranchName() string {
	return opts.RefFullName[len(git.BranchPrefix):]
}

// RepoFullName returns repo full name
func (opts PushUpdateOptions) RepoFullName() string {
	return opts.RepoUserName + "/" + opts.RepoName
}

// pushQueue represents a queue to handle update pull request tests
var pushQueue queue.Queue

// handle passed PR IDs and test the PRs
func handle(data ...queue.Data) {
	for _, datum := range data {
		opts := datum.([]*PushUpdateOptions)
		if err := pushUpdates(opts); err != nil {
			log.Error("pushUpdate failed: %v", err)
		}
	}
}

func initPushQueue() error {
	pushQueue = queue.CreateQueue("push_update", handle, []*PushUpdateOptions{}).(queue.Queue)
	if pushQueue == nil {
		return fmt.Errorf("Unable to create push_update Queue")
	}

	go graceful.GetManager().RunWithShutdownFns(pushQueue.Run)
	return nil
}

// PushUpdate is an alias of PushUpdates for single push update options
func PushUpdate(opts *PushUpdateOptions) error {
	return PushUpdates([]*PushUpdateOptions{opts})
}

// PushUpdates adds a push update to push queue
func PushUpdates(opts []*PushUpdateOptions) error {
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
func pushUpdates(optsList []*PushUpdateOptions) error {
	if len(optsList) == 0 {
		return nil
	}

	repo, err := models.GetRepositoryByOwnerAndName(optsList[0].RepoUserName, optsList[0].RepoName)
	if err != nil {
		return fmt.Errorf("GetRepositoryByOwnerAndName failed: %v", err)
	}

	repoPath := repo.RepoPath()
	_, err = git.NewCommand("update-server-info").RunInDir(repoPath)
	if err != nil {
		return fmt.Errorf("Failed to call 'git update-server-info': %v", err)
	}
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
				cache.Remove(repo.GetCommitsCountCacheKey(tagName, true))
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
				// Clear cache for branch commit count
				cache.Remove(repo.GetCommitsCountCacheKey(opts.BranchName(), true))

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
				}

				commits = repo_module.ListToPushCommits(l)

				if err = models.RemoveDeletedBranch(repo.ID, branch); err != nil {
					log.Error("models.RemoveDeletedBranch %s/%s failed: %v", repo.ID, branch, err)
				}

				log.Trace("TriggerTask '%s/%s' by %s", repo.Name, branch, pusher.Name)

				go pull_service.AddTestPullRequestTask(pusher, repo.ID, branch, true, opts.OldCommitID, opts.NewCommitID)
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
	PushUpdateOptions

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
			notification.NotifyPushCommits(opts.Pusher, repo, opts.RefFullName, opts.OldCommitID, opts.NewCommitID, opts.Commits)
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
