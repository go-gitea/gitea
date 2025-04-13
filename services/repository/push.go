// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	issue_service "code.gitea.io/gitea/services/issue"
	notify_service "code.gitea.io/gitea/services/notify"
	pull_service "code.gitea.io/gitea/services/pull"
)

// pushQueue represents a queue to handle update pull request tests
var pushQueue *queue.WorkerPoolQueue[[]*repo_module.PushUpdateOptions]

// handle passed PR IDs and test the PRs
func handler(items ...[]*repo_module.PushUpdateOptions) [][]*repo_module.PushUpdateOptions {
	for _, opts := range items {
		if err := pushUpdates(opts); err != nil {
			// Username and repository stays the same between items in opts.
			pushUpdate := opts[0]
			log.Error("pushUpdate[%s/%s] failed: %v", pushUpdate.RepoUserName, pushUpdate.RepoName, err)
		}
	}
	return nil
}

func initPushQueue() error {
	pushQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "push_update", handler)
	if pushQueue == nil {
		return errors.New("unable to create push_update queue")
	}
	go graceful.GetManager().RunWithCancel(pushQueue)
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
			return errors.New("Old and new revisions are both NULL")
		}
	}

	return pushQueue.Push(opts)
}

// pushUpdates generates push action history feeds for push updating multiple refs
func pushUpdates(optsList []*repo_module.PushUpdateOptions) error {
	if len(optsList) == 0 {
		return nil
	}

	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("PushUpdates: %s/%s", optsList[0].RepoUserName, optsList[0].RepoName))
	defer finished()

	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, optsList[0].RepoUserName, optsList[0].RepoName)
	if err != nil {
		return fmt.Errorf("GetRepositoryByOwnerAndName failed: %w", err)
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return fmt.Errorf("OpenRepository[%s]: %w", repo.FullName(), err)
	}
	defer gitRepo.Close()

	if err = repo_module.UpdateRepoSize(ctx, repo); err != nil {
		return fmt.Errorf("Failed to update size for repository: %v", err)
	}

	addTags := make([]string, 0, len(optsList))
	delTags := make([]string, 0, len(optsList))
	var pusher *user_model.User
	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)

	for _, opts := range optsList {
		log.Trace("pushUpdates: %-v %s %s %s", repo, opts.OldCommitID, opts.NewCommitID, opts.RefFullName)

		if opts.IsNewRef() && opts.IsDelRef() {
			return fmt.Errorf("old and new revisions are both %s", objectFormat.EmptyObjectID())
		}
		if opts.RefFullName.IsTag() {
			if pusher == nil || pusher.ID != opts.PusherID {
				if opts.PusherID == user_model.ActionsUserID {
					pusher = user_model.NewActionsUser()
				} else {
					var err error
					if pusher, err = user_model.GetUserByID(ctx, opts.PusherID); err != nil {
						return err
					}
				}
			}
			tagName := opts.RefFullName.TagName()
			if opts.IsDelRef() {
				notify_service.PushCommits(
					ctx, pusher, repo,
					&repo_module.PushUpdateOptions{
						RefFullName: git.RefNameFromTag(tagName),
						OldCommitID: opts.OldCommitID,
						NewCommitID: objectFormat.EmptyObjectID().String(),
					}, repo_module.NewPushCommits())

				delTags = append(delTags, tagName)
				notify_service.DeleteRef(ctx, pusher, repo, opts.RefFullName)
			} else { // is new tag
				newCommit, err := gitRepo.GetCommit(opts.NewCommitID)
				if err != nil {
					// in case there is dirty data, for example, the "github.com/git/git" repository has tags pointing to non-existing commits
					if !errors.Is(err, util.ErrNotExist) {
						log.Error("Unable to get tag commit: gitRepo.GetCommit(%s) in %s/%s[%d]: %v", opts.NewCommitID, repo.OwnerName, repo.Name, repo.ID, err)
					}
				} else {
					commits := repo_module.NewPushCommits()
					commits.HeadCommit = repo_module.CommitToPushCommit(newCommit)
					commits.CompareURL = repo.ComposeCompareURL(objectFormat.EmptyObjectID().String(), opts.NewCommitID)

					notify_service.PushCommits(
						ctx, pusher, repo,
						&repo_module.PushUpdateOptions{
							RefFullName: opts.RefFullName,
							OldCommitID: objectFormat.EmptyObjectID().String(),
							NewCommitID: opts.NewCommitID,
						}, commits)

					addTags = append(addTags, tagName)
					notify_service.CreateRef(ctx, pusher, repo, opts.RefFullName, opts.NewCommitID)
				}
			}
		} else if opts.RefFullName.IsBranch() {
			if pusher == nil || pusher.ID != opts.PusherID {
				if opts.PusherID == user_model.ActionsUserID {
					pusher = user_model.NewActionsUser()
				} else {
					var err error
					if pusher, err = user_model.GetUserByID(ctx, opts.PusherID); err != nil {
						return err
					}
				}
			}

			if !opts.IsDelRef() {
				branch := opts.RefFullName.BranchName()

				log.Trace("TriggerTask '%s/%s' by %s", repo.Name, branch, pusher.Name)

				newCommit, err := gitRepo.GetCommit(opts.NewCommitID)
				if err != nil {
					return fmt.Errorf("gitRepo.GetCommit(%s) in %s/%s[%d]: %w", opts.NewCommitID, repo.OwnerName, repo.Name, repo.ID, err)
				}

				// Push new branch.
				var l []*git.Commit
				if opts.IsNewRef() {
					l, err = pushNewBranch(ctx, repo, pusher, opts, newCommit)
				} else {
					l, err = pushUpdateBranch(ctx, repo, pusher, opts, newCommit)
				}
				if err != nil {
					return err
				}

				// delete cache for divergence
				if branch == repo.DefaultBranch {
					if err := DelRepoDivergenceFromCache(ctx, repo.ID); err != nil {
						log.Error("DelRepoDivergenceFromCache: %v", err)
					}
				} else {
					if err := DelDivergenceFromCache(repo.ID, branch); err != nil {
						log.Error("DelDivergenceFromCache: %v", err)
					}
				}

				commits := repo_module.GitToPushCommits(l)
				commits.HeadCommit = repo_module.CommitToPushCommit(newCommit)

				if err := issue_service.UpdateIssuesCommit(ctx, pusher, repo, commits.Commits, opts.RefName()); err != nil {
					log.Error("updateIssuesCommit: %v", err)
				}

				commits.CompareURL = getCompareURL(repo, gitRepo, objectFormat, commits.Commits, opts)

				if len(commits.Commits) > setting.UI.FeedMaxCommitNum {
					commits.Commits = commits.Commits[:setting.UI.FeedMaxCommitNum]
				}

				notify_service.PushCommits(ctx, pusher, repo, opts, commits)

				// Cache for big repository
				if err := CacheRef(graceful.GetManager().HammerContext(), repo, gitRepo, opts.RefFullName); err != nil {
					log.Error("repo_module.CacheRef %s/%s failed: %v", repo.ID, branch, err)
				}
			} else {
				pushDeleteBranch(ctx, repo, pusher, opts)
			}

			// Even if user delete a branch on a repository which he didn't watch, he will be watch that.
			if err = repo_model.WatchIfAuto(ctx, opts.PusherID, repo.ID, true); err != nil {
				log.Warn("Fail to perform auto watch on user %v for repo %v: %v", opts.PusherID, repo.ID, err)
			}
		} else {
			log.Trace("Non-tag and non-branch commits pushed.")
		}
	}

	if len(addTags)+len(delTags) > 0 {
		if err := PushUpdateAddDeleteTags(ctx, repo, gitRepo, addTags, delTags); err != nil {
			return fmt.Errorf("PushUpdateAddDeleteTags: %w", err)
		}
	}

	// Change repository last updated time.
	if err := repo_model.UpdateRepositoryUpdatedTime(ctx, repo.ID, time.Now()); err != nil {
		return fmt.Errorf("UpdateRepositoryUpdatedTime: %w", err)
	}

	return nil
}

func getCompareURL(repo *repo_model.Repository, gitRepo *git.Repository, objectFormat git.ObjectFormat, commits []*repo_module.PushCommit, opts *repo_module.PushUpdateOptions) string {
	oldCommitID := opts.OldCommitID
	if oldCommitID == objectFormat.EmptyObjectID().String() && len(commits) > 0 {
		oldCommit, err := gitRepo.GetCommit(commits[len(commits)-1].Sha1)
		if err != nil && !git.IsErrNotExist(err) {
			log.Error("unable to GetCommit %s from %-v: %v", oldCommitID, repo, err)
		}
		if oldCommit != nil {
			for i := 0; i < oldCommit.ParentCount(); i++ {
				commitID, _ := oldCommit.ParentID(i)
				if !commitID.IsZero() {
					oldCommitID = commitID.String()
					break
				}
			}
		}
	}

	if oldCommitID == objectFormat.EmptyObjectID().String() && repo.DefaultBranch != opts.RefFullName.BranchName() {
		oldCommitID = repo.DefaultBranch
	}

	if oldCommitID != objectFormat.EmptyObjectID().String() {
		return repo.ComposeCompareURL(oldCommitID, opts.NewCommitID)
	}
	return ""
}

func pushNewBranch(ctx context.Context, repo *repo_model.Repository, pusher *user_model.User, opts *repo_module.PushUpdateOptions, newCommit *git.Commit) ([]*git.Commit, error) {
	if repo.IsEmpty { // Change default branch and empty status only if pushed ref is non-empty branch.
		repo.DefaultBranch = opts.RefName()
		repo.IsEmpty = false
		if repo.DefaultBranch != setting.Repository.DefaultBranch {
			if err := gitrepo.SetDefaultBranch(ctx, repo, repo.DefaultBranch); err != nil {
				return nil, err
			}
		}
		// Update the is empty and default_branch columns
		if err := repo_model.UpdateRepositoryCols(ctx, repo, "default_branch", "is_empty"); err != nil {
			return nil, fmt.Errorf("UpdateRepositoryCols: %w", err)
		}
	}

	l, err := newCommit.CommitsBeforeLimit(10)
	if err != nil {
		return nil, fmt.Errorf("newCommit.CommitsBeforeLimit: %w", err)
	}
	notify_service.CreateRef(ctx, pusher, repo, opts.RefFullName, opts.NewCommitID)
	return l, nil
}

func pushUpdateBranch(_ context.Context, repo *repo_model.Repository, pusher *user_model.User, opts *repo_module.PushUpdateOptions, newCommit *git.Commit) ([]*git.Commit, error) {
	l, err := newCommit.CommitsBeforeUntil(opts.OldCommitID)
	if err != nil {
		return nil, fmt.Errorf("newCommit.CommitsBeforeUntil: %w", err)
	}

	branch := opts.RefFullName.BranchName()

	isForcePush, err := newCommit.IsForcePush(opts.OldCommitID)
	if err != nil {
		log.Error("IsForcePush %s:%s failed: %v", repo.FullName(), branch, err)
	}

	// only update branch can trigger pull request task because the pull request hasn't been created yet when creating a branch
	go pull_service.AddTestPullRequestTask(pull_service.TestPullRequestOptions{
		RepoID:      repo.ID,
		Doer:        pusher,
		Branch:      branch,
		IsSync:      true,
		IsForcePush: isForcePush,
		OldCommitID: opts.OldCommitID,
		NewCommitID: opts.NewCommitID,
	})

	if isForcePush {
		log.Trace("Push %s is a force push", opts.NewCommitID)

		cache.Remove(repo.GetCommitsCountCacheKey(opts.RefName(), true))
	} else {
		// TODO: increment update the commit count cache but not remove
		cache.Remove(repo.GetCommitsCountCacheKey(opts.RefName(), true))
	}

	return l, nil
}

func pushDeleteBranch(ctx context.Context, repo *repo_model.Repository, pusher *user_model.User, opts *repo_module.PushUpdateOptions) {
	notify_service.DeleteRef(ctx, pusher, repo, opts.RefFullName)

	if err := pull_service.AdjustPullsCausedByBranchDeleted(ctx, pusher, repo, opts.RefFullName.BranchName()); err != nil {
		// close all related pulls
		log.Error("close related pull request failed: %v", err)
	}
}

// PushUpdateAddDeleteTags updates a number of added and delete tags
func PushUpdateAddDeleteTags(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, addTags, delTags []string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := repo_model.PushUpdateDeleteTagsContext(ctx, repo, delTags); err != nil {
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

	releases, err := db.Find[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		RepoID:        repo.ID,
		TagNames:      tags,
		IncludeDrafts: true,
		IncludeTags:   true,
	})
	if err != nil {
		return fmt.Errorf("db.Find[repo_model.Release]: %w", err)
	}
	relMap := make(map[string]*repo_model.Release)
	for _, rel := range releases {
		relMap[rel.LowerTagName] = rel
	}

	lowerTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowerTags = append(lowerTags, strings.ToLower(tag))
	}

	newReleases := make([]*repo_model.Release, 0, len(lowerTags)-len(relMap))

	emailToUser := make(map[string]*user_model.User)

	for i, lowerTag := range lowerTags {
		tag, err := gitRepo.GetTag(tags[i])
		if err != nil {
			return fmt.Errorf("GetTag: %w", err)
		}
		commit, err := gitRepo.GetTagCommit(tag.Name)
		if err != nil {
			return fmt.Errorf("Commit: %w", err)
		}

		sig := tag.Tagger
		if sig == nil {
			sig = commit.Author
		}
		if sig == nil {
			sig = commit.Committer
		}
		var author *user_model.User
		createdAt := time.Unix(1, 0)

		if sig != nil {
			var ok bool
			author, ok = emailToUser[sig.Email]
			if !ok {
				author, err = user_model.GetUserByEmail(ctx, sig.Email)
				if err != nil && !user_model.IsErrUserNotExist(err) {
					return fmt.Errorf("GetUserByEmail: %w", err)
				}
				if author != nil {
					emailToUser[sig.Email] = author
				}
			}
			createdAt = sig.When
		}

		commitsCount, err := commit.CommitsCount()
		if err != nil {
			return fmt.Errorf("CommitsCount: %w", err)
		}

		rel, has := relMap[lowerTag]

		parts := strings.SplitN(tag.Message, "\n", 2)
		note := ""
		if len(parts) > 1 {
			note = parts[1]
		}
		if !has {
			rel = &repo_model.Release{
				RepoID:       repo.ID,
				Title:        parts[0],
				TagName:      tags[i],
				LowerTagName: lowerTag,
				Target:       "",
				Sha1:         commit.ID.String(),
				NumCommits:   commitsCount,
				Note:         note,
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
			if rel.IsTag {
				rel.Title = parts[0]
				rel.Note = note
				if author != nil {
					rel.PublisherID = author.ID
				}
			} else {
				rel.IsDraft = false
			}
			if err = repo_model.UpdateRelease(ctx, rel); err != nil {
				return fmt.Errorf("Update: %w", err)
			}
		}
	}

	if len(newReleases) > 0 {
		if err = db.Insert(ctx, newReleases); err != nil {
			return fmt.Errorf("Insert: %w", err)
		}
	}

	return nil
}
