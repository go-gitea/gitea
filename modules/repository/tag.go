// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

// SyncRepoTags synchronizes releases table with repository tags
func SyncRepoTags(ctx context.Context, repoID int64) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}

	gitRepo, err := OpenRepository(ctx, repo)
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	return SyncReleasesWithTags(ctx, repo, gitRepo)
}

// SyncReleasesWithTags synchronizes release table with repository tags
func SyncReleasesWithTags(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository) error {
	log.Debug("SyncReleasesWithTags: in Repo[%d:%s/%s]", repo.ID, repo.OwnerName, repo.Name)

	// optimized procedure for pull-mirrors which saves a lot of time (in
	// particular for repos with many tags).
	if repo.IsMirror {
		return pullMirrorReleaseSync(ctx, repo, gitRepo)
	}

	existingRelTags := make(container.Set[string])
	opts := repo_model.FindReleasesOptions{
		IncludeDrafts: true,
		IncludeTags:   true,
		ListOptions:   db.ListOptions{PageSize: 50},
		RepoID:        repo.ID,
	}
	for page := 1; ; page++ {
		opts.Page = page
		rels, err := db.Find[repo_model.Release](gitRepo.Ctx, opts)
		if err != nil {
			return fmt.Errorf("unable to GetReleasesByRepoID in Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
		}
		if len(rels) == 0 {
			break
		}
		for _, rel := range rels {
			if rel.IsDraft {
				continue
			}
			commitID, err := gitRepo.GetTagCommitID(rel.TagName)
			if err != nil && !git.IsErrNotExist(err) {
				return fmt.Errorf("unable to GetTagCommitID for %q in Repo[%d:%s/%s]: %w", rel.TagName, repo.ID, repo.OwnerName, repo.Name, err)
			}
			if git.IsErrNotExist(err) || commitID != rel.Sha1 {
				if err := repo_model.PushUpdateDeleteTag(ctx, repo, rel.TagName); err != nil {
					return fmt.Errorf("unable to PushUpdateDeleteTag: %q in Repo[%d:%s/%s]: %w", rel.TagName, repo.ID, repo.OwnerName, repo.Name, err)
				}
			} else {
				existingRelTags.Add(strings.ToLower(rel.TagName))
			}
		}
	}

	_, err := gitRepo.WalkReferences(git.ObjectTag, 0, 0, func(sha1, refname string) error {
		tagName := strings.TrimPrefix(refname, git.TagPrefix)
		if existingRelTags.Contains(strings.ToLower(tagName)) {
			return nil
		}

		if err := PushUpdateAddTag(ctx, repo, gitRepo, tagName, sha1, refname); err != nil {
			return fmt.Errorf("unable to PushUpdateAddTag: %q to Repo[%d:%s/%s]: %w", tagName, repo.ID, repo.OwnerName, repo.Name, err)
		}

		return nil
	})
	return err
}

// PushUpdateAddTag must be called for any push actions to add tag
func PushUpdateAddTag(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, tagName, sha1, refname string) error {
	tag, err := gitRepo.GetTagWithID(sha1, tagName)
	if err != nil {
		return fmt.Errorf("unable to GetTag: %w", err)
	}
	commit, err := tag.Commit(gitRepo)
	if err != nil {
		return fmt.Errorf("unable to get tag Commit: %w", err)
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
		author, err = user_model.GetUserByEmail(ctx, sig.Email)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			return fmt.Errorf("unable to GetUserByEmail for %q: %w", sig.Email, err)
		}
		createdAt = sig.When
	}

	commitsCount, err := commit.CommitsCount()
	if err != nil {
		return fmt.Errorf("unable to get CommitsCount: %w", err)
	}

	rel := repo_model.Release{
		RepoID:       repo.ID,
		TagName:      tagName,
		LowerTagName: strings.ToLower(tagName),
		Sha1:         commit.ID.String(),
		NumCommits:   commitsCount,
		CreatedUnix:  timeutil.TimeStamp(createdAt.Unix()),
		IsTag:        true,
	}
	if author != nil {
		rel.PublisherID = author.ID
	}

	return repo_model.SaveOrUpdateTag(ctx, repo, &rel)
}

// pullMirrorReleaseSync is a pull-mirror specific tag<->release table
// synchronization which overwrites all Releases from the repository tags. This
// can be relied on since a pull-mirror is always identical to its
// upstream. Hence, after each sync we want the pull-mirror release set to be
// identical to the upstream tag set. This is much more efficient for
// repositories like https://github.com/vim/vim (with over 13000 tags).
func pullMirrorReleaseSync(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository) error {
	log.Trace("pullMirrorReleaseSync: rebuilding releases for pull-mirror Repo[%d:%s/%s]", repo.ID, repo.OwnerName, repo.Name)
	tags, numTags, err := gitRepo.GetTagInfos(0, 0)
	if err != nil {
		return fmt.Errorf("unable to GetTagInfos in pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
	}
	err = db.WithTx(ctx, func(ctx context.Context) error {
		//
		// clear out existing releases
		//
		if _, err := db.DeleteByBean(ctx, &repo_model.Release{RepoID: repo.ID}); err != nil {
			return fmt.Errorf("unable to clear releases for pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
		}
		//
		// make release set identical to upstream tags
		//
		for _, tag := range tags {
			release := repo_model.Release{
				RepoID:       repo.ID,
				TagName:      tag.Name,
				LowerTagName: strings.ToLower(tag.Name),
				Sha1:         tag.Object.String(),
				// NOTE: ignored, since NumCommits are unused
				// for pull-mirrors (only relevant when
				// displaying releases, IsTag: false)
				NumCommits:  -1,
				CreatedUnix: timeutil.TimeStamp(tag.Tagger.When.Unix()),
				IsTag:       true,
			}
			if err := db.Insert(ctx, release); err != nil {
				return fmt.Errorf("unable insert tag %s for pull-mirror Repo[%d:%s/%s]: %w", tag.Name, repo.ID, repo.OwnerName, repo.Name, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to rebuild release table for pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
	}

	log.Trace("pullMirrorReleaseSync: done rebuilding %d releases", numTags)
	return nil
}
