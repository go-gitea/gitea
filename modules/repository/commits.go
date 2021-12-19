// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"code.gitea.io/gitea/models/avatars"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
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
	Commits    []*PushCommit
	HeadCommit *PushCommit
	CompareURL string
	Len        int

	avatars    map[string]string
	emailUsers map[string]*user_model.User
}

// NewPushCommits creates a new PushCommits object.
func NewPushCommits() *PushCommits {
	return &PushCommits{
		avatars:    make(map[string]string),
		emailUsers: make(map[string]*user_model.User),
	}
}

// toAPIPayloadCommit converts a single PushCommit to an api.PayloadCommit object.
func (pc *PushCommits) toAPIPayloadCommit(ctx context.Context, repoPath, repoLink string, commit *PushCommit) (*api.PayloadCommit, error) {
	var err error
	authorUsername := ""
	author, ok := pc.emailUsers[commit.AuthorEmail]
	if !ok {
		author, err = user_model.GetUserByEmail(commit.AuthorEmail)
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
		committer, err = user_model.GetUserByEmail(commit.CommitterEmail)
		if err == nil {
			// TODO: check errors other than email not found.
			committerUsername = committer.Name
			pc.emailUsers[commit.CommitterEmail] = committer
		}
	} else {
		committerUsername = committer.Name
	}

	fileStatus, err := git.GetCommitFileStatus(ctx, repoPath, commit.Sha1)
	if err != nil {
		return nil, fmt.Errorf("FileStatus [commit_sha1: %s]: %v", commit.Sha1, err)
	}

	return &api.PayloadCommit{
		ID:      commit.Sha1,
		Message: commit.Message,
		URL:     fmt.Sprintf("%s/commit/%s", repoLink, url.PathEscape(commit.Sha1)),
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
	}, nil
}

// ToAPIPayloadCommits converts a PushCommits object to api.PayloadCommit format.
// It returns all converted commits and, if provided, the head commit or an error otherwise.
func (pc *PushCommits) ToAPIPayloadCommits(ctx context.Context, repoPath, repoLink string) ([]*api.PayloadCommit, *api.PayloadCommit, error) {
	commits := make([]*api.PayloadCommit, len(pc.Commits))
	var headCommit *api.PayloadCommit

	if pc.emailUsers == nil {
		pc.emailUsers = make(map[string]*user_model.User)
	}
	for i, commit := range pc.Commits {
		apiCommit, err := pc.toAPIPayloadCommit(ctx, repoPath, repoLink, commit)
		if err != nil {
			return nil, nil, err
		}

		commits[i] = apiCommit
		if pc.HeadCommit != nil && pc.HeadCommit.Sha1 == commits[i].ID {
			headCommit = apiCommit
		}
	}
	if pc.HeadCommit != nil && headCommit == nil {
		var err error
		headCommit, err = pc.toAPIPayloadCommit(ctx, repoPath, repoLink, pc.HeadCommit)
		if err != nil {
			return nil, nil, err
		}
	}
	return commits, headCommit, nil
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

	size := avatars.DefaultAvatarPixelSize * setting.Avatar.RenderedSizeFactor

	u, ok := pc.emailUsers[email]
	if !ok {
		var err error
		u, err = user_model.GetUserByEmail(email)
		if err != nil {
			pc.avatars[email] = avatars.GenerateEmailAvatarFastLink(email, size)
			if !user_model.IsErrUserNotExist(err) {
				log.Error("GetUserByEmail: %v", err)
				return ""
			}
		} else {
			pc.emailUsers[email] = u
		}
	}
	if u != nil {
		pc.avatars[email] = u.AvatarLinkWithSize(size)
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

// GitToPushCommits transforms a list of git.Commits to PushCommits type.
func GitToPushCommits(gitCommits []*git.Commit) *PushCommits {
	commits := make([]*PushCommit, 0, len(gitCommits))
	for _, commit := range gitCommits {
		commits = append(commits, CommitToPushCommit(commit))
	}
	return &PushCommits{
		Commits:    commits,
		HeadCommit: nil,
		CompareURL: "",
		Len:        len(commits),
		avatars:    make(map[string]string),
		emailUsers: make(map[string]*user_model.User),
	}
}
