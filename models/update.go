// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"container/list"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"
)

// env keys for git hooks need
const (
	EnvRepoName     = "GITEA_REPO_NAME"
	EnvRepoUsername = "GITEA_REPO_USER_NAME"
	EnvRepoIsWiki   = "GITEA_REPO_IS_WIKI"
	EnvPusherName   = "GITEA_PUSHER_NAME"
	EnvPusherEmail  = "GITEA_PUSHER_EMAIL"
	EnvPusherID     = "GITEA_PUSHER_ID"
	EnvKeyID        = "GITEA_KEY_ID"
	EnvIsDeployKey  = "GITEA_IS_DEPLOY_KEY"
	EnvIsInternal   = "GITEA_INTERNAL_PUSH"
)

// CommitToPushCommit transforms a git.Commit to PushCommit type.
func CommitToPushCommit(commit *git.Commit) *PushCommit {
	return &PushCommit{
		Sha1:           commit.ID.String(),
		Message:        commit.Message(),
		AuthorEmail:    commit.Author.Email,
		AuthorName:     commit.Author.Name,
		CommitterEmail: commit.Committer.Email,
		CommitterName:  commit.Committer.Name,
		Timestamp:      commit.Author.When,
	}
}

// ListToPushCommits transforms a list.List to PushCommits type.
func ListToPushCommits(l *list.List) *PushCommits {
	var commits []*PushCommit
	var actEmail string
	for e := l.Front(); e != nil; e = e.Next() {
		commit := e.Value.(*git.Commit)
		if actEmail == "" {
			actEmail = commit.Committer.Email
		}
		commits = append(commits, CommitToPushCommit(commit))
	}
	return &PushCommits{l.Len(), commits, "", make(map[string]string), make(map[string]*User)}
}

// PushUpdateAddDeleteTags updates a number of added and delete tags
func PushUpdateAddDeleteTags(repo *Repository, gitRepo *git.Repository, addTags, delTags []string) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return fmt.Errorf("Unable to begin sess in PushUpdateDeleteTags: %v", err)
	}
	if err := pushUpdateDeleteTags(sess, repo, delTags); err != nil {
		return err
	}
	if err := pushUpdateAddTags(sess, repo, gitRepo, addTags); err != nil {
		return err
	}

	return sess.Commit()
}

// PushUpdateDeleteTags updates a number of delete tags
func PushUpdateDeleteTags(repo *Repository, tags []string) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return fmt.Errorf("Unable to begin sess in PushUpdateDeleteTags: %v", err)
	}
	if err := pushUpdateDeleteTags(sess, repo, tags); err != nil {
		return err
	}

	return sess.Commit()
}

func pushUpdateDeleteTags(e Engine, repo *Repository, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	lowerTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowerTags = append(lowerTags, strings.ToLower(tag))
	}

	if _, err := e.
		Where("repo_id = ? AND is_tag = ?", repo.ID, true).
		In("lower_tag_name", lowerTags).
		Delete(new(Release)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	if _, err := e.
		Where("repo_id = ? AND is_tag = ?", repo.ID, false).
		In("lower_tag_name", lowerTags).
		Cols("is_draft", "num_commits", "sha1").
		Update(&Release{
			IsDraft: true,
		}); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	return nil
}

// PushUpdateDeleteTag must be called for any push actions to delete tag
func PushUpdateDeleteTag(repo *Repository, tagName string) error {
	rel, err := GetRelease(repo.ID, tagName)
	if err != nil {
		if IsErrReleaseNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetRelease: %v", err)
	}
	if rel.IsTag {
		if _, err = x.ID(rel.ID).Delete(new(Release)); err != nil {
			return fmt.Errorf("Delete: %v", err)
		}
	} else {
		rel.IsDraft = true
		rel.NumCommits = 0
		rel.Sha1 = ""
		if _, err = x.ID(rel.ID).AllCols().Update(rel); err != nil {
			return fmt.Errorf("Update: %v", err)
		}
	}

	return nil
}

// PushUpdateAddTags updates a number of add tags
func PushUpdateAddTags(repo *Repository, gitRepo *git.Repository, tags []string) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return fmt.Errorf("Unable to begin sess in PushUpdateAddTags: %v", err)
	}
	if err := pushUpdateAddTags(sess, repo, gitRepo, tags); err != nil {
		return err
	}

	return sess.Commit()
}
func pushUpdateAddTags(e Engine, repo *Repository, gitRepo *git.Repository, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	lowerTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowerTags = append(lowerTags, strings.ToLower(tag))
	}

	releases := make([]Release, 0, len(tags))
	if err := e.Where("repo_id = ?", repo.ID).
		In("lower_tag_name", lowerTags).Find(&releases); err != nil {
		return fmt.Errorf("GetRelease: %v", err)
	}
	relMap := make(map[string]*Release)
	for _, rel := range releases {
		relMap[rel.LowerTagName] = &rel
	}

	newReleases := make([]*Release, 0, len(lowerTags)-len(relMap))

	emailToUser := make(map[string]*User)

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
		var author *User
		var createdAt = time.Unix(1, 0)

		if sig != nil {
			var ok bool
			author, ok = emailToUser[sig.Email]
			if !ok {
				author, err = GetUserByEmail(sig.Email)
				if err != nil && !IsErrUserNotExist(err) {
					return fmt.Errorf("GetUserByEmail: %v", err)
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
			rel = &Release{
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
			if _, err = e.ID(rel.ID).AllCols().Update(rel); err != nil {
				return fmt.Errorf("Update: %v", err)
			}
		}
	}

	if len(newReleases) > 0 {
		if _, err := e.Insert(newReleases); err != nil {
			return fmt.Errorf("Insert: %v", err)
		}
	}

	return nil
}

// PushUpdateAddTag must be called for any push actions to add tag
func PushUpdateAddTag(repo *Repository, gitRepo *git.Repository, tagName string) error {
	rel, err := GetRelease(repo.ID, tagName)
	if err != nil && !IsErrReleaseNotExist(err) {
		return fmt.Errorf("GetRelease: %v", err)
	}

	tag, err := gitRepo.GetTag(tagName)
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

	var author *User
	var createdAt = time.Unix(1, 0)

	if sig != nil {
		author, err = GetUserByEmail(sig.Email)
		if err != nil && !IsErrUserNotExist(err) {
			return fmt.Errorf("GetUserByEmail: %v", err)
		}
		createdAt = sig.When
	}

	commitsCount, err := commit.CommitsCount()
	if err != nil {
		return fmt.Errorf("CommitsCount: %v", err)
	}

	if rel == nil {
		rel = &Release{
			RepoID:       repo.ID,
			Title:        "",
			TagName:      tagName,
			LowerTagName: strings.ToLower(tagName),
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

		if _, err = x.InsertOne(rel); err != nil {
			return fmt.Errorf("InsertOne: %v", err)
		}
	} else {
		rel.Sha1 = commit.ID.String()
		rel.CreatedUnix = timeutil.TimeStamp(createdAt.Unix())
		rel.NumCommits = commitsCount
		rel.IsDraft = false
		if rel.IsTag && author != nil {
			rel.PublisherID = author.ID
		}
		if _, err = x.ID(rel.ID).AllCols().Update(rel); err != nil {
			return fmt.Errorf("Update: %v", err)
		}
	}
	return nil
}
