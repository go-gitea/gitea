// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"container/list"
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// PushCommit represents a commit in a push operation.
type PushCommit struct {
	Sha1           string
	Message        string
	AuthorEmail    string
	AuthorName     string
	CommitterEmail string
	CommitterName  string
	Timestamp      time.Time
}

// PushCommits represents list of commits in a push operation.
type PushCommits struct {
	Len        int
	Commits    []*PushCommit
	CompareURL string

	avatars    map[string]string
	emailUsers map[string]*models.User
}

// NewPushCommits creates a new PushCommits object.
func NewPushCommits() *PushCommits {
	return &PushCommits{
		avatars:    make(map[string]string),
		emailUsers: make(map[string]*models.User),
	}
}

// ToAPIPayloadCommits converts a PushCommits object to
// api.PayloadCommit format.
func (pc *PushCommits) ToAPIPayloadCommits(repoPath, repoLink string) ([]*api.PayloadCommit, error) {
	commits := make([]*api.PayloadCommit, len(pc.Commits))

	if pc.emailUsers == nil {
		pc.emailUsers = make(map[string]*models.User)
	}
	var err error
	for i, commit := range pc.Commits {
		authorUsername := ""
		author, ok := pc.emailUsers[commit.AuthorEmail]
		if !ok {
			author, err = models.GetUserByEmail(commit.AuthorEmail)
			if err == nil {
				authorUsername = author.Name
				pc.emailUsers[commit.AuthorEmail] = author
			}
		} else {
			authorUsername = author.Name
		}

		committerUsername := ""
		committer, ok := pc.emailUsers[commit.CommitterEmail]
		if !ok {
			committer, err = models.GetUserByEmail(commit.CommitterEmail)
			if err == nil {
				// TODO: check errors other than email not found.
				committerUsername = committer.Name
				pc.emailUsers[commit.CommitterEmail] = committer
			}
		} else {
			committerUsername = committer.Name
		}

		fileStatus, err := git.GetCommitFileStatus(repoPath, commit.Sha1)
		if err != nil {
			return nil, fmt.Errorf("FileStatus [commit_sha1: %s]: %v", commit.Sha1, err)
		}

		commits[i] = &api.PayloadCommit{
			ID:      commit.Sha1,
			Message: commit.Message,
			URL:     fmt.Sprintf("%s/commit/%s", repoLink, commit.Sha1),
			Author: &api.PayloadUser{
				Name:     commit.AuthorName,
				Email:    commit.AuthorEmail,
				UserName: authorUsername,
			},
			Committer: &api.PayloadUser{
				Name:     commit.CommitterName,
				Email:    commit.CommitterEmail,
				UserName: committerUsername,
			},
			Added:     fileStatus.Added,
			Removed:   fileStatus.Removed,
			Modified:  fileStatus.Modified,
			Timestamp: commit.Timestamp,
		}
	}
	return commits, nil
}

// AvatarLink tries to match user in database with e-mail
// in order to show custom avatar, and falls back to general avatar link.
func (pc *PushCommits) AvatarLink(email string) string {
	if pc.avatars == nil {
		pc.avatars = make(map[string]string)
	}
	avatar, ok := pc.avatars[email]
	if ok {
		return avatar
	}

	size := models.DefaultAvatarPixelSize * models.AvatarRenderedSizeFactor

	u, ok := pc.emailUsers[email]
	if !ok {
		var err error
		u, err = models.GetUserByEmail(email)
		if err != nil {
			pc.avatars[email] = models.SizedAvatarLink(email, size)
			if !models.IsErrUserNotExist(err) {
				log.Error("GetUserByEmail: %v", err)
				return ""
			}
		} else {
			pc.emailUsers[email] = u
		}
	}
	if u != nil {
		pc.avatars[email] = u.RealSizedAvatarLink(size)
	}

	return pc.avatars[email]
}

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
	return &PushCommits{l.Len(), commits, "", make(map[string]string), make(map[string]*models.User)}
}
