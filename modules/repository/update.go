// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"
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

// PushUpdateAddDeleteTags updates a number of added and delete tags
func PushUpdateAddDeleteTags(repo *models.Repository, gitRepo *git.Repository, addTags, delTags []string) error {
	return models.WithTx(func(ctx models.DBContext) error {
		if err := models.PushUpdateDeleteTagsContext(ctx, repo, delTags); err != nil {
			return err
		}
		return pushUpdateAddTags(ctx, repo, gitRepo, addTags)
	})
}

// pushUpdateAddTags updates a number of add tags
func pushUpdateAddTags(ctx models.DBContext, repo *models.Repository, gitRepo *git.Repository, tags []string) error {
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

	emailToUser := make(map[string]*models.User)

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
		var author *models.User
		var createdAt = time.Unix(1, 0)

		if sig != nil {
			var ok bool
			author, ok = emailToUser[sig.Email]
			if !ok {
				author, err = models.GetUserByEmailContext(ctx, sig.Email)
				if err != nil && !models.IsErrUserNotExist(err) {
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
