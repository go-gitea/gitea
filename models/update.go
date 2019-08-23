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
