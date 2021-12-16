// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/queue"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	issue_service "code.gitea.io/gitea/services/issue"
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
	pushQueue = queue.CreateQueue("push_update", handle, []*repo_module.PushUpdateOptions{})
	if pushQueue == nil {
		return errors.New("unable to create push_update Queue")
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

	repo, err := repo_model.GetRepositoryByOwnerAndName(optsList[0].RepoUserName, optsList[0].RepoName)
	if err != nil {
		return fmt.Errorf("GetRepositoryByOwnerAndName failed: %v", err)
	}

	repoPath := repo.RepoPath()
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	if err = models.UpdateRepoSize(db.DefaultContext, repo); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	addTags := make([]string, 0, len(optsList))
	delTags := make([]string, 0, len(optsList))
	var pusher *user_model.User

	for _, opts := range optsList {
		if opts.IsNewRef() && opts.IsDelRef() {
			return fmt.Errorf("Old and new revisions are both %s", git.EmptySHA)
		}
		if opts.IsTag() { // If is tag reference
			if pusher == nil || pusher.ID != opts.PusherID {
				var err error
				if pusher, err = user_model.GetUserByID(opts.PusherID); err != nil {
					return err
				}
			}
			tagName := opts.TagName()
			if opts.IsDelRef() {
				notification.NotifyPushCommits(
					pusher, repo,
					&repo_module.PushUpdateOptions{
						RefFullName: git.TagPrefix + tagName,
						OldCommitID: opts.OldCommitID,
						NewCommitID: git.EmptySHA,
					}, repo_module.NewPushCommits())

				delTags = append(delTags, tagName)
				notification.NotifyDeleteRef(pusher, repo, "tag", opts.RefFullName)
			} else { // is new tag
				notification.NotifyPushCommits(
					pusher, repo,
					&repo_module.PushUpdateOptions{
						RefFullName: git.TagPrefix + tagName,
						OldCommitID: git.EmptySHA,
						NewCommitID: opts.NewCommitID,
					}, repo_module.NewPushCommits())

				addTags = append(addTags, tagName)
				notification.NotifyCreateRef(pusher, repo, "tag", opts.RefFullName)
			}
		} else if opts.IsBranch() { // If is branch reference
			if pusher == nil || pusher.ID != opts.PusherID {
				var err error
				if pusher, err = user_model.GetUserByID(opts.PusherID); err != nil {
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

				refName := opts.RefName()

				// Push new branch.
				var l []*git.Commit
				if opts.IsNewRef() {
					if repo.IsEmpty { // Change default branch and empty status only if pushed ref is non-empty branch.
						repo.DefaultBranch = refName
						repo.IsEmpty = false
						if repo.DefaultBranch != setting.Repository.DefaultBranch {
							if err := gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
								if !git.IsErrUnsupportedVersion(err) {
									return err
								}
							}
						}
						// Update the is empty and default_branch columns
						if err := repo_model.UpdateRepositoryCols(repo, "default_branch", "is_empty"); err != nil {
							return fmt.Errorf("UpdateRepositoryCols: %v", err)
						}
					}

					l, err = newCommit.CommitsBeforeLimit(10)
					if err != nil {
						return fmt.Errorf("newCommit.CommitsBeforeLimit: %v", err)
					}
					notification.NotifyCreateRef(pusher, repo, "branch", opts.RefFullName)
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

				commits := repo_module.GitToPushCommits(l)
				commits.HeadCommit = repo_module.CommitToPushCommit(newCommit)

				if err := issue_service.UpdateIssuesCommit(pusher, repo, commits.Commits, refName); err != nil {
					log.Error("updateIssuesCommit: %v", err)
				}

				if len(commits.Commits) > setting.UI.FeedMaxCommitNum {
					commits.Commits = commits.Commits[:setting.UI.FeedMaxCommitNum]
				}
				commits.CompareURL = repo.ComposeCompareURL(opts.OldCommitID, opts.NewCommitID)
				notification.NotifyPushCommits(pusher, repo, opts, commits)

				if err = models.RemoveDeletedBranchByName(repo.ID, branch); err != nil {
					log.Error("models.RemoveDeletedBranch %s/%s failed: %v", repo.ID, branch, err)
				}

				// Cache for big repository
				if err := CacheRef(graceful.GetManager().HammerContext(), repo, gitRepo, opts.RefFullName); err != nil {
					log.Error("repo_module.CacheRef %s/%s failed: %v", repo.ID, branch, err)
				}
			} else {
				notification.NotifyDeleteRef(pusher, repo, "branch", opts.RefFullName)
				if err = pull_service.CloseBranchPulls(pusher, repo.ID, branch); err != nil {
					// close all related pulls
					log.Error("close related pull request failed: %v", err)
				}
			}

			// Even if user delete a branch on a repository which he didn't watch, he will be watch that.
			if err = repo_model.WatchIfAuto(opts.PusherID, repo.ID, true); err != nil {
				log.Warn("Fail to perform auto watch on user %v for repo %v: %v", opts.PusherID, repo.ID, err)
			}
		} else {
			log.Trace("Non-tag and non-branch commits pushed.")
		}
	}
	if err := PushUpdateAddDeleteTags(repo, gitRepo, addTags, delTags); err != nil {
		return fmt.Errorf("PushUpdateAddDeleteTags: %v", err)
	}

	// Change repository last updated time.
	if err := repo_model.UpdateRepositoryUpdatedTime(repo.ID, time.Now()); err != nil {
		return fmt.Errorf("UpdateRepositoryUpdatedTime: %v", err)
	}

	return nil
}

// PushUpdateAddDeleteTags updates a number of added and delete tags
func PushUpdateAddDeleteTags(repo *repo_model.Repository, gitRepo *git.Repository, addTags, delTags []string) error {
	return db.WithTx(func(ctx context.Context) error {
		if err := models.PushUpdateDeleteTagsContext(ctx, repo, delTags); err != nil {
			return err
		}
		return pushUpdateAddTags(ctx, repo, gitRepo, addTags)
	})
}

// pushUpdateAddTags updates a number of add tags
func pushUpdateAddTags(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	lowerTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowerTags = append(lowerTags, strings.ToLower(tag))
	}

	releases, err := models.GetReleasesByRepoIDAndNames(ctx, repo.ID, lowerTags)
	if err != nil {
		return fmt.Errorf("GetReleasesByRepoIDAndNames: %v", err)
	}
	relMap := make(map[string]*models.Release)
	for _, rel := range releases {
		relMap[rel.LowerTagName] = rel
	}

	newReleases := make([]*models.Release, 0, len(lowerTags)-len(relMap))

	emailToUser := make(map[string]*user_model.User)

	for i, lowerTag := range lowerTags {
		tag, err := gitRepo.GetTag(tags[i])
		if err != nil {
			return fmt.Errorf("GetTag: %v", err)
		}
		commit, err := tag.Commit()
		if err != nil {
			return fmt.Errorf("Commit: %v", err)
		}

		sig := tag.Tagger
		if sig == nil {
			sig = commit.Author
		}
		if sig == nil {
			sig = commit.Committer
		}
		var author *user_model.User
		var createdAt = time.Unix(1, 0)

		if sig != nil {
			var ok bool
			author, ok = emailToUser[sig.Email]
			if !ok {
				author, err = user_model.GetUserByEmailContext(ctx, sig.Email)
				if err != nil && !user_model.IsErrUserNotExist(err) {
					return fmt.Errorf("GetUserByEmail: %v", err)
				}
				if author != nil {
					emailToUser[sig.Email] = author
				}
			}
			createdAt = sig.When
		}

		commitsCount, err := commit.CommitsCount()
		if err != nil {
			return fmt.Errorf("CommitsCount: %v", err)
		}

		rel, has := relMap[lowerTag]

		if !has {
			rel = &models.Release{
				RepoID:       repo.ID,
				Title:        "",
				TagName:      tags[i],
				LowerTagName: lowerTag,
				Target:       "",
				Sha1:         commit.ID.String(),
				NumCommits:   commitsCount,
				Note:         "",
				IsDraft:      false,
				IsPrerelease: false,
				IsTag:        true,
				CreatedUnix:  timeutil.TimeStamp(createdAt.Unix()),
			}
			if author != nil {
				rel.PublisherID = author.ID
			}

			newReleases = append(newReleases, rel)
		} else {
			rel.Sha1 = commit.ID.String()
			rel.CreatedUnix = timeutil.TimeStamp(createdAt.Unix())
			rel.NumCommits = commitsCount
			rel.IsDraft = false
			if rel.IsTag && author != nil {
				rel.PublisherID = author.ID
			}
			if err = models.UpdateRelease(ctx, rel); err != nil {
				return fmt.Errorf("Update: %v", err)
			}
		}
	}

	if len(newReleases) > 0 {
		if err = models.InsertReleasesContext(ctx, newReleases); err != nil {
			return fmt.Errorf("Insert: %v", err)
		}
	}

	return nil
}
